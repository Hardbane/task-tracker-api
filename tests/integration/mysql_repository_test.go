package integration

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
	"github.com/testcontainers/testcontainers-go/wait"

	"task-team-api/internal/repository"
)

func TestRepositoryReportsWithMySQL(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION") != "1" {
		t.Skip("set RUN_INTEGRATION=1 to run Docker-based integration tests")
	}
	ctx := context.Background()

	container, err := mysql.Run(ctx,
		"mysql:8.4",
		mysql.WithDatabase("task_db"),
		mysql.WithUsername("task_user"),
		mysql.WithPassword("task_password"),
		testcontainers.WithWaitStrategy(wait.ForLog("port: 3306  MySQL Community Server").WithStartupTimeout(2*time.Minute)),
	)
	if err != nil {
		t.Fatalf("start mysql container: %v", err)
	}
	defer func() { _ = testcontainers.TerminateContainer(container) }()

	dsn, err := container.ConnectionString(ctx, "parseTime=true", "multiStatements=true")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	migration, err := os.ReadFile(filepath.Join("..", "..", "migrations", "001_init.sql"))
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if _, err := db.ExecContext(ctx, string(migration)); err != nil {
		t.Fatalf("apply migration: %v", err)
	}

	seed := `
		INSERT INTO users (id, name, email, password_hash) VALUES
		(1, 'Owner', 'owner@example.com', 'x'), (2, 'Dev', 'dev@example.com', 'x');
		INSERT INTO teams (id, name, created_by) VALUES (1, 'Core', 1);
		INSERT INTO team_members (team_id, user_id, role) VALUES (1, 1, 'owner'), (1, 2, 'member');
		INSERT INTO tasks (id, team_id, title, status, assignee_id, created_by, completed_at, created_at) VALUES
		(1, 1, 'Done task', 'done', 2, 1, NOW(), NOW()),
		(2, 1, 'Todo task', 'todo', 2, 2, NULL, NOW());`
	if _, err := db.ExecContext(ctx, seed); err != nil {
		t.Fatalf("seed data: %v", err)
	}

	repo := repository.New(db)
	summaries, err := repo.TeamSummaries(ctx)
	if err != nil {
		t.Fatalf("TeamSummaries() error = %v", err)
	}
	if len(summaries) != 1 || summaries[0].MembersCount != 2 || summaries[0].DoneTasksLast7Days != 1 {
		t.Fatalf("unexpected summaries: %+v", summaries)
	}
}
