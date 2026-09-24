package mailreport

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"io"
	"math/rand"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/textproto"
	"regexp"
	"strings"
	"testing"
)

type mimeNode struct {
	kind     string
	params   map[string]string
	header   textproto.MIMEHeader
	body     []byte
	wireBody []byte
	children []*mimeNode
}

func parseMIMETree(t *testing.T, header textproto.MIMEHeader, reader io.Reader) *mimeNode {
	t.Helper()
	kind, params, err := mime.ParseMediaType(header.Get("Content-Type"))
	if err != nil {
		t.Fatal(err)
	}
	node := &mimeNode{kind: kind, params: params, header: header}
	if strings.HasPrefix(kind, "multipart/") {
		if params["boundary"] == "" {
			t.Fatal("missing multipart boundary")
		}
		parts := multipart.NewReader(reader, params["boundary"])
		for {
			part, err := parts.NextRawPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			node.children = append(node.children, parseMIMETree(t, part.Header, part))
		}
		return node
	}
	node.wireBody, err = io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	reader = bytes.NewReader(node.wireBody)
	switch header.Get("Content-Transfer-Encoding") {
	case "quoted-printable":
		reader = quotedprintable.NewReader(reader)
	case "base64":
		reader = base64.NewDecoder(base64.StdEncoding, reader)
	default:
		t.Fatalf("unsupported transfer encoding: %s", header.Get("Content-Transfer-Encoding"))
	}
	node.body, err = io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	return node
}

func captureSMTPMessage(t *testing.T, text, html string) []byte {
	t.Helper()
	received := make(chan []byte, 1)
	cfg, done := localSMTP(t, func(conn net.Conn) {
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
				received <- body
				reply("250 accepted")
			case line == "QUIT":
				reply("221 bye")
				return
			default:
				reply("500 unexpected")
			}
		}
	})
	if err := Send(context.Background(), cfg, []string{"reader@example.test"}, "中文图表早报", text, html); err != nil {
		t.Fatal(err)
	}
	<-done
	select {
	case body := <-received:
		return body
	default:
		t.Fatal("SMTP accepted no message")
		return nil
	}
}

func testPNG(t *testing.T, width, height int, shade uint8) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: shade, G: uint8(x), B: uint8(y), A: 255})
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		t.Fatal(err)
	}
	return encoded.Bytes()
}

func pngURI(data []byte) string {
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)
}

