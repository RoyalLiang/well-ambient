package mailreport

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/textproto"
	"strconv"
	"strings"
	"testing"
	"time"
	"well-ambient/internal/config"
)

func localSMTP(t *testing.T, serve func(net.Conn)) (config.SMTPConfig, <-chan struct{}) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		serve(conn)
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		select {
		case <-done:
		case <-time.After(6 * time.Second):
			t.Error("fake SMTP did not stop")
		}
	})
	_, portText, _ := net.SplitHostPort(listener.Addr().String())
	port, _ := strconv.Atoi(portText)
	return config.SMTPConfig{Enabled: true, Host: "127.0.0.1", Port: port, TLSMode: "none", From: "sender@example.test"}, done
}
func TestSendSubmitsMultipartMessageToLocalSMTP(t *testing.T) {
	received := make(chan []byte, 1)
	cfg, done := localSMTP(t, func(conn net.Conn) {
		reader := textproto.NewReader(bufio.NewReader(conn))
		writer := bufio.NewWriter(conn)
		reply := func(s string) { fmt.Fprint(writer, s+"\r\n"); _ = writer.Flush() }
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
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := Send(ctx, cfg, []string{"recipient@example.test"}, "中文早报", "纯文本内容", "<p>HTML内容</p>"); err != nil {
		t.Fatal(err)
	}
	<-done
	body := <-received
	message, err := mail.ReadMessage(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	subject, err := new(mime.WordDecoder).DecodeHeader(message.Header.Get("Subject"))
	if err != nil || subject != "中文早报" {
		t.Fatalf("subject=%q err=%v", subject, err)
	}
	kind, params, err := mime.ParseMediaType(message.Header.Get("Content-Type"))
	if err != nil || kind != "multipart/alternative" {
		t.Fatalf("MIME kind=%q err=%v", kind, err)
	}
	parts := multipart.NewReader(message.Body, params["boundary"])
	for _, expected := range []string{"纯文本内容", "<p>HTML内容</p>"} {
		part, err := parts.NextRawPart()
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := io.ReadAll(quotedprintable.NewReader(part))
		if err != nil || string(decoded) != expected {
			t.Fatalf("body=%q err=%v", decoded, err)
		}
	}
	if !strings.Contains(message.Header.Get("To"), "recipient@example.test") {
		t.Fatal("actual recipient missing from To header")
	}
}
func TestSendCancellationInterruptsGreeting(t *testing.T) {
	accepted := make(chan struct{})
	cfg, _ := localSMTP(t, func(conn net.Conn) { close(accepted); buffer := make([]byte, 1); _, _ = conn.Read(buffer) })
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- Send(ctx, cfg, []string{"recipient@example.test"}, "test", "text", "<p>text</p>") }()
	<-accepted
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected canceled: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation failed")
	}
}
func TestSendRejectsUnsafeConfigurationAndHeaders(t *testing.T) {
	base := config.SMTPConfig{Enabled: true, Host: "127.0.0.1", Port: 1, TLSMode: "none", From: "sender@example.test"}
	cases := []struct {
		name               string
		mutate             func(*config.SMTPConfig)
		recipient, subject string
	}{
		{"disabled", func(c *config.SMTPConfig) { c.Enabled = false }, "to@example.test", "test"},
		{"plaintext remote", func(c *config.SMTPConfig) { c.Host = "example.test" }, "to@example.test", "test"},
		{"plaintext credentials", func(c *config.SMTPConfig) { c.Username = "user"; c.Password = "secret" }, "to@example.test", "test"},
		{"header injection", func(c *config.SMTPConfig) {}, "to@example.test", "test\r\nBcc: attacker@example.test"},
		{"envelope injection", func(c *config.SMTPConfig) {}, "to@example.test\r\nRCPT TO:<attacker@example.test>", "test"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base
			tc.mutate(&cfg)
			if err := Send(context.Background(), cfg, []string{tc.recipient}, tc.subject, "", " "); err == nil {
				t.Fatal("unsafe request accepted")
			}
		})
	}
}
func TestSendRequiresSTARTTLS(t *testing.T) {
	cfg, _ := localSMTP(t, func(conn net.Conn) {
		reader := bufio.NewReader(conn)
		fmt.Fprint(conn, "220 localhost SMTP\r\n")
		_, _ = reader.ReadString('\n')
		fmt.Fprint(conn, "250 localhost\r\n")
		_, _ = reader.ReadString('\n')
	})
	cfg.TLSMode = "starttls"
	if err := Send(context.Background(), cfg, []string{"to@example.test"}, "test", "text", "html"); err == nil || !strings.Contains(err.Error(), "STARTTLS") {
		t.Fatalf("required STARTTLS bypassed: %v", err)
	}
}
func TestSendDoesNotExposeSMTPReplyContent(t *testing.T) {
	cfg, _ := localSMTP(t, func(conn net.Conn) {
		reader := textproto.NewReader(bufio.NewReader(conn))
		writer := bufio.NewWriter(conn)
		reply := func(s string) { fmt.Fprint(writer, s+"\r\n"); _ = writer.Flush() }
		reply("220 localhost SMTP")
		for {
			line, err := reader.ReadLine()
			if err != nil {
				return
			}
			if strings.HasPrefix(line, "EHLO") {
				reply("250 localhost")
			} else {
				reply("550 secret-password-echo")
				return
			}
		}
	})
	err := Send(context.Background(), cfg, []string{"to@example.test"}, "test", "text", "html")
	if err == nil || strings.Contains(err.Error(), "secret-password-echo") {
		t.Fatalf("unsafe error: %v", err)
	}
}
