package planner

import (
	"context"
	"fmt"
	"github.com/DBCDK/morph/cache"
	"github.com/DBCDK/morph/events"
	"github.com/DBCDK/morph/logging"
	"github.com/DBCDK/morph/steps"
	"github.com/crillab/gophersat/solver"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"maps"
	"slices"
	"sort"
	"time"
)

const (
	Waiting string = "waiting"
	Queued  string = "queued"
	Blocked string = "blocked"
	Running string = "running"
	Done    string = "done"
	Failed  string = "failed"
)

// FIXME: IDEA: Deployment simulation - make a fake MorphContext where things like SSH-calls are faked and logged instead

func (mega *MegaContext) Run(ctx context.Context) error {
	mega.context = ctx

	// FIXME: This doesn't terminate
	for len(mega.queuedSteps) > 0 || len(mega.StepsNotTerminated()) > 0 {
		// Run everything needed for every iteration
		mega.processQueue()

		time.Sleep(time.Second)

		// Wait for next tick
		// FIXME: make ticking work (and not block)
		//<-mega.tickChan
	}
	return nil
}

func (mega *MegaContext) tick() {
	// FIXME: Ticking disabled
	//mega.tickChan <- true
}

func (mega *MegaContext) StepsNotTerminated() []string {
	notTerminated := make([]string, 0)

	for stepId, status := range mega.StepStatus.GetCopy() {
		if status != Done && status != Failed {
			notTerminated = append(notTerminated, stepId)
		}
	}

	return notTerminated
}

func (mega *MegaContext) processQueue() {

	//remove steps that have been started from the queue!

	mega.queueLock.Lock()
	defer mega.queueLock.Unlock()

	stepsStillQueued := make([]steps.Step, 0)
	stepStatuses := make([]events.StepStatus, 0)

	zLogBefore := zerolog.Arr()
	zLogAfter := zerolog.Arr()
	zLogStarted := zerolog.Arr()

	for _, step := range mega.queuedSteps {
		zLogBefore.Str(step.Id)

		// TODO: log if rejected by dependencies or the solver
		dependenciesSatisfied, blockedBy := mega.DependenciesSatisfied(step.DependsOn)
		//solverSatisfied := mega.CanStartStep(step, mega.Steps.GetCopy(), mega.StepStatus.GetCopy(), mega.Constraints)
		solverSatisfied := mega.CanStartStep(step)

		//if dependenciesSatisfied, blockedBy := mega.DependenciesSatisfied(step.DependsOn); dependenciesSatisfied && mega.CanStartStep(step) {
		if dependenciesSatisfied && solverSatisfied {
			// FIXME: This appears to not be taken into account in the solver:
			mega.UpdateStepStatus(step.Id, Running)
			fmt.Println("!!!!!!!!!!!!!!!!!!1!")
			fmt.Println("!!!!!!!!!!!!!!!!!!1!")
			fmt.Println("!!!!!!!!!!!!!!!!!!1!")
			fmt.Println("!!!!!!!!!!!!!!!!!!1!")

			go func() {
				// FIXME: use errgroup here instead to have a shared group for everything running
				err := mega.ExecuteStep(context.TODO(), step)

				if err != nil {
					switch step.OnFailure {
					case "retry":
						mega.UpdateStepStatus(step.Id, Queued)
						log.Error().Err(err).Msg("Error while running step (retrying)")

						mega.retryCounts.Run(step.Id, 0, func(value int) int {
							return value + 1 // FIXME: Test that this actually works
						})

						mega.QueueStep(step) // FIXME: Handle step status update

					case "ignore":
						mega.UpdateStepStatus(step.Id, Failed)
						log.Error().Err(err).Msg("Error while running step (ignored)")

					default: // propagate error
						mega.UpdateStepStatus(step.Id, Failed)

						// FIXME: stop processing on err
					}
				} else {
					mega.UpdateStepStatus(step.Id, Done)
				}
			}()
			zLogStarted.Str(step.Id)

		} else {
			// TODO: use the list of non-ready in the UI

			stepsStillQueued = append(stepsStillQueued, step)
			zLogAfter.Str(step.Id)

			stepStatuses = append(stepStatuses, events.StepStatus{
				Step:      step,
				BlockedBy: blockedBy,
			})
		}
	}

	mega.queuedSteps = stepsStillQueued

	mega.EventManager.SendEvent(events.QueueStatus{
		Queue: stepStatuses,
	})

	log.Info().
		Str("event", "queue-process-result").
		Array("queue-before", zLogBefore).
		Array("queue-after", zLogAfter).
		Array("started", zLogStarted).
		Msg("finished going through the processing queue")
}