func TestSendPNGImagesAsRelatedMIME(t *testing.T) {
	first := testPNG(t, 17, 11, 80)
	second := testPNG(t, 13, 9, 160)
	html := `<html><body><h1>中文趋势</h1><img alt="项目 A &amp; B" width="17" src="` + pngURI(first) + `"><IMG SRC='` + pngURI(second) + `'/><img src=` + pngURI(first) + `></body></html>`
	raw := captureSMTPMessage(t, "纯文本回退：完成 12，新增 3。", html)
	message, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	subject, err := new(mime.WordDecoder).DecodeHeader(message.Header.Get("Subject"))
	if err != nil || subject != "中文图表早报" {
		t.Fatalf("subject=%q err=%v", subject, err)
	}
	tree := parseMIMETree(t, textproto.MIMEHeader(message.Header), message.Body)
	if tree.kind != "multipart/alternative" || len(tree.children) != 2 {
		t.Fatalf("expected plain + related alternative, got %s with %d children", tree.kind, len(tree.children))
	}
	plain, related := tree.children[0], tree.children[1]
	if plain.kind != "text/plain" || string(plain.body) != "纯文本回退：完成 12，新增 3。" {
		t.Fatalf("lost plain text fallback: %s %q", plain.kind, plain.body)
	}
	if related.kind != "multipart/related" || len(related.children) != 3 {
		t.Fatalf("expected HTML + 2 deduplicated images in related, got %s with %d children", related.kind, len(related.children))
	}
	if related.params["type"] != "text/html" || related.children[0].kind != "text/html" {
		t.Fatalf("HTML must be related root: %#v", related.params)
	}
	htmlBody := string(related.children[0].body)
	if strings.Contains(strings.ToLower(htmlBody), "<svg") || strings.Contains(strings.ToLower(htmlBody), "data:image") {
		t.Fatalf("unsupported image markup remains: %s", htmlBody)
	}
	if !strings.Contains(htmlBody, "中文趋势") || !strings.Contains(htmlBody, `width="17"`) || !strings.Contains(htmlBody, "项目 A &amp; B") {
		t.Fatalf("HTML content or image attributes lost: %s", htmlBody)
	}
	attachments := make(map[string][]byte)
	for _, part := range related.children[1:] {
		if part.kind != "image/png" || part.header.Get("Content-Transfer-Encoding") != "base64" {
			t.Fatalf("wrong image MIME: %v", part.header)
		}
		disposition, params, err := mime.ParseMediaType(part.header.Get("Content-Disposition"))
		if err != nil || disposition != "inline" || !strings.HasSuffix(params["filename"], ".png") {
			t.Fatalf("invalid inline disposition: %v (%v)", part.header, err)
		}
		id := part.header.Get("Content-ID")
		if len(id) < 3 || id[0] != '<' || id[len(id)-1] != '>' {
			t.Fatalf("invalid Content-ID: %q", id)
		}
		id = id[1 : len(id)-1]
		if _, duplicate := attachments[id]; duplicate {
			t.Fatalf("duplicate Content-ID: %q", id)
		}
		attachments[id] = part.body
		decoded, err := png.Decode(bytes.NewReader(part.body))
		if err != nil || decoded.Bounds().Empty() {
			t.Fatalf("unreadable PNG: %v", err)
		}
		lines := strings.Split(strings.TrimSpace(string(part.wireBody)), "\n")
		for i, line := range lines {
			length := len(strings.TrimSuffix(line, "\r"))
			if length > 76 || (i < len(lines)-1 && length != 76) {
				t.Fatalf("base64 line %d length=%d", i, length)
			}
		}
	}
	refs := regexp.MustCompile(`src="cid:([^"]+)"`).FindAllStringSubmatch(htmlBody, -1)
	if len(refs) != 3 || refs[0][1] != refs[2][1] || refs[0][1] == refs[1][1] {
		t.Fatalf("CID references not deduplicated: %v", refs)
	}
	for _, ref := range refs {
		if _, found := attachments[ref[1]]; !found {
			t.Fatalf("CID %q has no matching attachment", ref[1])
		}
	}
	if !bytes.Equal(attachments[refs[0][1]], first) || !bytes.Equal(attachments[refs[1][1]], second) {
		t.Fatal("attached PNG bytes differ from source")
	}
}

