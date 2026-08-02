package domain

import "testing"

func TestOperationTransitions(t *testing.T) {
	tests := []struct {
		from OperationStatus
		to   OperationStatus
		want bool
	}{
		{OperationQueued, OperationRunning, true},
		{OperationQueued, OperationFailed, true},
		{OperationRunning, OperationSucceeded, true},
		{OperationRunning, OperationRollingBack, true},
		{OperationRollingBack, OperationRolledBack, true},
		{OperationSucceeded, OperationRunning, false},
		{OperationRolledBack, OperationRunning, false},
		{OperationQueued, OperationSucceeded, false},
	}

	for _, test := range tests {
		if got := test.from.CanTransitionTo(test.to); got != test.want {
			t.Errorf("transition %s -> %s: got %v, want %v", test.from, test.to, got, test.want)
		}
	}
}

func TestTerminalOperationStates(t *testing.T) {
	for _, status := range []OperationStatus{OperationSucceeded, OperationFailed, OperationRolledBack} {
		if !status.Terminal() {
			t.Errorf("expected %s to be terminal", status)
		}
	}

	for _, status := range []OperationStatus{OperationQueued, OperationRunning, OperationRollingBack} {
		if status.Terminal() {
			t.Errorf("expected %s not to be terminal", status)
		}
	}
}
