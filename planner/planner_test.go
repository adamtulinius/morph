package planner

import (
	"github.com/DBCDK/morph/common"
	"github.com/DBCDK/morph/events"
	"github.com/DBCDK/morph/nix"
	"slices"
	"testing"
)

func TestSolverGetNumeralId(t *testing.T) {
	ids0 := make([]string, 0)

	stringId1 := "donkey"
	expectedIds1 := append(make([]string, 0), stringId1)
	ids1, nextId1 := solverGetNumericalId(ids0, stringId1)

	stringId2 := "zebra"
	expectedIds2 := append(expectedIds1, stringId2)
	ids2, nextId2 := solverGetNumericalId(ids1, stringId2)

	if nextId1 != 1 {
		t.Fatalf("Expected next numerical id = 1, got id = %d", nextId1)
	}

	if len(ids1) != 1 {
		t.Fatalf("Expected returned list of ids to have length = 1, got length = %d", len(ids1))
	}

	if !slices.Equal(expectedIds1, ids1) {
		t.Fatalf("Expected returned list of ids to be %v, got %v", expectedIds1, ids1)
	}

	if nextId2 != 2 {
		t.Fatalf("Expected next numerical id = 2, got id = %d", nextId2)
	}

	if len(ids2) != 2 {
		t.Fatalf("Expected returned list of ids to have length = 2, got length = %d", len(ids2))
	}

	if !slices.Equal(expectedIds2, ids2) {
		t.Fatalf("Expected returned list of ids to be %v, got %v", expectedIds2, ids2)
	}

	ids2_, nextId1_ := solverGetNumericalId(ids2, stringId1)

	if nextId1 != nextId1_ {
		t.Fatalf("Expected to get previous numerical id = %d when querying for %s again, but got id = %d", nextId1, stringId1, nextId1_)
	}

	if !slices.Equal(ids2, ids2_) {
		t.Fatalf("Expected to get previous list of ids = %v when querying for %s again, but got ids = %v", ids2, stringId1, ids2_)
	}
}

func TestWeightsOfOnes(t *testing.T) {

	count0 := 0
	result0 := weightsOfOnes(count0)
	expected0 := make([]int, 0)

	count5 := 5
	result5 := weightsOfOnes(count5)
	expected5 := []int{1, 1, 1, 1, 1}

	if !slices.Equal(result0, expected0) {
		t.Fatalf("Expected array to have %d ones, got %v", count0, result0)
	}

	if !slices.Equal(result5, expected5) {
		t.Fatalf("Expected array to have %d ones, got %v", count5, result5)
	}
}

func TestCanStartStepCardinality1(t *testing.T) {
	hosts := make(map[string]nix.Host)
	mctx := common.MorphContext{}

	label1Key := "type"
	label1Value := "web"

	constraints := make([]nix.Constraint, 0)
	constraints = append(constraints, nix.NewConstraint(nix.LabelSelector{Label: label1Key, Value: label1Value}, 1))

	mega := NewMegaContext(events.NewManager(), hosts, &mctx, constraints)

	step1 := EmptyStep()
	step2 := EmptyStep()
	step3 := EmptyStep()

	step1.Id = "host:1"
	step2.Id = "host:2"
	step3.Id = "host:3"

	step1.Labels[label1Key] = label1Value
	step2.Labels[label1Key] = label1Value
	step3.Labels[label1Key] = label1Value

	mega.Steps.Update(step1.Id, step1)
	mega.Steps.Update(step2.Id, step2)
	mega.Steps.Update(step3.Id, step3)

	mega.QueueStep(step1)
	mega.QueueStep(step2)
	mega.QueueStep(step3)

	if !mega.CanStartStep(step1) {
		t.Fatalf("Expected to be able to start step = %s", step1.Id)
	}

	mega.StepStatus.Update(step1.Id, Running)

	if mega.CanStartStep(step2) {
		t.Fatalf("Should not be able to start step = %s with constraints = %v", step2.Id, mega.Constraints)
	}
}