func TestEncodeRejectsUnsupportedOrInvalidInlineImages(t *testing.T) {
	valid := testPNG(t, 8, 6, 30)
	corrupt := append([]byte(nil), valid...)
	corrupt[len(corrupt)-1] ^= 1
	dimensions := func(width, height uint32) []byte {
		data := append([]byte(nil), valid...)
		binary.BigEndian.PutUint32(data[16:20], width)
		binary.BigEndian.PutUint32(data[20:24], height)
		binary.BigEndian.PutUint32(data[29:33], crc32.ChecksumIEEE(data[12:29]))
		return data
	}
	cases := []struct {
		name, html string
	}{
		{"svg markup", `<SVG viewBox="0 0 10 10"><path d="M0 0"/></SVG>`},
		{"namespaced svg", `<svg:svg xmlns:svg="http://www.w3.org/2000/svg"/>`},
		{"svg data", `<img src="data:image/svg+xml;base64,PHN2Zy8+">`},
		{"jpeg data", `<img src="data:image/jpeg;base64,aGVsbG8=">`},
		{"gif data", `<img src="data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///ywAAAAAAQABAAACAUwAOw==">`},
		{"other data", `<img src="data:text/plain;base64,aGVsbG8=">`},
		{"non base64 PNG", `<img src="data:image/png,%89PNG">`},
		{"invalid base64", `<img src="data:image/png;base64,###">`},
		{"empty PNG", `<img src="data:image/png;base64,">`},
		{"wrong PNG bytes", `<img src="` + pngURI([]byte("not a PNG")) + `">`},
		{"truncated PNG", `<img src="` + pngURI(valid[:len(valid)/2]) + `">`},
		{"corrupt PNG checksum", `<img src="` + pngURI(corrupt) + `">`},
		{"trailing PNG payload", `<img src="` + pngURI(append(append([]byte(nil), valid...), []byte("<svg/>")...)) + `">`},
		{"excessive width", `<img src="` + pngURI(dimensions(8193, 1)) + `">`},
		{"excessive height", `<img src="` + pngURI(dimensions(1, 8193)) + `">`},
		{"excessive pixels", `<img src="` + pngURI(dimensions(4097, 4096)) + `">`},
		{"zero dimension", `<img src="` + pngURI(dimensions(0, 1)) + `">`},
		{"srcset data", `<img srcset="` + pngURI(valid) + ` 1x">`},
		{"css data", `<div style="background:url(` + pngURI(valid) + `)">chart</div>`},
		{"style block data", `<style>.chart{background:url(` + pngURI(valid) + `)}</style>`},
		{"style block cid", `<style>.chart{background:url("cid:missing@example.test")}</style>`},
		{"entity encoded data", `<div style="background:url(d&#97;ta:image/png;base64,AA==)">chart</div>`},
		{"object data", `<object data="` + pngURI(valid) + `"></object>`},
		{"SVG object", `<object type="image/svg+xml" data="chart.svg"></object>`},
		{"dangling cid", `<img src="cid:missing@example.test">`},
		{"duplicate src", `<img src="` + pngURI(valid) + `" src="cid:missing@example.test">`},
		{"truncated image tag", `<img src="` + pngURI(valid) + `"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			message, err := encodeMessage("sender@example.test", "test", "fallback", tc.html)
			if err == nil || len(message) != 0 {
				t.Fatalf("invalid image accepted: err=%v, message bytes=%d", err, len(message))
			}
		})
	}
}

func uncompressedPNG(t *testing.T, width, height int, seed int64) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	if _, err := rand.New(rand.NewSource(seed)).Read(img.Pix); err != nil {
		t.Fatal(err)
	}
	var data bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.NoCompression}
	if err := encoder.Encode(&data, img); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func TestEncodeLimitsMessageAndInlineImageResources(t *testing.T) {
	t.Run("single PNG bytes", func(t *testing.T) {
		data := uncompressedPNG(t, 600, 500, 1)
		if len(data) <= 1<<20 {
			t.Fatal("fixture must exceed the 1 MiB single-image limit")
		}
		if _, err := encodeMessage("sender@example.test", "test", "", `<img src="`+pngURI(data)+`">`); err == nil {
			t.Fatal("oversized PNG accepted")
		}
	})
	t.Run("cumulative PNG bytes", func(t *testing.T) {
		var html strings.Builder
		total := 0
		for i := int64(0); i < 4; i++ {
			data := uncompressedPNG(t, 400, 350, i)
			total += len(data)
			if len(data) > 1<<20 {
				t.Fatal("each image must fit the single-image budget")
			}
			fmt.Fprintf(&html, `<img src="%s">`, pngURI(data))
		}
		if total <= 2<<20 || html.Len() >= 4<<20 {
			t.Fatal("fixture must exceed cumulative image budget but fit input budget")
		}
		if _, err := encodeMessage("sender@example.test", "test", "", html.String()); err == nil {
			t.Fatal("cumulative image limit bypassed")
		}
	})
	t.Run("unique image count", func(t *testing.T) {
		var html strings.Builder
		for i := 0; i < 65; i++ {
			fmt.Fprintf(&html, `<img src="%s">`, pngURI(testPNG(t, 1, 1, uint8(i))))
		}
		if _, err := encodeMessage("sender@example.test", "test", "", html.String()); err == nil {
			t.Fatal("65 unique images accepted")
		}
	})
	t.Run("input bytes", func(t *testing.T) {
		if _, err := encodeMessage("sender@example.test", "test", strings.Repeat("a", 4<<20), "extra"); err == nil {
			t.Fatal("input byte limit bypassed")
		}
	})
	t.Run("encoded MIME bytes", func(t *testing.T) {
		// UTF-8 is below the input budget; quoted-printable exceeds the wire budget.
		if _, err := encodeMessage("sender@example.test", "test", strings.Repeat("中", 500000), "<p>fallback</p>"); err == nil {
			t.Fatal("quoted-printable expansion exceeded MIME budget")
		}
	})
}

func TestSendRejectsInvalidPNGBeforeSMTPConnection(t *testing.T) {
	connected := make(chan struct{}, 1)
	cfg, _ := localSMTP(t, func(conn net.Conn) { connected <- struct{}{} })
	err := Send(context.Background(), cfg, []string{"reader@example.test"}, "test", "fallback", `<img src="data:image/png;base64,aGVsbG8=">`)
	if err == nil || err.Error() != "cannot encode email" {
		t.Fatalf("image validation must fail before SMTP: %v", err)
	}
	select {
	case <-connected:
		t.Fatal("invalid image opened SMTP connection")
	default:
	}
}

func TestSendInlineImageCountAndDeduplicationBoundaries(t *testing.T) {
	cases := []struct {
		name        string
		references  int
		unique      bool
		attachments int
	}{
		{"64 unique images", 64, true, 64},
		{"65 references to one image", 65, false, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var html strings.Builder
			for i := 0; i < tc.references; i++ {
				shade := uint8(80)
				if tc.unique {
					shade = uint8(i)
				}
				fmt.Fprintf(&html, `<img src="%s">`, pngURI(testPNG(t, 2, 2, shade)))
			}
			raw := captureSMTPMessage(t, "图片数量边界", html.String())
			message, err := mail.ReadMessage(bytes.NewReader(raw))
			if err != nil {
				t.Fatal(err)
			}
			tree := parseMIMETree(t, textproto.MIMEHeader(message.Header), message.Body)
			if len(tree.children) != 2 {
				t.Fatalf("expected 2 alternatives, got %d", len(tree.children))
			}
			related := tree.children[1]
			if related.kind != "multipart/related" || len(related.children) != tc.attachments+1 {
				t.Fatalf("wrong image count: %s with %d children", related.kind, len(related.children))
			}
			ids := make(map[string]bool)
			for _, part := range related.children[1:] {
				if part.kind != "image/png" {
					t.Fatalf("unexpected attachment: %s", part.kind)
				}
				if _, err := png.Decode(bytes.NewReader(part.body)); err != nil {
					t.Fatal(err)
				}
				ids[strings.Trim(part.header.Get("Content-ID"), "<>")] = true
			}
			refs := regexp.MustCompile(`src="cid:([^"]+)"`).FindAllStringSubmatch(string(related.children[0].body), -1)
			if len(refs) != tc.references {
				t.Fatalf("lost image references: %d", len(refs))
			}
			for _, ref := range refs {
				if !ids[ref[1]] {
					t.Fatalf("missing attachment for CID: %s", ref[1])
				}
			}
		})
	}
}

