package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"task-team-api/internal/models"
)

var ErrNotFound = errors.New("not found")

type Repository struct {
	db *sql.DB
}

func New(db *sql.DB) *Repository { return &Repository{db: db} }

func (r *Repository) CreateUser(ctx context.Context, name, email, passwordHash string) (models.User, error) {
	res, err := r.db.ExecContext(ctx, `INSERT INTO users (name, email, password_hash) VALUES (?, ?, ?)`, name, email, passwordHash)
	if err != nil {
		return models.User{}, err
	}
	id, _ := res.LastInsertId()
	return r.GetUserByID(ctx, id)
}

func (r *Repository) GetUserByID(ctx context.Context, id int64) (models.User, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, name, email, password_hash, created_at FROM users WHERE id = ?`, id)
	var u models.User
	if err := row.Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.User{}, ErrNotFound
		}
		return models.User{}, err
	}
	return u, nil
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (models.User, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, name, email, password_hash, created_at FROM users WHERE email = ?`, email)
	var u models.User
	if err := row.Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.User{}, ErrNotFound
		}
		return models.User{}, err
	}
	return u, nil
}

func (r *Repository) CreateTeamWithOwner(ctx context.Context, name string, ownerID int64) (models.Team, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return models.Team{}, err
	}
	defer rollbackUnlessCommitted(tx)

	res, err := tx.ExecContext(ctx, `INSERT INTO teams (name, created_by) VALUES (?, ?)`, name, ownerID)
	if err != nil {
		return models.Team{}, err
	}
	teamID, _ := res.LastInsertId()
	if _, err := tx.ExecContext(ctx, `INSERT INTO team_members (team_id, user_id, role) VALUES (?, ?, 'owner')`, teamID, ownerID); err != nil {
		return models.Team{}, err
	}
	if err := tx.Commit(); err != nil {
		return models.Team{}, err
	}
	return r.GetTeam(ctx, teamID, ownerID)
}

func (r *Repository) GetTeam(ctx context.Context, teamID, userID int64) (models.Team, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT t.id, t.name, t.created_by, tm.role, t.created_at
		FROM teams t
		JOIN team_members tm ON tm.team_id = t.id
		WHERE t.id = ? AND tm.user_id = ?`, teamID, userID)
	var t models.Team
	if err := row.Scan(&t.ID, &t.Name, &t.CreatedBy, &t.Role, &t.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Team{}, ErrNotFound
		}
		return models.Team{}, err
	}
	return t, nil
}

func (r *Repository) ListTeamsForUser(ctx context.Context, userID int64) ([]models.Team, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT t.id, t.name, t.created_by, tm.role, t.created_at
		FROM teams t
		JOIN team_members tm ON tm.team_id = t.id
		WHERE tm.user_id = ?
		ORDER BY t.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var teams []models.Team
	for rows.Next() {
		var t models.Team
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedBy, &t.Role, &t.CreatedAt); err != nil {
			return nil, err
		}
		teams = append(teams, t)
	}
	return teams, rows.Err()
}

func (r *Repository) GetTeamRole(ctx context.Context, teamID, userID int64) (models.Role, error) {
	row := r.db.QueryRowContext(ctx, `SELECT role FROM team_members WHERE team_id = ? AND user_id = ?`, teamID, userID)
	var role models.Role
	if err := row.Scan(&role); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	return role, nil
}

func (r *Repository) AddTeamMember(ctx context.Context, teamID, userID int64, role models.Role) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO team_members (team_id, user_id, role)
		VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE role = VALUES(role)`, teamID, userID, role)
	return err
}

func (r *Repository) CreateTask(ctx context.Context, t models.Task) (models.Task, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return models.Task{}, err
	}
	defer rollbackUnlessCommitted(tx)

	var completedAt interface{}
	if t.Status == models.StatusDone {
		completedAt = time.Now()
	}
	res, err := tx.ExecContext(ctx, `
		INSERT INTO tasks (team_id, title, description, status, assignee_id, created_by, completed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, t.TeamID, t.Title, t.Description, t.Status, t.AssigneeID, t.CreatedBy, completedAt)
	if err != nil {
		return models.Task{}, err
	}
	taskID, _ := res.LastInsertId()
	if _, err := tx.ExecContext(ctx, `INSERT INTO task_history (task_id, changed_by, field_name, old_value, new_value) VALUES (?, ?, 'created', NULL, ?)`, taskID, t.CreatedBy, t.Title); err != nil {
		return models.Task{}, err
	}
	if err := tx.Commit(); err != nil {
		return models.Task{}, err
	}
	return r.GetTaskByID(ctx, taskID)
}

func (r *Repository) GetTaskByID(ctx context.Context, taskID int64) (models.Task, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, team_id, title, description, status, assignee_id, created_by, created_at, updated_at, completed_at FROM tasks WHERE id = ?`, taskID)
	return scanTask(row)
}

