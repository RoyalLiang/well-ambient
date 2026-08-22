package readmodel

import (
	"errors"
	"testing"
)

func TestCursorCodecBindsPositionToContractAndScope(t *testing.T) {
	codec, err := NewCursorCodec("context-facts", map[string]any{"status": "active", "projects": []string{"HIT"}})
	if err != nil {
		t.Fatalf("create codec: %v", err)
	}
	type position struct {
		UpdatedUnix int64  `json:"updated_unix"`
		ID          uint   `json:"id"`
		TieBreaker  string `json:"tie_breaker"`
	}
	want := position{UpdatedUnix: 1234, ID: 88, TieBreaker: "HIT-1"}
	cursor, err := codec.Encode("watermark-42", want)
	if err != nil {
		t.Fatalf("encode cursor: %v", err)
	}
	var got position
	watermark, err := codec.Decode(cursor, &got)
	if err != nil {
		t.Fatalf("decode cursor: %v", err)
	}
	if watermark != "watermark-42" || got != want {
		t.Fatalf("decoded watermark/position = %q/%+v, want watermark-42/%+v", watermark, got, want)
	}

	otherScope, err := NewCursorCodec("context-facts", map[string]any{"status": "archived", "projects": []string{"HIT"}})
	if err != nil {
		t.Fatalf("create other codec: %v", err)
	}
	if _, err := otherScope.Decode(cursor, &got); !errors.Is(err, ErrCursorScope) {
		t.Fatalf("scope mismatch error = %v, want ErrCursorScope", err)
	}
	otherContract, err := NewCursorCodec("context-documents", map[string]any{"status": "active", "projects": []string{"HIT"}})
	if err != nil {
		t.Fatalf("create other contract codec: %v", err)
	}
	if _, err := otherContract.Decode(cursor, &got); !errors.Is(err, ErrCursorScope) {
		t.Fatalf("contract mismatch error = %v, want ErrCursorScope", err)
	}
}

func TestCursorCodecRejectsTampering(t *testing.T) {
	codec, err := NewCursorCodec("audit", map[string]string{"actor": "eddie"})
	if err != nil {
		t.Fatalf("create codec: %v", err)
	}
	cursor, err := codec.Encode("9", struct {
		ID uint `json:"id"`
	}{ID: 9})
	if err != nil {
		t.Fatalf("encode cursor: %v", err)
	}
	tampered := cursor[:len(cursor)-1] + "A"
	var position struct {
		ID uint `json:"id"`
	}
	if _, err := codec.Decode(tampered, &position); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("tampered cursor error = %v, want ErrInvalidCursor", err)
	}
}

func TestNormalizeLimitUsesDefaultsCapsAndRejectsInvalidValues(t *testing.T) {
	checks := []struct {
		raw        string
		defaultVal int
		max        int
		want       int
		wantErr    bool
	}{
		{raw: "", defaultVal: 50, max: 100, want: 50},
		{raw: "20", defaultVal: 50, max: 100, want: 20},
		{raw: "1000", defaultVal: 50, max: 100, want: 100},
		{raw: "0", defaultVal: 50, max: 100, wantErr: true},
		{raw: "bad", defaultVal: 50, max: 100, wantErr: true},
	}
	for _, check := range checks {
		got, err := NormalizeLimit(check.raw, check.defaultVal, check.max)
		if (err != nil) != check.wantErr || (!check.wantErr && got != check.want) {
			t.Errorf("NormalizeLimit(%q, %d, %d) = %d, %v; want %d, error=%v", check.raw, check.defaultVal, check.max, got, err, check.want, check.wantErr)
		}
	}
}
