package dailyjira

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
)

type Bucket string
type Direction string

const (
	BucketToday        Bucket    = "today"
	BucketRecentWatch  Bucket    = "recent_watch"
	BucketThreeDay     Bucket    = "three_day"
	BucketSevenDay     Bucket    = "seven_day"
	BucketUnclassified Bucket    = "unclassified"
	DirectionNext      Direction = "next"
	DirectionPrevious  Direction = "previous"

	defaultPageLimit = 100
	maxPageLimit     = 100
	cursorVersion    = 2
)

var (
	ErrInvalidCursor = errors.New("invalid Daily Jira cursor")
	ErrStaleCursor   = errors.New("Daily Jira cursor belongs to an older projection generation")
)

type Scope struct {
	ProjectKeys     []string
	Assignees       []string
	FilterAssignees bool
}

type Query struct {
	Bucket    Bucket
	Search    string
	Cursor    string
	Direction Direction
	Limit     int
	Scope     Scope
	Now       time.Time
}

type Summary struct {
	Total        int64
	Today        int64
	ThreeDay     int64
	SevenDay     int64
	RecentWatch  int64
	Unclassified int64
}

type Item struct {
	TaskID         string     `gorm:"column:task_id"`
	Title          string     `gorm:"column:title"`
	ProjectKey     string     `gorm:"column:project_key"`
	Project        string     `gorm:"column:project"`
	Assignee       string     `gorm:"column:assignee"`
	Reporter       string     `gorm:"column:reporter"`
	Status         string     `gorm:"column:status"`
	IssueType      string     `gorm:"column:issue_type"`
	TaskCreatedAt  time.Time  `gorm:"column:task_created_at"`
	LastActivityAt time.Time  `gorm:"column:last_activity_at"`
	DueDate        *time.Time `gorm:"column:due_date"`
	DecisionLogs   string     `gorm:"column:decision_logs"`
	Bucket         Bucket     `gorm:"column:bucket"`
	Overdue        bool       `gorm:"column:overdue"`
	CreatedUnix    int64      `gorm:"column:created_unix"`
	ActivityUnix   int64      `gorm:"column:activity_unix"`
}

type Page struct {
	GeneratedAt    time.Time
	Generation     int64
	SearchMode     string
	Summary        Summary
	Items          []Item
	PreviousCursor string
	HasPrevious    bool
	NextCursor     string
	HasMore        bool
}

type Reader struct {
	conn *gorm.DB
}

type pageCursor struct {
	Version      int    `json:"v"`
	Generation   int64  `json:"g"`
	Fingerprint  string `json:"f"`
	SortOverdue  int    `json:"o"`
	CreatedUnix  int64  `json:"c"`
	ActivityUnix int64  `json:"a"`
	TaskID       string `json:"t"`
}

func NewReader(conn *gorm.DB) *Reader {
	return &Reader{conn: conn}
}

