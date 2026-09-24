package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image/png"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/textproto"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
	"well-ambient/internal/config"
	"well-ambient/internal/mailreport"

	nethtml "golang.org/x/net/html"
)

type emailDeliveryMIME struct {
	kind     string
	header   textproto.MIMEHeader
	body     []byte
	children []*emailDeliveryMIME
}

func readEmailDeliveryMIME(t *testing.T, header textproto.MIMEHeader, body io.Reader) *emailDeliveryMIME {
	t.Helper()
	kind, params, err := mime.ParseMediaType(header.Get("Content-Type"))
	if err != nil {
		t.Fatal(err)
	}
	part := &emailDeliveryMIME{kind: kind, header: header}
	if strings.HasPrefix(kind, "multipart/") {
		if params["boundary"] == "" {
			t.Fatal("multipart boundary missing")
		}
		reader := multipart.NewReader(body, params["boundary"])
		for {
			child, err := reader.NextRawPart()
			if err == io.EOF {
				return part
			}
			if err != nil {
				t.Fatal(err)
			}
			part.children = append(part.children, readEmailDeliveryMIME(t, child.Header, child))
		}
	}
	switch header.Get("Content-Transfer-Encoding") {
	case "quoted-printable":
		body = quotedprintable.NewReader(body)
	case "base64":
		body = base64.NewDecoder(base64.StdEncoding, body)
	default:
		t.Fatalf("unexpected transfer encoding: %v", header)
	}
	part.body, err = io.ReadAll(body)
	if err != nil {
		t.Fatal(err)
	}
	return part
}

func submitEmailDemoToLocalSMTP(t *testing.T, report emailReport) []byte {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	received := make(chan []byte, 1)
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		reader := textproto.NewReader(bufio.NewReader(conn))
		reply := func(line string) { _, _ = fmt.Fprint(conn, line+"\r\n") }
		reply("220 localhost SMTP")
		for {
			line, err := reader.ReadLine()
			if err != nil {
				return
			}
			switch {
			case strings.HasPrefix(line, "EHLO"), strings.HasPrefix(line, "HELO"):
				reply("250 localhost")
			case strings.HasPrefix(line, "MAIL FROM:"), strings.HasPrefix(line, "RCPT TO:"):
				reply("250 ok")
			case line == "DATA":
				reply("354 send data")
				body, err := reader.ReadDotBytes()
				if err != nil {
					return
				}
				// DotReader normalizes line endings; restore canonical EML CRLF.
				received <- bytes.ReplaceAll(body, []byte("\n"), []byte("\r\n"))
				reply("250 accepted")
			case line == "QUIT":
				reply("221 bye")
				return
			default:
				reply("500 unexpected")
			}
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		select {
		case <-done:
		case <-time.After(6 * time.Second):
			t.Error("local SMTP did not stop")
		}
	})
	_, portString, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.SMTPConfig{Enabled: true, Host: "127.0.0.1", Port: port, TLSMode: "none", From: "demo@example.test"}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := mailreport.Send(ctx, cfg, []string{"reader@example.test"}, report.Subject, report.Text, report.HTML); err != nil {
		t.Fatal(err)
	}
	<-done
	select {
	case body := <-received:
		return body
	default:
		t.Fatal("SMTP received no message")
		return nil
	}
}

func decodedEmailDeliveryHTML(t *testing.T, html string, images map[string][]byte) (string, int) {
	t.Helper()
	var result strings.Builder
	references := 0
	classes := make(map[string]int)
	referenced := make(map[string]bool)
	tokens := nethtml.NewTokenizer(strings.NewReader(html))
	for {
		kind := tokens.Next()
		if kind == nethtml.ErrorToken {
			if tokens.Err() != io.EOF {
				t.Fatal(tokens.Err())
			}
			break
		}
		raw := string(tokens.Raw())
		if kind == nethtml.StartTagToken || kind == nethtml.SelfClosingTagToken {
			token := tokens.Token()
			imageSource := false
			for i := range token.Attr {
				attr := &token.Attr[i]
				if strings.Contains(attr.Val, "cid:") && !(token.Data == "img" && attr.Key == "src") {
					t.Fatalf("CID reference outside an image source: %v", attr)
				}
				if token.Data != "img" {
					continue
				}
				if attr.Key == "class" {
					for _, class := range strings.Fields(attr.Val) {
						classes[class]++
					}
				}
				if attr.Key != "src" {
					continue
				}
				if !strings.HasPrefix(attr.Val, "cid:") {
					t.Fatalf("image not backed by a MIME CID: %s", attr.Val)
				}
				cid := strings.TrimPrefix(attr.Val, "cid:")
				data, ok := images[cid]
				if !ok {
					t.Fatalf("missing PNG attachment for CID %q", cid)
				}
				attr.Val = "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)
				imageSource = true
				referenced[cid] = true
				references++
			}
			if token.Data == "img" {
				if !imageSource {
					t.Fatal("image has no CID source")
				}
				raw = token.String()
			}
		}
		result.WriteString(raw)
	}
	for _, class := range []string{"email-resolution-image", "email-status-image", "email-trend-image", "email-distribution-bar", "email-commit-bar"} {
		if classes[class] == 0 {
			t.Errorf("delivered report is missing top/bottom chart class %q", class)
		}
	}
	if references < 5 || len(referenced) != len(images) {
		t.Fatalf("chart coverage: %d references, %d referenced attachments, %d attachments", references, len(referenced), len(images))
	}
	return result.String(), references
}

