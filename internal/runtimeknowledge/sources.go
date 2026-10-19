package runtimeknowledge

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"well-ambient/internal/db"
)

const contentLimit = 16000

var kinds = []string{"jira", "solution", "context_fact", "context_document", "review", "capability"}

func hash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func clip(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	value = value[:limit]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value
}

func item(kind, id, scope, scopeID, version, title, content, claim string, updated time.Time) db.RuntimeKnowledgeItem {
	scope = strings.ToLower(strings.TrimSpace(scope))
	if scope == "repository" {
		scope = "repo"
	}
	if scope == "project" {
		scopeID = strings.ToUpper(strings.TrimSpace(scopeID))
	}
	if scope == "repo" {
		scopeID = strings.ToLower(strings.TrimSpace(scopeID))
	}
	title = clip(title, 512)
	fullHash := hash(content)
	return db.RuntimeKnowledgeItem{
		Kind: kind, SourceID: id, Scope: scope, ScopeID: scopeID,
		SourceRef: kind + ":" + id + "@" + version, Title: title,
		Content: clip(content, contentLimit), ContentHash: fullHash,
		FullContent: content,
		Fingerprint: hash(strings.Join([]string{kind, id, scope, scopeID, version, title, fullHash, updated.UTC().Format(time.RFC3339Nano)}, "\x00")),
		Claim:       claim, SourceUpdatedAt: updated, Truncated: len(content) > contentLimit,
	}
}

func decode(content []byte, encoding, expectedHash string) (string, error) {
	const limit = 4 << 20
	var raw []byte
	switch encoding {
	case "identity":
		raw = content
	case "gzip":
		reader, err := gzip.NewReader(bytes.NewReader(content))
		if err != nil {
			return "", err
		}
		defer reader.Close()
		raw, err = io.ReadAll(io.LimitReader(reader, limit+1))
		if err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("unsupported source encoding")
	}
	if len(raw) > limit || (expectedHash != "" && hash(string(raw)) != expectedHash) {
		return "", fmt.Errorf("source integrity failure")
	}
	return string(raw), nil
}

// Index every source segment so a topic near the end of a long design remains
// retrievable. Each segment retains the hash and version of the entire source.
func chunks(entry db.RuntimeKnowledgeItem) []db.RuntimeKnowledgeItem {
	text := entry.FullContent
	parts := []string{}
	for len(text) > 0 {
		part := clip(text, contentLimit)
		parts = append(parts, part)
		if len(part) == len(text) {
			break
		}
		// Overlap preserves identifiers and Chinese terms crossing a segment edge.
		step := len(part) - 256
		for step > 0 && (text[step]&0xc0) == 0x80 {
			step--
		}
		text = text[step:]
	}
	if len(parts) == 0 {
		parts = append(parts, "")
	}
	result := make([]db.RuntimeKnowledgeItem, 0, len(parts))
	for i, part := range parts {
		chunk := entry
		chunk.FullContent = ""
		chunk.Content = part
		chunk.ChunkIndex = i + 1
		chunk.ChunkCount = len(parts)
		chunk.Truncated = len(parts) > 1
		if len(parts) > 1 {
			chunk.SourceRef = fmt.Sprintf("%s#chunk=%d", entry.SourceRef, i+1)
		}
		chunk.Fingerprint = hash(fmt.Sprintf("%s\x00chunk=%d/%d", entry.Fingerprint, i+1, len(parts)))
		result = append(result, chunk)
	}
	return result
}

func matchingChunk(current, stored db.RuntimeKnowledgeItem) (db.RuntimeKnowledgeItem, bool) {
	parts := chunks(current)
	if stored.ChunkIndex < 1 || stored.ChunkIndex > len(parts) {
		return current, false
	}
	return parts[stored.ChunkIndex-1], true
}
