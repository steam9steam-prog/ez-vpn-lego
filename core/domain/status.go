package domain

import "fmt"

type LifecycleStatus string

const (
	LifecycleActive   LifecycleStatus = "active"
	LifecycleDisabled LifecycleStatus = "disabled"
)

func (s LifecycleStatus) Validate() error {
	switch s {
	case LifecycleActive, LifecycleDisabled:
		return nil
	default:
		return fmt.Errorf("invalid lifecycle status %q", s)
	}
}

type OperationStatus string

const (
	OperationQueued      OperationStatus = "queued"
	OperationRunning     OperationStatus = "running"
	OperationSucceeded   OperationStatus = "succeeded"
	OperationFailed      OperationStatus = "failed"
	OperationRollingBack OperationStatus = "rolling_back"
	OperationRolledBack  OperationStatus = "rolled_back"
)

func (s OperationStatus) Terminal() bool {
	return s == OperationSucceeded || s == OperationFailed || s == OperationRolledBack
}

func (s OperationStatus) CanTransitionTo(next OperationStatus) bool {
	allowed := map[OperationStatus]map[OperationStatus]bool{
		OperationQueued: {
			OperationRunning: true,
			OperationFailed:  true,
		},
		OperationRunning: {
			OperationSucceeded:   true,
			OperationFailed:      true,
			OperationRollingBack: true,
		},
		OperationRollingBack: {
			OperationRolledBack: true,
			OperationFailed:     true,
		},
	}

	return allowed[s][next]
}