func TestEmailPNGDeliveryThroughLocalSMTPAllStyles(t *testing.T) {
	output := os.Getenv("WELL_EMAIL_COMPAT_OUTPUT")
	if output != "" && !filepath.IsAbs(output) {
		t.Fatal("WELL_EMAIL_COMPAT_OUTPUT must be an absolute directory")
	}
	for _, style := range []string{"brief", "focus", "ledger", "hyperframe"} {
		t.Run(style, func(t *testing.T) {
			template := config.DefaultEmailTemplate()
			template.Style = style
			report, err := emailTemplateDemo(template)
			if err != nil {
				t.Fatal(err)
			}
			raw := submitEmailDemoToLocalSMTP(t, report)
			message, err := mail.ReadMessage(bytes.NewReader(raw))
			if err != nil {
				t.Fatal(err)
			}
			subject, err := new(mime.WordDecoder).DecodeHeader(message.Header.Get("Subject"))
			if err != nil || subject != report.Subject {
				t.Fatalf("subject=%q, err=%v", subject, err)
			}
			tree := readEmailDeliveryMIME(t, textproto.MIMEHeader(message.Header), message.Body)
			if tree.kind != "multipart/alternative" || len(tree.children) != 2 {
				t.Fatalf("invalid outer alternatives: %s (%d)", tree.kind, len(tree.children))
			}
			plain, related := tree.children[0], tree.children[1]
			if plain.kind != "text/plain" ||
				strings.ReplaceAll(string(plain.body), "\r\n", "\n") != strings.ReplaceAll(report.Text, "\r\n", "\n") ||
				!strings.Contains(string(plain.body), "示例负责人甲") {
				t.Fatal("Chinese plain-text fallback changed or missing")
			}
			if related.kind != "multipart/related" || len(related.children) < 2 || related.children[0].kind != "text/html" {
				t.Fatalf("missing related HTML + PNG images: %s", related.kind)
			}
			html := string(related.children[0].body)
			if lower := strings.ToLower(html); strings.Contains(lower, "<svg") || strings.Contains(lower, "data:") {
				t.Fatal("delivered HTML contains SVG or a data URI")
			}
			if !strings.Contains(html, "示例负责人甲") || !strings.Contains(html, `data-email-style="`+style+`"`) {
				t.Fatal("delivered HTML lost the selected style or Chinese facts")
			}
			images := make(map[string][]byte)
			for _, part := range related.children[1:] {
				disposition, _, err := mime.ParseMediaType(part.header.Get("Content-Disposition"))
				if err != nil || disposition != "inline" || part.kind != "image/png" {
					t.Fatalf("unexpected attachment: %v (%v)", part.header, err)
				}
				id := part.header.Get("Content-ID")
				if !strings.HasPrefix(id, "<") || !strings.HasSuffix(id, ">") {
					t.Fatalf("invalid Content-ID %q", id)
				}
				cid := id[1 : len(id)-1]
				if _, exists := images[cid]; exists {
					t.Fatalf("duplicate attachment CID: %s", cid)
				}
				decoded, err := png.Decode(bytes.NewReader(part.body))
				if err != nil || decoded.Bounds().Empty() {
					t.Fatalf("attachment %s is not a readable PNG: %v", cid, err)
				}
				images[cid] = part.body
			}
			preview, references := decodedEmailDeliveryHTML(t, html, images)
			t.Logf("%s: %d image references, %d PNG attachments, %d MIME bytes", style, references, len(images), len(raw))
			if output != "" {
				if err := os.MkdirAll(output, 0755); err != nil {
					t.Fatal(err)
				}
				base := filepath.Join(output, "email-feishu-"+style)
				if err := os.WriteFile(base+".eml", raw, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(base+".html", []byte(preview), 0600); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