func (r *Reader) ReadPage(ctx context.Context, query Query) (Page, error) {
	if r == nil || r.conn == nil {
		return Page{}, fmt.Errorf("Daily Jira read model is not initialized")
	}
	query = normalizeQuery(query)
	if query.Direction == DirectionPrevious && query.Cursor == "" {
		return Page{}, ErrInvalidCursor
	}
	fingerprint := queryFingerprint(query)
	now := query.Now
	if now.IsZero() {
		now = time.Now()
	}

	var result Page
	transactionOptions := &sql.TxOptions{ReadOnly: true}
	if r.conn.Dialector.Name() == "postgres" {
		transactionOptions.Isolation = sql.LevelRepeatableRead
	}
	err := r.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var state struct {
			Generation int64  `gorm:"column:generation"`
			SearchMode string `gorm:"column:search_mode"`
		}
		if tx.Dialector.Name() == "postgres" {
			if err := tx.Raw("SELECT generation FROM read_model_generations WHERE dataset = ?", "task_facts").Scan(&state).Error; err != nil {
				return fmt.Errorf("read PostgreSQL Daily Jira generation: %w", err)
			}
			state.SearchMode = "postgres_prefix"
		} else if err := tx.Raw("SELECT generation, search_mode FROM daily_jira_audit_state WHERE id = 1").Scan(&state).Error; err != nil {
			return fmt.Errorf("read Daily Jira generation: %w", err)
		}
		if state.Generation <= 0 {
			return fmt.Errorf("Daily Jira generation is not initialized")
		}

		var cursor *pageCursor
		if query.Cursor != "" {
			decoded, err := decodeCursor(query.Cursor)
			if err != nil {
				return err
			}
			if decoded.Fingerprint != fingerprint {
				return ErrInvalidCursor
			}
			if decoded.Generation != state.Generation {
				return ErrStaleCursor
			}
			cursor = &decoded
		}

		summary, err := readSummary(tx, query.Scope)
		if err != nil {
			return err
		}
		items, err := readItems(tx, query, cursor, state.SearchMode)
		if err != nil {
			return err
		}

		result = Page{
			GeneratedAt: now,
			Generation:  state.Generation,
			SearchMode:  state.SearchMode,
			Summary:     summary,
			Items:       items,
		}
		hasDirectionalContinuation := len(result.Items) > query.Limit
		if hasDirectionalContinuation {
			result.Items = result.Items[:query.Limit]
		}
		if query.Direction == DirectionPrevious {
			for left, right := 0, len(result.Items)-1; left < right; left, right = left+1, right-1 {
				result.Items[left], result.Items[right] = result.Items[right], result.Items[left]
			}
			result.HasPrevious = hasDirectionalContinuation
			result.HasMore = cursor != nil && len(result.Items) > 0
		} else {
			result.HasPrevious = cursor != nil && len(result.Items) > 0
			result.HasMore = hasDirectionalContinuation
		}
		if result.HasPrevious {
			encoded, err := encodeItemCursor(result.Items[0], state.Generation, fingerprint)
			if err != nil {
				return err
			}
			result.PreviousCursor = encoded
		}
		if result.HasMore {
			encoded, err := encodeItemCursor(result.Items[len(result.Items)-1], state.Generation, fingerprint)
			if err != nil {
				return err
			}
			result.NextCursor = encoded
		}
		return nil
	}, transactionOptions)
	if err != nil {
		return Page{}, err
	}
	return result, nil
}

