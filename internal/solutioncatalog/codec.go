package solutioncatalog

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
)

const (
	defaultCompressionThreshold = 1024
	maxCatalogPayloadBytes      = 4 * 1024 * 1024
)

type encodedPayload struct {
	stored   []byte
	encoding string
	hash     string
	rawBytes int
}

func encodePayload(raw []byte) (encodedPayload, error) {
	if len(raw) > maxCatalogPayloadBytes {
		return encodedPayload{}, fmt.Errorf("catalog payload exceeds %d bytes", maxCatalogPayloadBytes)
	}
	hash := sha256.Sum256(raw)
	result := encodedPayload{
		stored: append([]byte(nil), raw...), encoding: "identity",
		hash: hex.EncodeToString(hash[:]), rawBytes: len(raw),
	}
	if len(raw) < defaultCompressionThreshold {
		return result, nil
	}
	var compressed bytes.Buffer
	writer, err := gzip.NewWriterLevel(&compressed, gzip.BestSpeed)
	if err != nil {
		return encodedPayload{}, err
	}
	if _, err := writer.Write(raw); err != nil {
		_ = writer.Close()
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

func decodePayload(stored []byte, encoding, expectedHash string) ([]byte, error) {
	var raw []byte
	switch encoding {
	case "", "identity":
		raw = append([]byte(nil), stored...)
	case "gzip":
		reader, err := gzip.NewReader(bytes.NewReader(stored))
		if err != nil {
			return nil, err
		}
		decompressed, err := io.ReadAll(io.LimitReader(reader, maxCatalogPayloadBytes+1))
		closeErr := reader.Close()
		if err != nil {
			return nil, err
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if len(decompressed) > maxCatalogPayloadBytes {
			return nil, fmt.Errorf("decoded catalog payload exceeds %d bytes", maxCatalogPayloadBytes)
		}
		raw = decompressed
	default:
		return nil, fmt.Errorf("unsupported catalog payload encoding %q", encoding)
	}
	hash := sha256.Sum256(raw)
	if expectedHash != "" && hex.EncodeToString(hash[:]) != expectedHash {
		return nil, ErrIntegrity
	}
	return raw, nil
}
