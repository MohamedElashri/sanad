package policy

import (
	"testing"
	"time"
)

func TestEvaluateCooldown(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	cooldown := 14 * 24 * time.Hour

	tests := []struct {
		name      string
		candidate time.Time
		wantKind  DecisionKind
		wantAge   time.Duration
	}{
		{
			name:      "older than cooldown is eligible",
			candidate: now.Add(-15 * 24 * time.Hour),
			wantKind:  DecisionUpdate,
			wantAge:   15 * 24 * time.Hour,
		},
		{
			name:      "newer than cooldown is pending",
			candidate: now.Add(-13*24*time.Hour - 23*time.Hour),
			wantKind:  DecisionPending,
			wantAge:   13*24*time.Hour + 23*time.Hour,
		},
		{
			name:      "exact boundary is eligible",
			candidate: now.Add(-cooldown),
			wantKind:  DecisionUpdate,
			wantAge:   cooldown,
		},
		{
			name:      "future timestamp is pending",
			candidate: now.Add(time.Hour),
			wantKind:  DecisionPending,
			wantAge:   -time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateCooldown(now, tt.candidate, cooldown)
			if got.Kind != tt.wantKind {
				t.Fatalf("Kind = %q, want %q (%s)", got.Kind, tt.wantKind, got.Reason)
			}
			if got.Age != tt.wantAge {
				t.Fatalf("Age = %s, want %s", got.Age, tt.wantAge)
			}
		})
	}
}

func TestEvaluateCooldownErrors(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		now       time.Time
		candidate time.Time
		cooldown  time.Duration
	}{
		{name: "missing now", candidate: now},
		{name: "missing candidate", now: now},
		{name: "negative cooldown", now: now, candidate: now, cooldown: -time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateCooldown(tt.now, tt.candidate, tt.cooldown)
			if got.Kind != DecisionError {
				t.Fatalf("Kind = %q, want %q", got.Kind, DecisionError)
			}
			if got.Reason == "" {
				t.Fatal("Reason is empty")
			}
		})
	}
}
