package ui

import (
	"github.com/DBCDK/morph/planner"
	"github.com/DBCDK/morph/steps"
)

func CountChildSteps(step steps.Step) int {
	children := 0
	for _, child := range step.Steps {
		children += 1 + CountChildSteps(child)
	}

	return children
}

func CountChildStepsDone(m model, step steps.Step) int {
	childrenDone := 0
	for _, child := range step.Steps {
		if m.stepStatus[child.Id] == planner.Done {
			childrenDone += 1
		}

		childrenDone += CountChildStepsDone(m, child)
	}

	return childrenDone
}
