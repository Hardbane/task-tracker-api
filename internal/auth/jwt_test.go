package auth

import (
	"testing"
	"time"
)

func TestManagerGenerateAndParse(t *testing.T) {
	manager := NewManager("test-secret", time.Hour)
	token, err := manager.Generate(42)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	userID, err := manager.Parse(token)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if userID != 42 {
		t.Fatalf("expected user id 42, got %d", userID)
	}
}

func TestManagerRejectsInvalidToken(t *testing.T) {
	manager := NewManager("test-secret", time.Hour)
	if _, err := manager.Parse("bad-token"); err == nil {
		t.Fatal("expected invalid token error")
	}
}
