package server

import (
	"errors"
	"net/http"

	"well-ambient/internal/readmodel"
)

type readPageMeta struct {
	Limit      int    `json:"limit"`
	HasMore    bool   `json:"has_more"`
	NextCursor string `json:"next_cursor,omitempty"`
	Generation uint64 `json:"generation"`
}

func buildReadPageMeta(window readmodel.PageWindow, hasMore bool, lastPosition any) (readPageMeta, error) {
	page := readPageMeta{Limit: window.Limit, HasMore: hasMore, Generation: window.Generation}
	if !hasMore {
		return page, nil
	}
	next, err := window.Next(lastPosition)
	if err != nil {
		return readPageMeta{}, err
	}
	page.NextCursor = next
	return page, nil
}

func writeReadPageError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, readmodel.ErrStaleCursor):
		writeJSON(w, http.StatusConflict, map[string]any{"error": "stale_cursor", "message": "data changed; restart from the first page"})
	case errors.Is(err, readmodel.ErrCursorScope), errors.Is(err, readmodel.ErrInvalidCursor):
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_cursor", "message": err.Error()})
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_page", "message": err.Error()})
	}
}