func (mega *MegaContext) UpdateStepStatus(stepId string, status string) {
	log.Info().
		Str("event", "step-status").
		Str("step", stepId).
		Str("status", status).
		Msg("step update")

	mega.StepStatus.Update(stepId, status)

	mega.EventManager.SendEvent(events.StepUpdate{
		StepId: stepId,
		State:  status,
	})

	mega.tick()
}

func (mega *MegaContext) QueueStep(step steps.Step) {
	mega.queueLock.Lock()
	defer mega.queueLock.Unlock()

	// register the step
	mega.Steps.Update(step.Id, step)

	mega.EventManager.SendEvent(events.RegisterStep{Step: step})

	mega.queuedSteps = append(mega.queuedSteps, step)

	mega.UpdateStepStatus(step.Id, Queued)
}

func (mega *MegaContext) QueueSteps(steps ...steps.Step) {
	for _, step := range steps {
		log.Debug().Msg("queueing step: " + step.Id)
		mega.QueueStep(step)
	}
}

func StepMonitor(stepsDb *cache.LockedMap[steps.Step], m *cache.LockedMap[string]) {
	for {
		data := m.GetCopy()

		stepIds := make([]string, 0)
		for stepId, _ := range data {
			stepIds = append(stepIds, stepId)
		}

		sort.Strings(stepIds)

		for _, stepId := range stepIds {
			step, _ := stepsDb.Get(stepId)
			log.Debug().
				Dict("step", zerolog.Dict().
					Str("id", step.Id).
					Str("action", step.ActionName)).
				Msg(fmt.Sprintf("step: " + stepId + " state: " + data[stepId]))
		}

		time.Sleep(time.Second)
	}
}

//func waitForSlot(megaContext *MegaContext, step steps.Step) Slot {
//	// Der er noget med tags og bla bla, steps har fx ikke tags eller labels lige nu, og
//	// dette skal eksplicit ikke være på host-niveau men på step-niveau
//
//	// Idé: maxUnavailable = 1 for alle steps by default, jo mindre steppet er markeret som concurrent (e.g. health checks)
//	// Drop forskellen på parallel true/false i ExecuteStep. Alt er nu parallel by default, men med maxUnavailable = 1 (så samme resultat, men nu muligt at override)
//	// Host tags skal propageres til steps så e.g. location info kan komme med
//
//	slot := newSlot()
//
//	// Multiple constraints can overlap. Right now the first full match is returned
//	// and - in lack of a full match - the first partial match ("*")
//	// TODO: Turn constraints into a tree instead (label -> key -> chan)
//
//	for label, value := range step.Labels {
//		fullMatch := false
//		partialMatch := false
//		var channel chan bool
//
//		for _, constraint := range megaContext.Constraints {
//			if c, err := constraint.GetChan(label, value); err != nil {
//				// no match, ignore
//				continue
//			} else {
//				if constraint.Selector.Value == "*" {
//					fullMatch = true
//					channel = c
//					break
//				} else {
//					// Let first partial match ("*") win
//					if partialMatch == false {
//						partialMatch = true
//						channel = c
//					}
//				}
//			}
//		}
//
//		match := fullMatch || partialMatch
//
//		if match {
//			channel <- true
//			slot.AddChannel(channel)
//		}
//	}
//
//	return slot
//}

// FIXME: return err when dependencies cannot be satisfied, e.g. in case of dependency that failed
func (mega *MegaContext) DependenciesSatisfied(dependencies []string) (bool, []string) {
	dependenciesNotSatisfied := make([]string, 0)

	if len(dependencies) == 0 {
		return true, dependenciesNotSatisfied
	}

	for _, dependency := range dependencies {
		status, err := mega.StepStatus.Get(dependency)
		if err != nil {
			// Not started
			dependenciesNotSatisfied = append(dependenciesNotSatisfied, dependency)
		} else {
			if status != Done {
				dependenciesNotSatisfied = append(dependenciesNotSatisfied, dependency)
			}
		}
	}

	return len(dependenciesNotSatisfied) == 0, dependenciesNotSatisfied
}

