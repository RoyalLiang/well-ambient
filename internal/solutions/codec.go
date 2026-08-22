package solutions

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

const (
	defaultCompressionThreshold = 4 * 1024
	defaultMaxMarkdownBytes     = 4 * 1024 * 1024
)

type encodedMarkdown struct {
	normalized string
	stored     []byte
	encoding   string
	hash       string
	rawBytes   int
}

func encodeMarkdown(markdown string, threshold, maxBytes int) (encodedMarkdown, error) {
	normalized := normalizeMarkdown(markdown)
	if normalized == "" {
		return encodedMarkdown{}, fmt.Errorf("%w: markdown is required", ErrInvalid)
	}
	raw := []byte(normalized)
	if len(raw) > maxBytes {
		return encodedMarkdown{}, fmt.Errorf("%w: markdown is %d bytes, limit is %d", ErrTooLarge, len(raw), maxBytes)
	}
	result := encodedMarkdown{
		normalized: normalized,
		stored:     raw,
		encoding:   "identity",
		hash:       hashBytes(raw),
		rawBytes:   len(raw),
	}
	if threshold <= 0 || len(raw) < threshold {
		return result, nil
	}
	var compressed bytes.Buffer
	writer, err := gzip.NewWriterLevel(&compressed, gzip.BestSpeed)
	if err != nil {
		return encodedMarkdown{}, err
	}
	if _, err := writer.Write(raw); err != nil {
		return encodedMarkdown{}, err
	}
	if err := writer.Close(); err != nil {
		return encodedMarkdown{}, err
	}
	if compressed.Len() < len(raw) {
		result.stored = compressed.Bytes()
		result.encoding = "gzip"
	}
	return result, nil
}

func decodeMarkdown(content []byte, encoding, expectedHash string, maxBytes int) (string, error) {
	var raw []byte
	switch encoding {
	case "identity":
		raw = append([]byte(nil), content...)
	case "gzip":
		reader, err := gzip.NewReader(bytes.NewReader(content))
		if err != nil {
			return "", fmt.Errorf("%w: %v", ErrIntegrity, err)
		}
		raw, err = io.ReadAll(io.LimitReader(reader, int64(maxBytes)+1))
		closeErr := reader.Close()
		if err != nil {
			return "", fmt.Errorf("%w: %v", ErrIntegrity, err)
		}
		if closeErr != nil {
			return "", fmt.Errorf("%w: %v", ErrIntegrity, closeErr)
		}
	default:
		return "", fmt.Errorf("%w: unsupported encoding %q", ErrIntegrity, encoding)
	}
	if len(raw) > maxBytes {
		return "", fmt.Errorf("%w: decoded markdown exceeds %d bytes", ErrIntegrity, maxBytes)
	}
	if hashBytes(raw) != expectedHash {
		return "", fmt.Errorf("%w: content hash mismatch", ErrIntegrity)
	}
	return string(raw), nil
}

func normalizeMarkdown(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return value + "\n"
}

func hashBytes(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}
