package email

import (
	"context"
	"fmt"
	"time"

	"github.com/sony/gobreaker"
)

type Client interface {
	SendInvite(ctx context.Context, toEmail string, teamName string) error
}

type MockClient struct {
	latency time.Duration
	breaker *gobreaker.CircuitBreaker
}

func NewMockClient(latency time.Duration) *MockClient {
	settings := gobreaker.Settings{
		Name:        "mock-email-service",
		MaxRequests: 3,
		Interval:    time.Minute,
		Timeout:     10 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 3
		},
	}
	return &MockClient{latency: latency, breaker: gobreaker.NewCircuitBreaker(settings)}
}

func (c *MockClient) SendInvite(ctx context.Context, toEmail string, teamName string) error {
	_, err := c.breaker.Execute(func() (interface{}, error) {
		select {
		case <-time.After(c.latency):
			return nil, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})
	if err != nil {
		return fmt.Errorf("email invite failed: %w", err)
	}
	return nil
}
