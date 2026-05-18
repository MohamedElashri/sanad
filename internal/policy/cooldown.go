package policy

import (
	"fmt"
	"time"
)

func CandidateAge(now time.Time, candidateTimestamp time.Time) time.Duration {
	if now.IsZero() || candidateTimestamp.IsZero() {
		return 0
	}
	return now.Sub(candidateTimestamp)
}

func EvaluateCooldown(now time.Time, candidateTimestamp time.Time, cooldown time.Duration) Decision {
	age := CandidateAge(now, candidateTimestamp)
	if cooldown < 0 {
		return Decision{
			Kind:   DecisionError,
			Reason: "cooldown must not be negative",
			Age:    age,
		}
	}
	if candidateTimestamp.IsZero() {
		return Decision{
			Kind:   DecisionError,
			Reason: "candidate timestamp is required for cooldown evaluation",
			Age:    age,
		}
	}
	if now.IsZero() {
		return Decision{
			Kind:   DecisionError,
			Reason: "current time is required for cooldown evaluation",
			Age:    age,
		}
	}
	if age < 0 {
		return Decision{
			Kind:   DecisionPending,
			Reason: "candidate timestamp is in the future",
			Age:    age,
		}
	}
	if age >= cooldown {
		return Decision{
			Kind:   DecisionUpdate,
			Reason: fmt.Sprintf("candidate age %s satisfies cooldown %s", age, cooldown),
			Age:    age,
		}
	}
	return Decision{
		Kind:   DecisionPending,
		Reason: fmt.Sprintf("candidate age %s is below cooldown %s", age, cooldown),
		Age:    age,
	}
}