func readItems(tx *gorm.DB, query Query, cursor *pageCursor, searchMode string) ([]Item, error) {
	if tx.Dialector.Name() == "postgres" {
		return readPostgresItems(tx, query, cursor)
	}
	var sqlBuilder strings.Builder
	sqlBuilder.WriteString(`SELECT e.task_id, e.title, e.project_key, e.project, e.assignee, e.reporter,
		e.status, e.issue_type, e.task_created_at, e.last_activity_at, e.due_date,
		e.decision_logs, e.bucket, e.overdue, e.created_unix, e.activity_unix
		FROM daily_jira_audit_entries AS e`)
	args := make([]any, 0, 16)
	search := strings.TrimSpace(query.Search)
	useFTS := searchMode == "fts5_trigram" && utf8.RuneCountInString(search) >= 3
	if useFTS {
		sqlBuilder.WriteString(" JOIN daily_jira_audit_search ON daily_jira_audit_search.rowid = e.rowid")
	} else if search != "" {
		sqlBuilder.WriteString(` JOIN (
			SELECT task_id FROM daily_jira_audit_entries INDEXED BY idx_daily_jira_short_task_search
				WHERE bucket = ? AND task_id GLOB ?
			UNION
			SELECT task_id FROM daily_jira_audit_entries INDEXED BY idx_daily_jira_short_title_search
				WHERE bucket = ? AND title_key GLOB ?
			UNION
			SELECT task_id FROM daily_jira_audit_entries INDEXED BY idx_daily_jira_short_project_search
				WHERE bucket = ? AND project_key GLOB ?
			UNION
			SELECT task_id FROM daily_jira_audit_entries INDEXED BY idx_daily_jira_short_assignee_search
				WHERE bucket = ? AND assignee_key GLOB ?
			UNION
			SELECT task_id FROM daily_jira_audit_entries INDEXED BY idx_daily_jira_short_status_search
				WHERE bucket = ? AND status_key GLOB ?
		) AS matching ON matching.task_id = e.task_id`)
		args = append(args,
			string(query.Bucket), strings.ToUpper(search)+"*",
			string(query.Bucket), strings.ToLower(search)+"*",
			string(query.Bucket), strings.ToUpper(search)+"*",
			string(query.Bucket), strings.ToLower(search)+"*",
			string(query.Bucket), strings.ToLower(search)+"*",
		)
	}
	sqlBuilder.WriteString(" WHERE e.bucket = ?")
	args = append(args, string(query.Bucket))
	appendScopePredicate(&sqlBuilder, &args, query.Scope, "e.")

	if search != "" {
		if useFTS {
			sqlBuilder.WriteString(" AND daily_jira_audit_search MATCH ?")
			args = append(args, quoteFTS(search))
		}
	}
	if cursor != nil {
		operator := ">"
		if query.Direction == DirectionPrevious {
			operator = "<"
		}
		sqlBuilder.WriteString(` AND (e.sort_overdue, e.created_unix, e.activity_unix, e.task_id) `)
		sqlBuilder.WriteString(operator)
		sqlBuilder.WriteString(` (?, ?, ?, ?)`)
		args = append(args,
			cursor.SortOverdue, cursor.CreatedUnix, cursor.ActivityUnix, cursor.TaskID,
		)
	}
	order := "ASC"
	if query.Direction == DirectionPrevious {
		order = "DESC"
	}
	sqlBuilder.WriteString(" ORDER BY e.sort_overdue ")
	sqlBuilder.WriteString(order)
	sqlBuilder.WriteString(", e.created_unix ")
	sqlBuilder.WriteString(order)
	sqlBuilder.WriteString(", e.activity_unix ")
	sqlBuilder.WriteString(order)
	sqlBuilder.WriteString(", e.task_id ")
	sqlBuilder.WriteString(order)
	sqlBuilder.WriteString(" LIMIT ?")
	args = append(args, query.Limit+1)

	var items []Item
	if err := tx.Raw(sqlBuilder.String(), args...).Scan(&items).Error; err != nil {
		return nil, fmt.Errorf("read Daily Jira page: %w", err)
	}
	return items, nil
}

func readSummary(tx *gorm.DB, scope Scope) (Summary, error) {
	if tx.Dialector.Name() == "postgres" {
		return readPostgresSummary(tx, scope)
	}
	var sqlBuilder strings.Builder
	sqlBuilder.WriteString("SELECT bucket, SUM(item_count) AS item_count FROM daily_jira_audit_counts WHERE item_count > 0")
	args := make([]any, 0, 8)
	appendScopePredicate(&sqlBuilder, &args, scope, "")
	sqlBuilder.WriteString(" GROUP BY bucket")

	var rows []struct {
		Bucket    Bucket `gorm:"column:bucket"`
		ItemCount int64  `gorm:"column:item_count"`
	}
	if err := tx.Raw(sqlBuilder.String(), args...).Scan(&rows).Error; err != nil {
		return Summary{}, fmt.Errorf("read Daily Jira summary: %w", err)
	}
	var summary Summary
	for _, row := range rows {
		switch row.Bucket {
		case BucketToday:
			summary.Today = row.ItemCount
		case BucketThreeDay:
			summary.ThreeDay = row.ItemCount
		case BucketSevenDay:
			summary.SevenDay = row.ItemCount
		case BucketRecentWatch:
			summary.RecentWatch = row.ItemCount
		case BucketUnclassified:
			summary.Unclassified = row.ItemCount
		}
	}
	summary.Total = summary.Today + summary.ThreeDay + summary.SevenDay
	return summary, nil
}

