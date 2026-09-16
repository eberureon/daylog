package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/eberureon/daylog/internal/domain"
	"github.com/google/uuid"
)

type Snapshot struct {
	Version        int                   `json:"version"`
	Tasks          []domain.Task         `json:"tasks"`
	JournalEntries []domain.JournalEntry `json:"journal_entries"`
	Projects       []SnapshotProject     `json:"projects"`
	Tags           []SnapshotTag         `json:"tags"`
	Reviews        []SnapshotReview      `json:"reviews"`
}

type SnapshotProject struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	NormalizedName string     `json:"normalized_name"`
	ArchivedAt     *time.Time `json:"archived_at,omitempty"`
}
type SnapshotTag struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	NormalizedName string     `json:"normalized_name"`
	ArchivedAt     *time.Time `json:"archived_at,omitempty"`
}
type SnapshotReview struct {
	Date        string    `json:"date"`
	CompletedAt time.Time `json:"completed_at"`
}

type SearchOptions struct {
	Query          string
	Project        string
	Tag            string
	TaskStatus     domain.TaskStatus
	TaskDate       string
	IncludeDeleted bool
}

func (s *Store) ExportSnapshot(ctx context.Context) (Snapshot, error) {
	tasks, err := s.ListTasks(ctx, true)
	if err != nil {
		return Snapshot{}, err
	}
	entries, err := s.ListJournalEntries(ctx, true)
	if err != nil {
		return Snapshot{}, err
	}
	snapshot := Snapshot{Version: 1, Tasks: tasks, JournalEntries: entries}
	for _, table := range []string{"projects", "tags"} {
		rows, err := s.db.QueryContext(ctx, `SELECT id, name, normalized_name, archived_at FROM `+table)
		if err != nil {
			return Snapshot{}, err
		}
		for rows.Next() {
			var id, name, normalized string
			var archived sql.NullString
			if err := rows.Scan(&id, &name, &normalized, &archived); err != nil {
				rows.Close()
				return Snapshot{}, err
			}
			date, err := parseStampPtr(archived)
			if err != nil {
				rows.Close()
				return Snapshot{}, err
			}
			if table == "projects" {
				snapshot.Projects = append(snapshot.Projects, SnapshotProject{id, name, normalized, date})
			} else {
				snapshot.Tags = append(snapshot.Tags, SnapshotTag{id, name, normalized, date})
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return Snapshot{}, err
		}
		rows.Close()
	}
	rows, err := s.db.QueryContext(ctx, `SELECT review_date, completed_at FROM daily_reviews`)
	if err != nil {
		return Snapshot{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var date, completed string
		if err := rows.Scan(&date, &completed); err != nil {
			return Snapshot{}, err
		}
		parsed, err := time.Parse(time.RFC3339Nano, completed)
		if err != nil {
			return Snapshot{}, err
		}
		snapshot.Reviews = append(snapshot.Reviews, SnapshotReview{date, parsed})
	}
	return snapshot, rows.Err()
}

func (s *Store) ImportSnapshot(ctx context.Context, snapshot Snapshot, replace bool) error {
	return s.ImportSnapshotWithPolicy(ctx, snapshot, replace, "import")
}

func (s *Store) ImportSnapshotWithPolicy(ctx context.Context, snapshot Snapshot, replace bool, policy string) error {
	conflicts, err := s.SnapshotConflicts(ctx, snapshot)
	if err != nil {
		return err
	}
	conflictSet := make(map[string]bool, len(conflicts))
	for _, conflict := range conflicts {
		conflictSet[conflict] = true
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if replace {
		for _, table := range []string{"journal_entry_tasks", "journal_entry_tags", "task_tags", "tasks", "journal_entries", "projects", "tags", "daily_reviews"} {
			if _, err := tx.ExecContext(ctx, "DELETE FROM "+table); err != nil {
				return err
			}
		}
	}
	for _, project := range snapshot.Projects {
		if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO projects(id, name, normalized_name, archived_at) VALUES (?, ?, ?, ?)`, project.ID, project.Name, project.NormalizedName, nullableStamp(project.ArchivedAt)); err != nil {
			return err
		}
	}
	for _, tag := range snapshot.Tags {
		if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO tags(id, name, normalized_name, archived_at) VALUES (?, ?, ?, ?)`, tag.ID, tag.Name, tag.NormalizedName, nullableStamp(tag.ArchivedAt)); err != nil {
			return err
		}
	}
	for _, task := range snapshot.Tasks {
		if policy == "local" && conflictSet["task:"+task.ID.String()] {
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO tasks(id, description, project, due_date, important, status, created_at, updated_at, completed_at, deleted_at) VALUES (?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?)`, task.ID.String(), task.Description, task.Project, nullableDate(task.DueDate), task.Important, task.Status, stamp(task.CreatedAt), stamp(task.UpdatedAt), nullableStamp(task.CompletedAt), nullableStamp(task.DeletedAt)); err != nil {
			return err
		}
		if err := setTags(ctx, tx, task.ID.String(), "task_tags", task.Tags); err != nil {
			return err
		}
	}
	for _, entry := range snapshot.JournalEntries {
		if policy == "local" && conflictSet["journal:"+entry.ID.String()] {
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO journal_entries(id, description, project, entry_date, created_at, updated_at, deleted_at) VALUES (?, ?, NULLIF(?, ''), ?, ?, ?, ?)`, entry.ID.String(), entry.Description, entry.Project, nullableDate(entry.EntryDate), stamp(entry.CreatedAt), stamp(entry.UpdatedAt), nullableStamp(entry.DeletedAt)); err != nil {
			return err
		}
		if err := setTags(ctx, tx, entry.ID.String(), "journal_entry_tags", entry.Tags); err != nil {
			return err
		}
		for _, taskID := range entry.TaskIDs {
			if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO journal_entry_tasks(journal_entry_id, task_id) SELECT ?, id FROM tasks WHERE id = ?`, entry.ID.String(), taskID.String()); err != nil {
				return err
			}
		}
	}
	for _, review := range snapshot.Reviews {
		if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO daily_reviews(review_date, completed_at) VALUES (?, ?)`, review.Date, stamp(review.CompletedAt)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) SnapshotConflicts(ctx context.Context, snapshot Snapshot) ([]string, error) {
	var conflicts []string
	for _, task := range snapshot.Tasks {
		var updated string
		err := s.db.QueryRowContext(ctx, `SELECT updated_at FROM tasks WHERE id = ?`, task.ID.String()).Scan(&updated)
		if err == nil && updated != stamp(task.UpdatedAt) {
			conflicts = append(conflicts, "task:"+task.ID.String())
		}
	}
	for _, entry := range snapshot.JournalEntries {
		var updated string
		err := s.db.QueryRowContext(ctx, `SELECT updated_at FROM journal_entries WHERE id = ?`, entry.ID.String()).Scan(&updated)
		if err == nil && updated != stamp(entry.UpdatedAt) {
			conflicts = append(conflicts, "journal:"+entry.ID.String())
		}
	}
	return conflicts, nil
}

func (s *Store) CreateTask(ctx context.Context, task domain.Task) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := ensureProject(ctx, tx, task.Project); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO tasks (id, description, project, due_date, important, status, created_at, updated_at) VALUES (?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?)`, task.ID.String(), task.Description, task.Project, nullableDate(task.DueDate), task.Important, task.Status, stamp(task.CreatedAt), stamp(task.UpdatedAt)); err != nil {
		return err
	}
	if err := setTags(ctx, tx, task.ID.String(), "task_tags", task.Tags); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) CreateJournalEntry(ctx context.Context, entry domain.JournalEntry) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := ensureProject(ctx, tx, entry.Project); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO journal_entries (id, description, project, entry_date, created_at, updated_at) VALUES (?, ?, NULLIF(?, ''), ?, ?, ?)`, entry.ID.String(), entry.Description, entry.Project, nullableDate(entry.EntryDate), stamp(entry.CreatedAt), stamp(entry.UpdatedAt)); err != nil {
		return err
	}
	if err := setTags(ctx, tx, entry.ID.String(), "journal_entry_tags", entry.Tags); err != nil {
		return err
	}
	for _, taskID := range entry.TaskIDs {
		if _, err = tx.ExecContext(ctx, `INSERT INTO journal_entry_tasks (journal_entry_id, task_id) SELECT ?, id FROM tasks WHERE id = ? AND deleted_at IS NULL`, entry.ID.String(), taskID.String()); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) UpdateTask(ctx context.Context, task domain.Task) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := ensureProject(ctx, tx, task.Project); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE tasks SET description = ?, project = NULLIF(?, ''), due_date = ?, important = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL`, task.Description, task.Project, nullableDate(task.DueDate), task.Important, stamp(task.UpdatedAt), task.ID.String())
	if err != nil {
		return err
	}
	if err := affected(result, task.ID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM task_tags WHERE task_id = ?`, task.ID.String()); err != nil {
		return err
	}
	if err := setTags(ctx, tx, task.ID.String(), "task_tags", task.Tags); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) UpdateJournalEntry(ctx context.Context, entry domain.JournalEntry) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := ensureProject(ctx, tx, entry.Project); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE journal_entries SET description = ?, project = NULLIF(?, ''), entry_date = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL`, entry.Description, entry.Project, nullableDate(entry.EntryDate), stamp(entry.UpdatedAt), entry.ID.String())
	if err != nil {
		return err
	}
	if err := affected(result, entry.ID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM journal_entry_tags WHERE journal_entry_id = ?`, entry.ID.String()); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM journal_entry_tasks WHERE journal_entry_id = ?`, entry.ID.String()); err != nil {
		return err
	}
	if err := setTags(ctx, tx, entry.ID.String(), "journal_entry_tags", entry.Tags); err != nil {
		return err
	}
	for _, taskID := range entry.TaskIDs {
		if _, err = tx.ExecContext(ctx, `INSERT INTO journal_entry_tasks (journal_entry_id, task_id) SELECT ?, id FROM tasks WHERE id = ? AND deleted_at IS NULL`, entry.ID.String(), taskID.String()); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ListTasks(ctx context.Context, includeDeleted bool) ([]domain.Task, error) {
	where := "WHERE deleted_at IS NULL"
	if includeDeleted {
		where = ""
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, description, project, due_date, important, status, created_at, updated_at, completed_at, deleted_at FROM tasks `+where+` ORDER BY CASE status WHEN 'open' THEN 0 WHEN 'done' THEN 1 ELSE 2 END, CASE WHEN due_date IS NULL THEN 1 ELSE 0 END, due_date, important DESC, created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.Task
	for rows.Next() {
		var t domain.Task
		var id, status string
		var project, due, created, updated, completed, deleted sql.NullString
		var important int
		if err := rows.Scan(&id, &t.Description, &project, &due, &important, &status, &created, &updated, &completed, &deleted); err != nil {
			return nil, err
		}
		t.ID, err = uuid.Parse(id)
		if err != nil {
			return nil, err
		}
		t.Project = project.String
		t.Important = important != 0
		t.Status = domain.TaskStatus(status)
		if t.DueDate, err = parseNullableDate(due); err != nil {
			return nil, err
		}
		if t.CreatedAt, err = parseStamp(created); err != nil {
			return nil, err
		}
		if t.UpdatedAt, err = parseStamp(updated); err != nil {
			return nil, err
		}
		if t.CompletedAt, err = parseStampPtr(completed); err != nil {
			return nil, err
		}
		if t.DeletedAt, err = parseStampPtr(deleted); err != nil {
			return nil, err
		}
		t.Tags, err = s.tagsFor(ctx, "task_tags", id)
		if err != nil {
			return nil, err
		}
		result = append(result, t)
	}
	return result, rows.Err()
}

func (s *Store) ListJournalEntries(ctx context.Context, includeDeleted bool) ([]domain.JournalEntry, error) {
	where := "WHERE deleted_at IS NULL"
	if includeDeleted {
		where = ""
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, description, project, entry_date, created_at, updated_at, deleted_at FROM journal_entries `+where+` ORDER BY COALESCE(entry_date, substr(created_at, 1, 10)) DESC, created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.JournalEntry
	for rows.Next() {
		var e domain.JournalEntry
		var id string
		var project, entryDate, created, updated, deleted sql.NullString
		if err := rows.Scan(&id, &e.Description, &project, &entryDate, &created, &updated, &deleted); err != nil {
			return nil, err
		}
		var err error
		e.ID, err = uuid.Parse(id)
		if err != nil {
			return nil, err
		}
		e.Project = project.String
		if e.EntryDate, err = parseNullableDate(entryDate); err != nil {
			return nil, err
		}
		if e.CreatedAt, err = parseStamp(created); err != nil {
			return nil, err
		}
		if e.UpdatedAt, err = parseStamp(updated); err != nil {
			return nil, err
		}
		if e.DeletedAt, err = parseStampPtr(deleted); err != nil {
			return nil, err
		}
		e.Tags, err = s.tagsFor(ctx, "journal_entry_tags", id)
		if err != nil {
			return nil, err
		}
		e.TaskIDs, err = s.taskLinks(ctx, id)
		if err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, rows.Err()
}

func (s *Store) Search(ctx context.Context, options SearchOptions) ([]domain.Task, []domain.JournalEntry, error) {
	query := strings.ToLower(strings.TrimSpace(options.Query))
	project := strings.ToLower(strings.TrimSpace(options.Project))
	tag := strings.ToLower(strings.TrimSpace(options.Tag))
	tasks, err := s.ListTasks(ctx, options.IncludeDeleted)
	if err != nil {
		return nil, nil, err
	}
	filteredTasks := tasks[:0]
	for _, task := range tasks {
		if query != "" && !strings.Contains(strings.ToLower(task.Description), query) && !strings.Contains(strings.ToLower(task.Project), query) && !containsFold(task.Tags, query) {
			continue
		}
		if project != "" && !strings.EqualFold(task.Project, project) {
			continue
		}
		if tag != "" && !containsFold(task.Tags, tag) {
			continue
		}
		if options.TaskStatus != "" && task.Status != options.TaskStatus {
			continue
		}
		if options.TaskDate != "" && (task.DueDate == nil || task.DueDate.Format("2006-01-02") != options.TaskDate) {
			continue
		}
		{
			filteredTasks = append(filteredTasks, task)
		}
	}
	entries, err := s.ListJournalEntries(ctx, options.IncludeDeleted)
	if err != nil {
		return nil, nil, err
	}
	filteredEntries := entries[:0]
	for _, entry := range entries {
		if query != "" && !strings.Contains(strings.ToLower(entry.Description), query) && !strings.Contains(strings.ToLower(entry.Project), query) && !containsFold(entry.Tags, query) {
			continue
		}
		if project != "" && !strings.EqualFold(entry.Project, project) {
			continue
		}
		if tag != "" && !containsFold(entry.Tags, tag) {
			continue
		}
		if options.TaskDate != "" {
			displayedDate := entry.CreatedAt.In(time.Local).Format("2006-01-02")
			if entry.EntryDate != nil {
				displayedDate = entry.EntryDate.In(time.Local).Format("2006-01-02")
			}
			if displayedDate != options.TaskDate {
				continue
			}
		}
		{
			filteredEntries = append(filteredEntries, entry)
		}
	}
	return filteredTasks, filteredEntries, nil
}

func containsFold(values []string, query string) bool {
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), query) {
			return true
		}
	}
	return false
}

func (s *Store) SetTaskStatus(ctx context.Context, id uuid.UUID, status domain.TaskStatus) error {
	if status != domain.TaskOpen && status != domain.TaskDone && status != domain.TaskArchived {
		return fmt.Errorf("invalid task status %q", status)
	}
	now := stamp(time.Now())
	var completed any
	if status == domain.TaskDone {
		completed = now
	}
	result, err := s.db.ExecContext(ctx, `UPDATE tasks SET status = ?, completed_at = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL`, status, completed, now, id.String())
	if err != nil {
		return err
	}
	return affected(result, id)
}

func (s *Store) SetTaskDeleted(ctx context.Context, id uuid.UUID, deleted bool) error {
	var value any
	if deleted {
		value = stamp(time.Now())
	}
	result, err := s.db.ExecContext(ctx, `UPDATE tasks SET deleted_at = ?, updated_at = ? WHERE id = ?`, value, stamp(time.Now()), id.String())
	if err != nil {
		return err
	}
	return affected(result, id)
}

func (s *Store) SetJournalDeleted(ctx context.Context, id uuid.UUID, deleted bool) error {
	var value any
	if deleted {
		value = stamp(time.Now())
	}
	result, err := s.db.ExecContext(ctx, `UPDATE journal_entries SET deleted_at = ?, updated_at = ? WHERE id = ?`, value, stamp(time.Now()), id.String())
	if err != nil {
		return err
	}
	return affected(result, id)
}

func (s *Store) MarkReview(ctx context.Context, reviewDate string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO daily_reviews(review_date, completed_at) VALUES (?, ?) ON CONFLICT(review_date) DO UPDATE SET completed_at = excluded.completed_at`, reviewDate, stamp(time.Now()))
	return err
}

