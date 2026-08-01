package dataassets

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
)

type encodedPayload struct {
	raw      []byte
	stored   []byte
	encoding string
	hash     string
}

func encodePayload(value any, compressionThreshold int, maxBytes int64) (encodedPayload, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return encodedPayload{}, fmt.Errorf("%w: payload is not JSON serializable: %v", ErrInvalidCommand, err)
	}
	if int64(len(raw)) > maxBytes {
		return encodedPayload{}, fmt.Errorf("%w: got %d bytes, limit is %d", ErrPayloadTooLarge, len(raw), maxBytes)
	}
	result := encodedPayload{
		raw:      raw,
		stored:   raw,
		encoding: "identity",
		hash:     hashBytes(raw),
	}
	if compressionThreshold <= 0 || len(raw) < compressionThreshold {
		return result, nil
	}
	var compressed bytes.Buffer
	writer, err := gzip.NewWriterLevel(&compressed, gzip.BestSpeed)
	if err != nil {
		return encodedPayload{}, err
	}
	if _, err := writer.Write(raw); err != nil {
		return encodedPayload{}, err
	}
	if err := writer.Close(); err != nil {
		return encodedPayload{}, err
	}
	if compressed.Len() < len(raw) {
		result.stored = compressed.Bytes()
		result.encoding = "gzip"
	}
	return result, nil
}

func decodePayload(data []byte, encoding, expectedHash string, maxBytes int64) ([]byte, error) {
	var raw []byte
	switch encoding {
	case "identity":
		raw = append([]byte(nil), data...)
	case "gzip":
		reader, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrPayloadIntegrity, err)
		}
		limited := io.LimitReader(reader, maxBytes+1)
		raw, err = io.ReadAll(limited)
		closeErr := reader.Close()
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrPayloadIntegrity, err)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("%w: %v", ErrPayloadIntegrity, closeErr)
		}
	default:
		return nil, fmt.Errorf("%w: unsupported encoding %q", ErrPayloadIntegrity, encoding)
	}
	if int64(len(raw)) > maxBytes {
		return nil, fmt.Errorf("%w: decoded payload exceeds %d bytes", ErrPayloadIntegrity, maxBytes)
	}
	if hashBytes(raw) != expectedHash {
		return nil, fmt.Errorf("%w: hash mismatch", ErrPayloadIntegrity)
	}
	return raw, nil
}

func hashBytes(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func hashJSON(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return hashBytes(data), nil
}