func TestCanStartStepCardinality2(t *testing.T) {
	hosts := make(map[string]nix.Host)
	mctx := common.MorphContext{}

	label1Key := "type"
	label1Value := "web"

	constraints := make([]nix.Constraint, 0)
	constraints = append(constraints, nix.NewConstraint(nix.LabelSelector{Label: label1Key, Value: label1Value}, 2))
	constraints = append(constraints, nix.NewConstraint(nix.LabelSelector{Label: "location", Value: "*"}, 1))
	//constraints = append(constraints, nix.NewConstraint(nix.LabelSelector{Label: "location", Value: "dc1"}, 1))
	//constraints = append(constraints, nix.NewConstraint(nix.LabelSelector{Label: "location", Value: "dc2"}, 1))

	mega := NewMegaContext(events.NewManager(), hosts, &mctx, constraints)

	step1 := EmptyStep()
	step2 := EmptyStep()
	step3 := EmptyStep()
	step4 := EmptyStep()

	step1.Id = "host:1"
	step2.Id = "host:2"
	step3.Id = "host:3"
	step4.Id = "host:4"

	step1.Labels[label1Key] = label1Value
	step2.Labels[label1Key] = label1Value
	step3.Labels[label1Key] = label1Value
	step4.Labels[label1Key] = label1Value

	step1.Labels["location"] = "dc1"
	step2.Labels["location"] = "dc1"
	step3.Labels["location"] = "dc2"
	step4.Labels["location"] = "dc2"

	mega.Steps.Update(step1.Id, step1)
	mega.Steps.Update(step2.Id, step2)
	mega.Steps.Update(step3.Id, step3)
	mega.Steps.Update(step4.Id, step3)

	mega.QueueStep(step1)
	mega.QueueStep(step2)
	mega.QueueStep(step3)
	mega.QueueStep(step4)

	// Emulate starting step1. Should succeed.
	if !mega.CanStartStep(step1) {
		t.Fatalf("Expected to be able to start step = %s", step1.Id)
	}

	// step1 is now started/running
	mega.StepStatus.Update(step1.Id, Running)

	// Emulate starting step2. Should fail since it has same location as step1.
	if mega.CanStartStep(step2) {
		t.Fatalf("Should not be able to start step = %s with constraints = %v", step2.Id, mega.Constraints)
	}

	// Emulate starting step3. Should succeed.
	if !mega.CanStartStep(step3) {
		t.Fatalf("Expected to be able to start step = %s with constraints = %v", step3.Id, mega.Constraints)
	}

	// reset step1
	mega.StepStatus.Update(step1.Id, Queued)

	// Emulate starting step3. Should succeed.
	if !mega.CanStartStep(step3) {
		t.Fatalf("Expected to be able to start step = %s with constraints = %v", step3.Id, mega.Constraints)
	}

	// step2 is now started/running
	mega.StepStatus.Update(step3.Id, Running)

	// Emulate starting step4. Should fail since it has same location as step1.
	if mega.CanStartStep(step4) {
		t.Fatalf("Should not be able to start step = %s with constraints = %v", step4.Id, mega.Constraints)
	}

	// Emulate starting step2. Should succeed.
	if !mega.CanStartStep(step2) {
		t.Fatalf("Expected to be able to start step = %s with constraints = %v", step3.Id, mega.Constraints)
	}

	// step2 is now started/running
	mega.StepStatus.Update(step3.Id, Done)

	// Emulate starting step4. Should succeed.
	if !mega.CanStartStep(step4) {
		t.Fatalf("Expected to be able to start step = %s with constraints = %v", step4.Id, mega.Constraints)
	}

	mega.StepStatus.Update(step4.Id, Done)

	// Emulate starting step1. Should succeed.
	if !mega.CanStartStep(step1) {
		t.Fatalf("Expected to be able to start step = %s with constraints = %v", step1.Id, mega.Constraints)
	}

	mega.StepStatus.Update(step1.Id, Running)

	// Emulate starting step2. Should fail since it has same location as step1.
	if mega.CanStartStep(step2) {
		t.Fatalf("Should not be able to start step = %s with constraints = %v", step2.Id, mega.Constraints)
	}

	mega.StepStatus.Update(step1.Id, Done)

	// Emulate starting step2. Should succeed.
	if !mega.CanStartStep(step2) {
		t.Fatalf("Expected to be able to start step = %s with constraints = %v", step2.Id, mega.Constraints)
	}

	mega.StepStatus.Update(step2.Id, Done)
}