func appendScopePredicate(builder *strings.Builder, args *[]any, scope Scope, prefix string) {
	projects := normalizeProjects(scope.ProjectKeys)
	if len(projects) > 0 {
		builder.WriteString(" AND ")
		builder.WriteString(prefix)
		builder.WriteString("project_key IN (")
		appendPlaceholders(builder, len(projects))
		builder.WriteString(")")
		for _, project := range projects {
			*args = append(*args, project)
		}
	}
	if scope.FilterAssignees {
		assignees := normalizeAssignees(scope.Assignees)
		assignees = append(assignees, "")
		builder.WriteString(" AND ")
		builder.WriteString(prefix)
		builder.WriteString("assignee_key IN (")
		appendPlaceholders(builder, len(assignees))
		builder.WriteString(")")
		for _, assignee := range assignees {
			*args = append(*args, assignee)
		}
	}
}

func appendPlaceholders(builder *strings.Builder, count int) {
	for index := 0; index < count; index++ {
		if index > 0 {
			builder.WriteByte(',')
		}
		builder.WriteByte('?')
	}
}

func normalizeQuery(query Query) Query {
	switch query.Bucket {
	case BucketToday, BucketThreeDay, BucketSevenDay:
	default:
		query.Bucket = BucketSevenDay
	}
	if query.Limit <= 0 {
		query.Limit = defaultPageLimit
	}
	if query.Limit > maxPageLimit {
		query.Limit = maxPageLimit
	}
	query.Search = strings.TrimSpace(query.Search)
	if query.Direction != DirectionPrevious {
		query.Direction = DirectionNext
	}
	query.Scope.ProjectKeys = normalizeProjects(query.Scope.ProjectKeys)
	query.Scope.Assignees = normalizeAssignees(query.Scope.Assignees)
	return query
}

func normalizeProjects(values []string) []string {
	return normalizeStrings(values, strings.ToUpper)
}

func normalizeAssignees(values []string) []string {
	return normalizeStrings(values, strings.ToLower)
}

func normalizeStrings(values []string, convert func(string) string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = convert(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func queryFingerprint(query Query) string {
	payload, _ := json.Marshal(struct {
		Bucket          Bucket   `json:"bucket"`
		Search          string   `json:"search"`
		ProjectKeys     []string `json:"projects"`
		Assignees       []string `json:"assignees"`
		FilterAssignees bool     `json:"filter_assignees"`
	}{query.Bucket, query.Search, query.Scope.ProjectKeys, query.Scope.Assignees, query.Scope.FilterAssignees})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:12])
}

