package repository

import (
	"testing"

	"task-team-api/internal/models"
)

func TestNormalizeStatus(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want models.TaskStatus
		ok   bool
	}{
		{name: "todo", in: "todo", want: models.StatusTodo, ok: true},
		{name: "in progress", in: "in_progress", want: models.StatusInProgress, ok: true},
		{name: "done", in: "done", want: models.StatusDone, ok: true},
		{name: "bad", in: "closed", want: "", ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := NormalizeStatus(tt.in)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("NormalizeStatus(%q) = %q, %v; want %q, %v", tt.in, got, ok, tt.want, tt.ok)
			}
		})
	}
}