func (r *Repository) ListTasks(ctx context.Context, teamID int64, status *models.TaskStatus, assigneeID *int64, limit, offset int) ([]models.Task, error) {
	query := `SELECT id, team_id, title, description, status, assignee_id, created_by, created_at, updated_at, completed_at FROM tasks WHERE team_id = ?`
	args := []interface{}{teamID}
	if status != nil {
		query += ` AND status = ?`
		args = append(args, *status)
	}
	if assigneeID != nil {
		query += ` AND assignee_id = ?`
		args = append(args, *assigneeID)
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		t, err := scanTaskRows(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (r *Repository) UpdateTask(ctx context.Context, taskID, changedBy int64, old models.Task, newTitle *string, newDescription *string, newStatus *models.TaskStatus, newAssigneeID *int64) (models.Task, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return models.Task{}, err
	}
	defer rollbackUnlessCommitted(tx)

	title := old.Title
	if newTitle != nil {
		title = *newTitle
	}
	description := old.Description
	if newDescription != nil {
		description = newDescription
	}
	status := old.Status
	if newStatus != nil {
		status = *newStatus
	}
	assigneeID := old.AssigneeID
	if newAssigneeID != nil {
		assigneeID = newAssigneeID
	}

	var completedAt interface{} = old.CompletedAt
	if old.Status != models.StatusDone && status == models.StatusDone {
		completedAt = time.Now()
	}
	if status != models.StatusDone {
		completedAt = nil
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE tasks
		SET title = ?, description = ?, status = ?, assignee_id = ?, completed_at = ?
		WHERE id = ?`, title, description, status, assigneeID, completedAt, taskID)
	if err != nil {
		return models.Task{}, err
	}

	changes := buildChanges(old, title, description, status, assigneeID)
	for _, ch := range changes {
		if _, err := tx.ExecContext(ctx, `INSERT INTO task_history (task_id, changed_by, field_name, old_value, new_value) VALUES (?, ?, ?, ?, ?)`, taskID, changedBy, ch.field, ch.oldValue, ch.newValue); err != nil {
			return models.Task{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return models.Task{}, err
	}
	return r.GetTaskByID(ctx, taskID)
}

func (r *Repository) ListTaskHistory(ctx context.Context, taskID int64) ([]models.TaskHistory, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, task_id, changed_by, field_name, old_value, new_value, created_at FROM task_history WHERE task_id = ? ORDER BY created_at DESC, id DESC`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []models.TaskHistory
	for rows.Next() {
		var h models.TaskHistory
		var oldValue, newValue sql.NullString
		if err := rows.Scan(&h.ID, &h.TaskID, &h.ChangedBy, &h.FieldName, &oldValue, &newValue, &h.CreatedAt); err != nil {
			return nil, err
		}
		h.OldValue = nullStringPtr(oldValue)
		h.NewValue = nullStringPtr(newValue)
		history = append(history, h)
	}
	return history, rows.Err()
}

func (r *Repository) TeamSummaries(ctx context.Context) ([]models.TeamSummary, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			t.id AS team_id,
			t.name AS team_name,
			COUNT(DISTINCT tm.user_id) AS members_count,
			COUNT(DISTINCT CASE WHEN ts.status = 'done' AND ts.completed_at >= NOW() - INTERVAL 7 DAY THEN ts.id END) AS done_tasks_last_7_days
		FROM teams t
		LEFT JOIN team_members tm ON tm.team_id = t.id
		LEFT JOIN tasks ts ON ts.team_id = t.id
		GROUP BY t.id, t.name
		ORDER BY t.name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.TeamSummary
	for rows.Next() {
		var s models.TeamSummary
		if err := rows.Scan(&s.TeamID, &s.TeamName, &s.MembersCount, &s.DoneTasksLast7Days); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

func (r *Repository) TopCreatorsByTeam(ctx context.Context, month time.Time) ([]models.TopCreator, error) {
	start := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, month.Location())
	end := start.AddDate(0, 1, 0)
	rows, err := r.db.QueryContext(ctx, `
		SELECT team_id, team_name, user_id, user_name, tasks_count, rank_num
		FROM (
			SELECT
				t.id AS team_id,
				t.name AS team_name,
				u.id AS user_id,
				u.name AS user_name,
				COUNT(ts.id) AS tasks_count,
				DENSE_RANK() OVER (PARTITION BY t.id ORDER BY COUNT(ts.id) DESC) AS rank_num
			FROM teams t
			JOIN tasks ts ON ts.team_id = t.id
			JOIN users u ON u.id = ts.created_by
			WHERE ts.created_at >= ? AND ts.created_at < ?
			GROUP BY t.id, t.name, u.id, u.name
		) ranked
		WHERE rank_num <= 3
		ORDER BY team_name ASC, rank_num ASC, tasks_count DESC`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.TopCreator
	for rows.Next() {
		var item models.TopCreator
		if err := rows.Scan(&item.TeamID, &item.TeamName, &item.UserID, &item.UserName, &item.TasksCount, &item.Rank); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Repository) InvalidAssigneeTasks(ctx context.Context) ([]models.InvalidAssigneeTask, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT ts.id, ts.title, t.id, t.name, u.id, u.email
		FROM tasks ts
		JOIN teams t ON t.id = ts.team_id
		JOIN users u ON u.id = ts.assignee_id
		LEFT JOIN team_members tm ON tm.team_id = ts.team_id AND tm.user_id = ts.assignee_id
		WHERE ts.assignee_id IS NOT NULL AND tm.user_id IS NULL
		ORDER BY ts.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.InvalidAssigneeTask
	for rows.Next() {
		var item models.InvalidAssigneeTask
		if err := rows.Scan(&item.TaskID, &item.TaskTitle, &item.TeamID, &item.TeamName, &item.AssigneeID, &item.AssigneeEmail); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Repository) IsTeamMember(ctx context.Context, teamID, userID int64) (bool, error) {
	var exists int
	err := r.db.QueryRowContext(ctx, `SELECT 1 FROM team_members WHERE team_id = ? AND user_id = ? LIMIT 1`, teamID, userID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func scanTask(row interface {
	Scan(dest ...interface{}) error
}) (models.Task, error) {
	var t models.Task
	var description sql.NullString
	var assigneeID sql.NullInt64
	var completedAt sql.NullTime
	if err := row.Scan(&t.ID, &t.TeamID, &t.Title, &description, &t.Status, &assigneeID, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt, &completedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Task{}, ErrNotFound
		}
		return models.Task{}, err
	}
	t.Description = nullStringPtr(description)
	if assigneeID.Valid {
		t.AssigneeID = &assigneeID.Int64
	}
	if completedAt.Valid {
		t.CompletedAt = &completedAt.Time
	}
	return t, nil
}

func scanTaskRows(rows *sql.Rows) (models.Task, error) { return scanTask(rows) }

func nullStringPtr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	return &v.String
}

func rollbackUnlessCommitted(tx *sql.Tx) {
	_ = tx.Rollback()
}

type change struct {
	field    string
	oldValue interface{}
	newValue interface{}
}

func buildChanges(old models.Task, title string, description *string, status models.TaskStatus, assigneeID *int64) []change {
	var changes []change
	if old.Title != title {
		changes = append(changes, change{"title", old.Title, title})
	}
	if strPtrValue(old.Description) != strPtrValue(description) {
		changes = append(changes, change{"description", strPtrValue(old.Description), strPtrValue(description)})
	}
	if old.Status != status {
		changes = append(changes, change{"status", string(old.Status), string(status)})
	}
	if intPtrValue(old.AssigneeID) != intPtrValue(assigneeID) {
		changes = append(changes, change{"assignee_id", intPtrValue(old.AssigneeID), intPtrValue(assigneeID)})
	}
	return changes
}

func strPtrValue(v *string) interface{} {
	if v == nil {
		return nil
	}
	return *v
}

func intPtrValue(v *int64) interface{} {
	if v == nil {
		return nil
	}
	return fmt.Sprintf("%d", *v)
}

func NormalizeStatus(s string) (models.TaskStatus, bool) {
	status := models.TaskStatus(strings.TrimSpace(s))
	switch status {
	case models.StatusTodo, models.StatusInProgress, models.StatusDone:
		return status, true
	default:
		return "", false
	}
}
