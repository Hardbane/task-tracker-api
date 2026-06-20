package models

import "time"

type Role string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
)

type TaskStatus string

const (
	StatusTodo       TaskStatus = "todo"
	StatusInProgress TaskStatus = "in_progress"
	StatusDone       TaskStatus = "done"
)

type User struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

type Team struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedBy int64     `json:"created_by"`
	Role      Role      `json:"role,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type Task struct {
	ID          int64      `json:"id"`
	TeamID      int64      `json:"team_id"`
	Title       string     `json:"title"`
	Description *string    `json:"description,omitempty"`
	Status      TaskStatus `json:"status"`
	AssigneeID  *int64     `json:"assignee_id,omitempty"`
	CreatedBy   int64      `json:"created_by"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

type TaskHistory struct {
	ID        int64     `json:"id"`
	TaskID    int64     `json:"task_id"`
	ChangedBy int64     `json:"changed_by"`
	FieldName string    `json:"field_name"`
	OldValue  *string   `json:"old_value,omitempty"`
	NewValue  *string   `json:"new_value,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type TeamSummary struct {
	TeamID             int64  `json:"team_id"`
	TeamName           string `json:"team_name"`
	MembersCount       int64  `json:"members_count"`
	DoneTasksLast7Days int64  `json:"done_tasks_last_7_days"`
}

type TopCreator struct {
	TeamID     int64  `json:"team_id"`
	TeamName   string `json:"team_name"`
	UserID     int64  `json:"user_id"`
	UserName   string `json:"user_name"`
	TasksCount int64  `json:"tasks_count"`
	Rank       int64  `json:"rank"`
}

type InvalidAssigneeTask struct {
	TaskID        int64  `json:"task_id"`
	TaskTitle     string `json:"task_title"`
	TeamID        int64  `json:"team_id"`
	TeamName      string `json:"team_name"`
	AssigneeID    int64  `json:"assignee_id"`
	AssigneeEmail string `json:"assignee_email"`
}
