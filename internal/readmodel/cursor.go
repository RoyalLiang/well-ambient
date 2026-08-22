package readmodel

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

var (
	ErrInvalidCursor = errors.New("invalid read cursor")
	ErrCursorScope   = errors.New("read cursor does not match the requested contract and scope")
)

const cursorVersion = 1

type CursorCodec struct {
	contract    string
	fingerprint string
}

type cursorEnvelope struct {
	Version     int             `json:"v"`
	Contract    string          `json:"contract"`
	Fingerprint string          `json:"fingerprint"`
	Watermark   string          `json:"watermark"`
	Position    json.RawMessage `json:"position"`
	Checksum    string          `json:"checksum"`
}

type cursorPayload struct {
	Version     int             `json:"v"`
	Contract    string          `json:"contract"`
	Fingerprint string          `json:"fingerprint"`
	Watermark   string          `json:"watermark"`
	Position    json.RawMessage `json:"position"`
}

func NewCursorCodec(contract string, scope any) (CursorCodec, error) {
	contract = strings.TrimSpace(contract)
	if contract == "" {
		return CursorCodec{}, fmt.Errorf("cursor contract is required")
	}
	scopeJSON, err := json.Marshal(scope)
	if err != nil {
		return CursorCodec{}, fmt.Errorf("encode cursor scope: %w", err)
	}
	digest := sha256.Sum256(scopeJSON)
	return CursorCodec{contract: contract, fingerprint: hex.EncodeToString(digest[:])}, nil
}

func (codec CursorCodec) Encode(watermark string, position any) (string, error) {
	if strings.TrimSpace(codec.contract) == "" || strings.TrimSpace(codec.fingerprint) == "" {
		return "", fmt.Errorf("cursor codec is not initialized")
	}
	positionJSON, err := json.Marshal(position)
	if err != nil {
		return "", fmt.Errorf("encode cursor position: %w", err)
	}
	payload := cursorPayload{
		Version: cursorVersion, Contract: codec.contract, Fingerprint: codec.fingerprint,
		Watermark: watermark, Position: positionJSON,
	}
	checksum, err := cursorChecksum(payload)
	if err != nil {
		return "", err
	}
	envelopeJSON, err := json.Marshal(cursorEnvelope{
		Version: payload.Version, Contract: payload.Contract, Fingerprint: payload.Fingerprint,
		Watermark: payload.Watermark, Position: payload.Position, Checksum: checksum,
	})
	if err != nil {
		return "", fmt.Errorf("encode cursor envelope: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(envelopeJSON), nil
}

func (codec CursorCodec) Decode(raw string, position any) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || position == nil {
		return "", ErrInvalidCursor
	}
	value := reflect.ValueOf(position)
	if value.Kind() != reflect.Pointer || value.IsNil() {
		return "", ErrInvalidCursor
	}
	envelopeJSON, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return "", ErrInvalidCursor
	}
	var envelope cursorEnvelope
	if err := json.Unmarshal(envelopeJSON, &envelope); err != nil {
		return "", ErrInvalidCursor
	}
	if envelope.Version != cursorVersion || envelope.Contract == "" || envelope.Fingerprint == "" || len(envelope.Position) == 0 || envelope.Checksum == "" {
		return "", ErrInvalidCursor
	}
	payload := cursorPayload{
		Version: envelope.Version, Contract: envelope.Contract, Fingerprint: envelope.Fingerprint,
		Watermark: envelope.Watermark, Position: envelope.Position,
	}
	wantChecksum, err := cursorChecksum(payload)
	if err != nil {
		return "", ErrInvalidCursor
	}
	if subtle.ConstantTimeCompare([]byte(wantChecksum), []byte(envelope.Checksum)) != 1 {
		return "", ErrInvalidCursor
	}
	if envelope.Contract != codec.contract || envelope.Fingerprint != codec.fingerprint {
		return "", ErrCursorScope
	}
	if err := json.Unmarshal(envelope.Position, position); err != nil {
		return "", ErrInvalidCursor
	}
	return envelope.Watermark, nil
}

func NormalizeLimit(raw string, defaultLimit, maxLimit int) (int, error) {
	if defaultLimit <= 0 || maxLimit <= 0 || defaultLimit > maxLimit {
		return 0, fmt.Errorf("invalid read limit policy")
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultLimit, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 {
		return 0, fmt.Errorf("limit must be a positive integer")
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	return limit, nil
}

func cursorChecksum(payload cursorPayload) (string, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode cursor checksum payload: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}