func encodeCursor(cursor pageCursor) (string, error) {
	payload, err := json.Marshal(cursor)
	if err != nil {
		return "", fmt.Errorf("encode Daily Jira cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func encodeItemCursor(item Item, generation int64, fingerprint string) (string, error) {
	return encodeCursor(pageCursor{
		Version:      cursorVersion,
		Generation:   generation,
		Fingerprint:  fingerprint,
		SortOverdue:  boolInt(!item.Overdue),
		CreatedUnix:  item.CreatedUnix,
		ActivityUnix: item.ActivityUnix,
		TaskID:       item.TaskID,
	})
}

func decodeCursor(value string) (pageCursor, error) {
	payload, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return pageCursor{}, ErrInvalidCursor
	}
	var cursor pageCursor
	if err := json.Unmarshal(payload, &cursor); err != nil || cursor.Version != cursorVersion || cursor.TaskID == "" {
		return pageCursor{}, ErrInvalidCursor
	}
	return cursor, nil
}

func quoteFTS(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func dayNumber(value time.Time) int64 {
	year, month, day := value.Date()
	utcDay := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	return utcDay.Unix()/86400 + 2440587
}

type RolloverResult struct {
	Processed int64
	Done      bool
}

// RollForwardBatch advances time-derived buckets outside the latency-sensitive read path.
// Each call updates at most batchLimit candidates per transition type and commits its own
// projection generation, so readers either see the old or the completed batch atomically.
func RollForwardBatch(ctx context.Context, conn *gorm.DB, now time.Time, batchLimit int) (RolloverResult, error) {
	if conn == nil {
		return RolloverResult{}, fmt.Errorf("Daily Jira read model is not initialized")
	}
	if conn.Dialector.Name() == "postgres" {
		// PostgreSQL derives time buckets from CURRENT_DATE in each repeatable
		// read transaction, so it does not require a persisted rollover job.
		return RolloverResult{Done: true}, nil
	}
	if now.IsZero() {
		now = time.Now()
	}
	if batchLimit <= 0 {
		batchLimit = 5000
	}
	today := dayNumber(now)
	var result RolloverResult
	err := conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var currentDay int64
		if err := tx.Raw("SELECT rollover_day FROM daily_jira_audit_state WHERE id = 1").Scan(&currentDay).Error; err != nil {
			return fmt.Errorf("read Daily Jira rollover day: %w", err)
		}
		if currentDay == today {
			result.Done = true
			return nil
		}
		bucketSQL := `UPDATE daily_jira_audit_entries
		SET bucket = CASE
			WHEN created_day IS NULL OR ? - created_day < 0 THEN 'unclassified'
			WHEN ? - created_day = 0 THEN 'today'
			WHEN ? - created_day BETWEEN 1 AND 2 THEN 'recent_watch'
			WHEN ? - created_day BETWEEN 3 AND 6 THEN 'three_day'
			ELSE 'seven_day'
		END,
		next_bucket_day = CASE
			WHEN created_day IS NULL OR ? - created_day < 0 THEN NULL
			WHEN ? - created_day = 0 THEN created_day + 1
			WHEN ? - created_day BETWEEN 1 AND 2 THEN created_day + 3
			WHEN ? - created_day BETWEEN 3 AND 6 THEN created_day + 7
			ELSE NULL
		END,
		updated_at = CURRENT_TIMESTAMP
		WHERE rowid IN (
			SELECT rowid FROM daily_jira_audit_entries
			WHERE next_bucket_day IS NOT NULL AND next_bucket_day <= ?
			ORDER BY next_bucket_day, rowid LIMIT ?
		)`
		bucketResult := tx.Exec(bucketSQL, today, today, today, today, today, today, today, today, today, batchLimit)
		if bucketResult.Error != nil {
			return fmt.Errorf("roll Daily Jira age buckets: %w", bucketResult.Error)
		}
		overdueResult := tx.Exec(`UPDATE daily_jira_audit_entries
		SET overdue = 1, sort_overdue = 0, updated_at = CURRENT_TIMESTAMP
		WHERE rowid IN (
			SELECT rowid FROM daily_jira_audit_entries
			WHERE overdue = 0 AND due_day IS NOT NULL AND due_day < ?
			ORDER BY due_day, rowid LIMIT ?
		)`, today, batchLimit)
		if overdueResult.Error != nil {
			return fmt.Errorf("roll Daily Jira overdue state: %w", overdueResult.Error)
		}
		result.Processed = bucketResult.RowsAffected + overdueResult.RowsAffected
		var remaining int
		if err := tx.Raw(`SELECT EXISTS(
		SELECT 1 FROM daily_jira_audit_entries
		WHERE (next_bucket_day IS NOT NULL AND next_bucket_day <= ?)
			OR (overdue = 0 AND due_day IS NOT NULL AND due_day < ?)
		LIMIT 1
	)`, today, today).Scan(&remaining).Error; err != nil {
			return fmt.Errorf("check Daily Jira rollover backlog: %w", err)
		}
		result.Done = remaining == 0
		if result.Processed > 0 || result.Done {
			stateSQL := "UPDATE daily_jira_audit_state SET generation = generation + 1, updated_at = CURRENT_TIMESTAMP WHERE id = 1"
			args := []any{}
			if result.Done {
				stateSQL = "UPDATE daily_jira_audit_state SET generation = generation + 1, rollover_day = ?, updated_at = CURRENT_TIMESTAMP WHERE id = 1"
				args = append(args, today)
			}
			if err := tx.Exec(stateSQL, args...).Error; err != nil {
				return fmt.Errorf("advance Daily Jira rollover generation: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return RolloverResult{}, err
	}
	return result, nil
}
