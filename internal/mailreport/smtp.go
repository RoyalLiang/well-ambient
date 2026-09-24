package mailreport

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"mime"
	"mime/multipart"
	"net"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"time"
	"well-ambient/internal/config"
)

func Send(ctx context.Context, cfg config.SMTPConfig, recipients []string, subject, text, html string) error {
	return sendWithRoots(ctx, cfg, recipients, subject, text, html, nil)
}

// Private certificate-pool seam allows local TLS protocol tests; production uses
// system trust and never exposes a certificate-verification bypass.
func sendWithRoots(ctx context.Context, cfg config.SMTPConfig, recipients []string, subject, text, html string, roots *x509.CertPool) error {
	if !cfg.Enabled {
		return fmt.Errorf("SMTP is disabled")
	}
	cfg = cfg.Normalized()
	if err := config.ValidateSMTP(cfg); err != nil {
		return err
	}
	if len(recipients) == 0 || len(recipients) > 100 {
		return fmt.Errorf("SMTP requires 1 to 100 recipients")
	}
	if strings.ContainsAny(subject, "\r\n\x00") || len(subject) > 1024 {
		return fmt.Errorf("invalid email subject")
	}
	if len(text) > maxMessageBytes-len(html) {
		return fmt.Errorf("email exceeds size limit")
	}
	seen := make(map[string]bool)
	addresses := make([]string, 0, len(recipients))
	for _, raw := range recipients {
		address, err := config.Mailbox(raw)
		if err != nil {
			return err
		}
		if !seen[address] {
			addresses = append(addresses, address)
			seen[address] = true
		}
	}
	message, err := encodeMessage(cfg.From, subject, text, html, addresses...)
	if err != nil {
		return fmt.Errorf("cannot encode email")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)))
	if err != nil {
		return sendError(ctx, "connection")
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	rawConn := conn
	stop := context.AfterFunc(ctx, func() { _ = rawConn.Close() })
	defer stop()
	tlsConfig := &tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12, RootCAs: roots}
	if cfg.TLSMode == "tls" {
		secured := tls.Client(conn, tlsConfig)
		if err := secured.HandshakeContext(ctx); err != nil {
			return sendError(ctx, "TLS handshake")
		}
		conn = secured
	}
	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		return sendError(ctx, "greeting")
	}
	defer client.Close()
	if cfg.TLSMode == "starttls" {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return fmt.Errorf("SMTP server does not support required STARTTLS")
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			return sendError(ctx, "STARTTLS")
		}
	}
	if cfg.Username != "" {
		if err := client.Auth(smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)); err != nil {
			return sendError(ctx, "authentication")
		}
	}
	if err := client.Mail(cfg.From); err != nil {
		return sendError(ctx, "sender")
	}
	for _, recipient := range addresses {
		if err := client.Rcpt(recipient); err != nil {
			return sendError(ctx, "recipient")
		}
	}
	writer, err := client.Data()
	if err != nil {
		return sendError(ctx, "DATA")
	}
	if _, err = writer.Write(message); err != nil {
		_ = writer.Close()
		return sendError(ctx, "body")
	}
	if err = writer.Close(); err != nil {
		return sendError(ctx, "acceptance (delivery outcome may be unknown)")
	}
	// DATA success is the acceptance boundary; QUIT failure must not invite retries.
	_ = client.Quit()
	return nil
}
func sendError(ctx context.Context, stage string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("SMTP %s: %w", stage, err)
	}
	// Replies can echo credentials or message content.
	return fmt.Errorf("SMTP %s failed", stage)
}
func encodeMessage(from, subject, text, html string, recipients ...string) ([]byte, error) {
	if len(text) > maxMessageBytes-len(html) {
		return nil, fmt.Errorf("email exceeds size limit")
	}
	html, images, err := inlinePNGImages(html)
	if err != nil {
		return nil, err
	}
	var body messageBuffer
	multipartWriter := multipart.NewWriter(&body)
	if err := writeTextPart(multipartWriter, "text/plain", text); err != nil {
		return nil, err
	}
	if len(images) == 0 {
		if err := writeTextPart(multipartWriter, "text/html", html); err != nil {
			return nil, err
		}
	} else {
		related := multipart.NewWriter(nil)
		headers := make(textproto.MIMEHeader)
		headers.Set("Content-Type", mime.FormatMediaType("multipart/related", map[string]string{
			"boundary": related.Boundary(),
			"type":     "text/html",
		}))
		writer, err := multipartWriter.CreatePart(headers)
		if err != nil {
			return nil, err
		}
		boundary := related.Boundary()
		related = multipart.NewWriter(writer)
		if err := related.SetBoundary(boundary); err != nil {
			return nil, err
		}
		if err := writeTextPart(related, "text/html", html); err != nil {
			return nil, err
		}
		for _, image := range images {
			if err := writePNGPart(related, image); err != nil {
				return nil, err
			}
		}
		if err := related.Close(); err != nil {
			return nil, err
		}
	}
	if err := multipartWriter.Close(); err != nil {
		return nil, err
	}
	var result bytes.Buffer
	to := "undisclosed-recipients:;"
	if len(recipients) > 0 {
		for _, recipient := range recipients {
			if _, err := config.Mailbox(recipient); err != nil {
				return nil, err
			}
		}
		to = strings.Join(recipients, ", ")
	}
	fmt.Fprintf(&result, "From: %s\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\nMIME-Version: 1.0\r\nContent-Type: multipart/alternative; boundary=%q\r\n\r\n", from, to, mime.QEncoding.Encode("utf-8", subject), time.Now().UTC().Format(time.RFC1123Z), multipartWriter.Boundary())
	if body.Len() > maxMessageBytes-result.Len() {
		return nil, fmt.Errorf("email exceeds encoded size limit")
	}
	result.Write(body.Bytes())
	return result.Bytes(), nil
}
