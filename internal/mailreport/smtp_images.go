package mailreport

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"image/png"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/textproto"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

const (
	maxMessageBytes  = 4 << 20
	maxPNGBytes      = 1 << 20
	maxTotalPNGBytes = 2 << 20
	maxInlinePNGs    = 64
	maxPNGDimension  = 8192
	maxPNGPixelCount = 16 << 20
)

var (
	inlineDataReference = regexp.MustCompile(`(?i)(?:^|[^a-z0-9+.-])data:`)
	inlineCIDReference  = regexp.MustCompile(`(?i)(?:^|[^a-z0-9+.-])cid:`)
	inlineCSSImage      = regexp.MustCompile(`(?i)url\s*\(\s*['"]?\s*(?:data|cid):`)
)

type inlinePNG struct {
	cid  string
	data []byte
}

type messageBuffer struct {
	bytes.Buffer
}

func (b *messageBuffer) Write(p []byte) (int, error) {
	if len(p) > maxMessageBytes-b.Len() {
		return 0, fmt.Errorf("email exceeds encoded size limit")
	}
	return b.Buffer.Write(p)
}

func inlinePNGImages(source string) (string, []inlinePNG, error) {
	var result strings.Builder
	var images []inlinePNG
	seen := make(map[string]bool)
	totalBytes := 0
	inStyle := false
	tokens := html.NewTokenizer(strings.NewReader(source))
	for {
		kind := tokens.Next()
		raw := string(tokens.Raw())
		if kind == html.ErrorToken {
			if tokens.Err() != io.EOF {
				return "", nil, tokens.Err()
			}
			if len(raw) > 0 {
				return "", nil, fmt.Errorf("incomplete HTML markup")
			}
			break
		}
		if kind == html.TextToken && inStyle && inlineCSSImage.MatchString(raw) {
			return "", nil, fmt.Errorf("unsupported inline CSS image")
		}
		if kind == html.StartTagToken || kind == html.SelfClosingTagToken || kind == html.EndTagToken {
			token := tokens.Token()
			if token.Data == "svg" || strings.HasSuffix(token.Data, ":svg") {
				return "", nil, fmt.Errorf("SVG is not supported in email")
			}
			if token.Data == "style" {
				inStyle = kind == html.StartTagToken
			}
			changed := false
			srcCount := 0
			for i := range token.Attr {
				attr := &token.Attr[i]
				value := strings.TrimSpace(attr.Val)
				lower := strings.ToLower(value)
				if attr.Key == "style" && inlineCSSImage.MatchString(value) {
					return "", nil, fmt.Errorf("unsupported inline CSS image")
				}
				if (token.Data == "object" || token.Data == "embed") && attr.Key == "type" && lower == "image/svg+xml" {
					return "", nil, fmt.Errorf("SVG is not supported in email")
				}
				// Descriptive text (including alt/title) can mention data: or cid:.
				// Only attributes that supply a resource are image references.
				if attr.Key != "src" && attr.Key != "srcset" && attr.Key != "background" &&
					attr.Key != "poster" && !(token.Data == "object" && attr.Key == "data") {
					continue
				}
				if inlineCIDReference.MatchString(value) {
					return "", nil, fmt.Errorf("HTML contains an unattached CID reference")
				}
				if token.Data == "img" && attr.Key == "src" {
					srcCount++
					if srcCount > 1 {
						return "", nil, fmt.Errorf("image has duplicate sources")
					}
				}
				if token.Data == "img" && attr.Key == "src" && strings.HasPrefix(lower, "data:") {
					const prefix = "data:image/png;base64,"
					if !strings.HasPrefix(lower, prefix) {
						return "", nil, fmt.Errorf("only base64 PNG data images are supported")
					}
					payload := value[len(prefix):]
					if len(payload) > base64.StdEncoding.EncodedLen(maxPNGBytes) {
						return "", nil, fmt.Errorf("PNG exceeds size limit")
					}
					data, err := base64.StdEncoding.Strict().DecodeString(payload)
					if err != nil {
						return "", nil, fmt.Errorf("invalid PNG base64")
					}
					cid := fmt.Sprintf("chart-%x@mailreport", sha256.Sum256(data))
					if !seen[cid] {
						if len(images) >= maxInlinePNGs {
							return "", nil, fmt.Errorf("email exceeds inline image count limit")
						}
						if len(data) > maxTotalPNGBytes-totalBytes {
							return "", nil, fmt.Errorf("email exceeds cumulative PNG size limit")
						}
						if err := validatePNG(data); err != nil {
							return "", nil, err
						}
						images = append(images, inlinePNG{cid: cid, data: data})
						seen[cid] = true
						totalBytes += len(data)
					}
					attr.Val = "cid:" + cid
					changed = true
					continue
				}
				if inlineDataReference.MatchString(value) || strings.Contains(lower, "image/svg+xml") {
					return "", nil, fmt.Errorf("unsupported inline image reference")
				}
			}
			if changed {
				raw = token.String()
			}
		}
		result.WriteString(raw)
	}
	return result.String(), images, nil
}

func validatePNG(data []byte) error {
	if len(data) == 0 || len(data) > maxPNGBytes {
		return fmt.Errorf("PNG exceeds size limit or is empty")
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("invalid PNG image")
	}
	// Check dimensions before allocating pixels, including highly compressed images.
	if cfg.Width <= 0 || cfg.Height <= 0 ||
		cfg.Width > maxPNGDimension || cfg.Height > maxPNGDimension ||
		cfg.Width > maxPNGPixelCount/cfg.Height {
		return fmt.Errorf("PNG dimensions exceed limit")
	}
	reader := bytes.NewReader(data)
	if _, err := png.Decode(reader); err != nil {
		return fmt.Errorf("invalid PNG image")
	}
	if reader.Len() != 0 {
		return fmt.Errorf("PNG contains trailing data")
	}
	return nil
}

func writeTextPart(writer *multipart.Writer, kind, content string) error {
	headers := make(textproto.MIMEHeader)
	headers.Set("Content-Type", kind+"; charset=UTF-8")
	headers.Set("Content-Transfer-Encoding", "quoted-printable")
	part, err := writer.CreatePart(headers)
	if err != nil {
		return err
	}
	qp := quotedprintable.NewWriter(part)
	if _, err := io.WriteString(qp, content); err != nil {
		return err
	}
	return qp.Close()
}

func writePNGPart(writer *multipart.Writer, image inlinePNG) error {
	headers := make(textproto.MIMEHeader)
	headers.Set("Content-Type", "image/png")
	headers.Set("Content-Transfer-Encoding", "base64")
	headers.Set("Content-ID", "<"+image.cid+">")
	headers.Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": image.cid + ".png"}))
	part, err := writer.CreatePart(headers)
	if err != nil {
		return err
	}
	encoded := base64.StdEncoding.EncodeToString(image.data)
	for len(encoded) > 0 {
		n := min(len(encoded), 76)
		if _, err := io.WriteString(part, encoded[:n]+"\r\n"); err != nil {
			return err
		}
		encoded = encoded[n:]
	}
	return nil
}
