package server

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"well-ambient/internal/dataassets"
	"well-ambient/internal/db"

	"gorm.io/gorm"
)

func (s *Server) handleListDataAssetEvents(w http.ResponseWriter, r *http.Request) {
	query, err := parseDataAssetTimelineQuery(r)
	if err != nil {
		writeDeliveryError(w, http.StatusUnprocessableEntity, "invalid_data_asset_query", err.Error())
		return
	}
	page, err := dataassets.New(db.DB).Timeline(r.Context(), query)
	if err != nil {
		writeDataAssetError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) handleGetDataAssetEvent(w http.ResponseWriter, r *http.Request) {
	eventID, err := parsePositiveUint(r.PathValue("id"))
	if err != nil {
		writeDeliveryError(w, http.StatusBadRequest, "invalid_data_asset_event_id", "event id must be a positive integer")
		return
	}
	record, err := dataassets.New(db.DB).Load(r.Context(), eventID)
	if err != nil {
		writeDataAssetError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (s *Server) handleGetDataAssetSnapshot(w http.ResponseWriter, r *http.Request) {
	snapshotID, err := parsePositiveUint(r.PathValue("id"))
	if err != nil {
		writeDeliveryError(w, http.StatusBadRequest, "invalid_data_asset_snapshot_id", "snapshot id must be a positive integer")
		return
	}
	record, err := dataassets.New(db.DB).LoadSnapshot(r.Context(), snapshotID)
	if err != nil {
		writeDataAssetError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (s *Server) handleGetLatestDataAssetSnapshot(w http.ResponseWriter, r *http.Request) {
	query := dataassets.LatestSnapshotQuery{
		Kind:      r.URL.Query().Get("kind"),
		ScopeType: r.URL.Query().Get("scope_type"),
		ScopeID:   r.URL.Query().Get("scope_id"),
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("as_of")); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeDeliveryError(w, http.StatusUnprocessableEntity, "invalid_data_asset_as_of", "as_of must be RFC3339")
			return
		}
		query.AsOf = &parsed
	}
	record, err := dataassets.New(db.DB).LatestSnapshot(r.Context(), query)
	if err != nil {
		writeDataAssetError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func parseDataAssetTimelineQuery(r *http.Request) (dataassets.TimelineQuery, error) {
	limit := 100
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			return dataassets.TimelineQuery{}, errors.New("limit must be a positive integer")
		}
		if parsed > 200 {
			parsed = 200
		}
		limit = parsed
	}
	query := dataassets.TimelineQuery{
		ProjectKey:     r.URL.Query().Get("project_key"),
		SubjectType:    r.URL.Query().Get("subject_type"),
		SubjectID:      r.URL.Query().Get("subject_id"),
		EventTypes:     splitDataAssetQueryValues(r.URL.Query()["event_type"]),
		SourceSystem:   r.URL.Query().Get("source_system"),
		SourceRecordID: r.URL.Query().Get("source_record_id"),
		CorrelationID:  r.URL.Query().Get("correlation_id"),
		Limit:          limit,
		Cursor:         r.URL.Query().Get("cursor"),
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("since")); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return dataassets.TimelineQuery{}, errors.New("since must be RFC3339")
		}
		query.Since = &parsed
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("until")); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return dataassets.TimelineQuery{}, errors.New("until must be RFC3339")
		}
		query.Until = &parsed
	}
	return query, nil
}

func splitDataAssetQueryValues(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		for _, item := range strings.Split(value, ",") {
			if item = strings.TrimSpace(item); item != "" {
				result = append(result, item)
			}
		}
	}
	return result
}

func parsePositiveUint(value string) (uint, error) {
	parsed, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed == 0 {
		return 0, errors.New("positive integer required")
	}
	return uint(parsed), nil
}

func writeDataAssetError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		writeDeliveryError(w, http.StatusNotFound, "data_asset_not_found", "data asset was not found")
	case errors.Is(err, dataassets.ErrQueryScopeRequired),
		errors.Is(err, dataassets.ErrCursorInvalid),
		errors.Is(err, dataassets.ErrCursorFilterMismatch),
		errors.Is(err, dataassets.ErrInvalidCommand):
		writeDeliveryError(w, http.StatusUnprocessableEntity, "invalid_data_asset_query", err.Error())
	case errors.Is(err, dataassets.ErrPayloadIntegrity):
		writeDeliveryError(w, http.StatusConflict, "data_asset_integrity_failed", err.Error())
	default:
		writeDeliveryError(w, http.StatusInternalServerError, "data_asset_query_failed", err.Error())
	}
}