// solverGetNumericalId takes a list of known unique strings, and a string to find (or make and ID for).
// Numerical ID's start at 0.
// Returns an updated list of unique strings, and the numerical ID
func solverGetNumericalId(ids []string, id string) ([]string, int) {
	numericalId := slices.Index(ids, id)

	if numericalId >= 0 {
		return ids, numericalId + 1
	}

	return append(ids, id), len(ids) + 1 // Numerical ID's start at 1
}

// For the solver it's
func weightsOfOnes(numberOfOnes int) []int {
	result := make([]int, numberOfOnes)

	for i := range numberOfOnes {
		result[i] = 1
	}

	return result
}

// Make sure to lock the queue before calling this, or results will be inconsistent
func (mega *MegaContext) CanStartStep(step steps.Step) bool {
	//func CanStartStep(step steps.Step, allSteps map[string]steps.Step, stepStatus map[string]string, constraints []nix.Constraint) bool {
	log.Debug().
		Str("component", "solver").Str("stepId", step.Id).
		Msg("CanStartStep running")

	// Early exit - no labels -> no matching constraints -> no limits
	if len(step.Labels) == 0 {
		log.Debug().
			Str("component", "solver").Str("stepId", step.Id).
			Msg("CanStartStep: YES (reason: no labels")
		return true
	}

	pbConstraints := make([]solver.PBConstr, 0)

	//ids := []string{step.Id}
	ids := []string{}
	idStepIdMap := make(map[int]string)
	allSteps := mega.Steps.GetCopy()
	allStepsIds := slices.Sorted(maps.Keys(allSteps))

	zLogStepDict := zerolog.Dict()

	for _, constraint := range mega.Constraints {
		//fmt.Printf("solver: constraint = %v\n", constraint)

		//for _, constraint := range constraints {
		matchingLabels := make(map[string]string)
		matchingIds := make([]int, 0)

		for label, value := range step.Labels {

			if !constraint.Selector.Match(label, value) {
				continue
			}

			matchingLabels[label] = value

			// For every label with a constraint, find steps with the same label, and map each step to a numerical ID

			//for otherStepId, otherStep := range allSteps {
			for _, otherStepId := range allStepsIds {
				otherStep := allSteps[otherStepId]

				// if other step has the label with the same value
				if otherValue, ok := otherStep.Labels[label]; ok && value == otherValue {
					newIds, id := solverGetNumericalId(ids, otherStepId)
					ids = newIds
					idStepIdMap[id] = otherStepId

					if !slices.Contains(matchingIds, id) {
						matchingIds = append(matchingIds, id)
					}

					//fmt.Printf("solver: loop other steps\n")
					//fmt.Printf("solver: new id:       %d\n", id)
					//fmt.Printf("solver: all ids:      %v\n", ids)
					//fmt.Printf("solver: new ids:      %v\n", newIds)
					//fmt.Printf("solver: matching ids: %v\n", matchingIds)
				}
			}

			statuses := make([]int, 0)

			// TODO: Find better names since this might not related to hosts at all
			allHosts := make([]int, 0)
			hostsUp := make([]int, 0)
			hostsDown := make([]int, 0)

			for _, i := range matchingIds {
				//fmt.Printf("matching id = %d\n", i)
				id := ids[i-1]
				status, err := mega.StepStatus.Get(id)
				//status, ok := stepStatus[id]
				if err != nil {
					// TODO: Log fatal here
					return false
				}

				allHosts = append(allHosts, i)
				zLogStepDict.Str(fmt.Sprintf("%d", i), id)

				// if self - always negative, since we're simulating this step progressing
				if id == step.Id {
					statuses = append(statuses, -i)
					hostsDown = append(hostsDown, i)
					continue
				}

				// FIXME: This should really be a measure of steps that are "unhealthy" as well
				if status == Running {
					statuses = append(statuses, -i)
					hostsDown = append(hostsDown, i)
				} else {
					statuses = append(statuses, i)
					hostsUp = append(hostsUp, i)
				}
			}

			log.Debug().
				Str("component", "solver").Str("stepId", step.Id).
				Dict("label", zerolog.Dict().Str("key", label).Str("value", value)).
				//Dict("steps", zLogStepDict).
				Dict("steps", logging.MapToZLogDict(idStepIdMap)).
				Array("statuses", logging.ArrayToZLogArray(statuses)).
				Array("allHosts", logging.ArrayToZLogArray(allHosts)).
				Array("hostsUp", logging.ArrayToZLogArray(hostsUp)).
				Array("hostsDown", logging.ArrayToZLogArray(hostsDown)).
				Msg("steps split")

			pbConstraints = append(pbConstraints, solver.AtLeast(allHosts, len(matchingIds)-constraint.MaxUnavailable))

			//[]solver.PBConstr{
			//	// hosts up (reverse of maxUnavailable for hosts
			//
			//	// Locations 1, 2, 3
			//	//solver.AtLeast([]int{1, 2}, 1),
			//	//solver.AtLeast([]int{3, 4}, 1),
			//	//solver.AtLeast([]int{5, 6}, 1),
			//}

			// hosts offline => sum to 0
			pbConstraints = append(pbConstraints, solver.Eq(hostsDown, weightsOfOnes(len(hostsDown)), 0)...)

			// hosts online => sum to len(hosts online)
			pbConstraints = append(pbConstraints, solver.Eq(hostsUp, weightsOfOnes(len(hostsUp)), len(hostsUp))...)

			// can offline host and online hosts be combined using weights?
			//pbConstraints = append(pbConstraints, solver.Eq([]int{1, 2, 3, 4, 6}, []int{-1, 1, -1, 1, 1}, 3)...)

		}
	}

	slices.Sort(ids) // Not necessary but makes debugging easier

	log.Debug().
		Str("component", "solver").Str("stepId", step.Id).
		Array("steps-in-solve", logging.ArrayToZLogArray(ids)).
		Msgf("Steps involved in the solve: %v", ids)

	pb := solver.ParsePBConstrs(pbConstraints)

	log.Debug().
		Str("component", "solver").Str("stepId", step.Id).
		Dict("steps", logging.MapToZLogDict(idStepIdMap)).
		Str("problem", pb.PBString()).
		Msg("problem string: " + pb.PBString())

	s := solver.New(pb)
	status := s.Solve()

	x := log.Debug().
		Str("component", "solver").Str("stepId", step.Id).
		Dict("steps", logging.MapToZLogDict(idStepIdMap)).
		Str("problem", pb.PBString()).
		Str("status", status.String())

	if status == solver.Sat {
		// get model as bools but only when solve was satisfiable
		//fmt.Println(s.Model())
		//s.OutputModel()
		x.
			Array("model", logging.ArrayToZLogArray(s.Model())).
			Msg("solve successful")
	} else {
		x.Msgf("solve failed: %s", status)
	}

	return status == solver.Sat
}

func (mega *MegaContext) waitForChildrenToComplete(ctx context.Context, step steps.Step) {
	childIds := make([]string, 0)
	for _, subStep := range step.Steps {
		childIds = append(childIds, subStep.Id)
	}

	for {
		if satisfied, _ := mega.DependenciesSatisfied(childIds); satisfied {
			return
		} else {
			time.Sleep(time.Second)
		}
	}
}

func (mega *MegaContext) ExecuteStep(ctx context.Context, step steps.Step) error {
	err := step.Action.Run(ctx, mega.MorphContext, mega.Hosts, mega.Cache)
	if err != nil {
		return err
	}

	if step.Parallel {
		// queue all steps
		mega.QueueSteps(step.Steps...)
	} else {
		// queue all steps but first make them depend on each other in order
		// (first sub step will depend on nothing extra)
		previousStepId := ""
		first := true
		for _, subStep := range step.Steps {
			if first {
				previousStepId = subStep.Id
				first = false
			} else {
				subStep.DependsOn = append(subStep.DependsOn, previousStepId)
			}

			previousStepId = subStep.Id

			mega.QueueStep(subStep)
		}
	}

	mega.waitForChildrenToComplete(ctx, step)

	return nil
}