func (s *Store) IsReviewed(ctx context.Context, reviewDate string) (bool, error) {
	var value int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM daily_reviews WHERE review_date = ?`, reviewDate).Scan(&value)
	return value > 0, err
}

func (s *Store) ToggleImportant(ctx context.Context, id uuid.UUID) error {
	return s.setImportant(ctx, id, `CASE important WHEN 0 THEN 1 ELSE 0 END`)
}
func (s *Store) SetImportant(ctx context.Context, id uuid.UUID, value bool) error {
	v := 0
	if value {
		v = 1
	}
	result, err := s.db.ExecContext(ctx, `UPDATE tasks SET important = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL`, v, stamp(time.Now()), id.String())
	if err != nil {
		return err
	}
	return affected(result, id)
}
func (s *Store) setImportant(ctx context.Context, id uuid.UUID, expression string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE tasks SET important = `+expression+`, updated_at = ? WHERE id = ? AND deleted_at IS NULL`, stamp(time.Now()), id.String())
	if err != nil {
		return err
	}
	return affected(result, id)
}

func (s *Store) tagsFor(ctx context.Context, relation, id string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT tags.name FROM tags JOIN `+relation+` ON tags.id = `+relation+`.tag_id WHERE `+relation+`.`+strings.TrimSuffix(relation, "_tags")+`_id = ? ORDER BY tags.name`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}
func (s *Store) taskLinks(ctx context.Context, id string) ([]uuid.UUID, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT task_id FROM journal_entry_tasks WHERE journal_entry_id = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []uuid.UUID
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		parsed, err := uuid.Parse(value)
		if err != nil {
			return nil, err
		}
		result = append(result, parsed)
	}
	return result, rows.Err()
}

func ensureProject(ctx context.Context, tx *sql.Tx, name string) error {
	if strings.TrimSpace(name) == "" {
		return nil
	}
	normalized, err := domain.NormalizeName(name)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO projects(id, name, normalized_name) VALUES (?, ?, ?)`, uuid.Must(uuid.NewV7()).String(), strings.TrimSpace(name), normalized)
	return err
}
func setTags(ctx context.Context, tx *sql.Tx, recordID, relation string, names []string) error {
	for _, name := range names {
		normalized, err := domain.NormalizeName(name)
		if err != nil {
			return err
		}
		tagID := uuid.Must(uuid.NewV7()).String()
		if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO tags(id, name, normalized_name) VALUES (?, ?, ?)`, tagID, strings.TrimSpace(name), normalized); err != nil {
			return err
		}
		var actualID string
		if err = tx.QueryRowContext(ctx, `SELECT id FROM tags WHERE normalized_name = ?`, normalized).Scan(&actualID); err != nil {
			return err
		}
		column := strings.TrimSuffix(relation, "_tags") + "_id"
		if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO `+relation+`(`+column+`, tag_id) VALUES (?, ?)`, recordID, actualID); err != nil {
			return err
		}
	}
	return nil
}
func affected(result sql.Result, id uuid.UUID) error {
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("record %s not found", id)
	}
	return nil
}
func stamp(value time.Time) string {
	return value.UTC().Truncate(time.Microsecond).Format(time.RFC3339Nano)
}
func nullableStamp(value *time.Time) any {
	if value == nil {
		return nil
	}
	return stamp(*value)
}
func parseStamp(value sql.NullString) (time.Time, error) {
	if !value.Valid {
		return time.Time{}, fmt.Errorf("missing timestamp")
	}
	return time.Parse(time.RFC3339Nano, value.String)
}
func parseStampPtr(value sql.NullString) (*time.Time, error) {
	if !value.Valid || value.String == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value.String)
	return &parsed, err
}
func nullableDate(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.Format("2006-01-02")
}
func parseNullableDate(value sql.NullString) (*time.Time, error) {
	if !value.Valid || value.String == "" {
		return nil, nil
	}
	if len(value.String) == 10 {
		parsed, err := time.ParseInLocation("2006-01-02", value.String, time.Local)
		return &parsed, err
	}
	parsed, err := time.Parse(time.RFC3339Nano, value.String)
	return &parsed, err
}
