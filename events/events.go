package events

import (
	"github.com/DBCDK/morph/steps"
)

type Event interface{}

type Log struct {
	Data string
}

type RegisterStep struct {
	Step steps.Step
}

type StepUpdate struct {
	StepId string
	State  string
}

// FIXME: Merge this with LogEvent
type StepLog struct {
	StepId string
	Data   string
}

type StepStatus struct {
	Step      steps.Step
	BlockedBy []string
}

type QueueStatus struct {
	Queue []StepStatus
}