func TestSendPNGWithEncodedAttributeAndCaseInsensitiveMediaType(t *testing.T) {
	uri := strings.Replace(pngURI(testPNG(t, 2, 2, 20)), "data:image/png;base64,", "DaTa:IMAGE/PNG;BASE64,", 1)
	uri = strings.Replace(uri, "DaTa:", "D&#97;Ta&#58;", 1)
	raw := captureSMTPMessage(t, "文本", `<IMG SRC=" `+uri+` " ALT="a &lt; b; cid:example image/svg+xml">`)
	message, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	tree := parseMIMETree(t, textproto.MIMEHeader(message.Header), message.Body)
	related := tree.children[1]
	if related.kind != "multipart/related" || len(related.children) != 2 {
		t.Fatalf("expected one related PNG: %s", related.kind)
	}
	if _, err := png.Decode(bytes.NewReader(related.children[1].body)); err != nil {
		t.Fatal(err)
	}
	html := string(related.children[0].body)
	cid := strings.Trim(related.children[1].header.Get("Content-ID"), "<>")
	if !strings.Contains(html, `src="cid:`+cid+`"`) || !strings.Contains(html, `alt="a &lt; b; cid:example image/svg+xml"`) {
		t.Fatalf("lost image or alt text: %s", html)
	}
}

func TestSendOrdinaryHTMLKeepsAlternativeCompatibility(t *testing.T) {
	html := `<style>.legend::before{content:"cid:example image/svg+xml"}</style><p title="metadata: ready">中文说明 &amp; summaries</p><p title="cid:example image/svg+xml">引用 cid:example 和类型 image/svg+xml；data:image/png 是说明文字。</p>`
	raw := captureSMTPMessage(t, "中文纯文本", html)
	message, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	tree := parseMIMETree(t, textproto.MIMEHeader(message.Header), message.Body)
	if tree.kind != "multipart/alternative" || len(tree.children) != 2 {
		t.Fatalf("expected two alternatives: %s (%d)", tree.kind, len(tree.children))
	}
	for i, want := range []struct{ kind, text string }{{"text/plain", "中文纯文本"}, {"text/html", html}} {
		part := tree.children[i]
		if part.kind != want.kind || string(part.body) != want.text || len(part.children) != 0 {
			t.Fatalf("changed ordinary %s: %s %q", want.kind, part.kind, part.body)
		}
	}
}
