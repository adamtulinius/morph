package steps

import (
	"encoding/json"
	"errors"
	"fmt"
)

type Step struct {
	Id            string            `json:"id"`
	Description   string            `json:"description"`
	ActionName    string            `json:"action"`
	Action        Action            `json:"-"`
	Parallel      bool              `json:"parallel"`
	OnFailure     string            `json:"on-failure"` // retry, exit, ignore
	Timeout       int               `json:"timeout"`
	RetryInterval int               `json:"retry-interval"` // interval between retries in case of OnFailure = "retry"
	Steps         []Step            `json:"steps"`
	DependsOn     []string          `json:"dependencies"`
	CanResume     bool              `json:"can-resume"`
	Labels        map[string]string `json:"labels,omitempty"`
}

type StepAlias Step

// The following MarshalJSON contains a lot of code that looks
// like it could be easily compacted. Ideas welcome.
// Type parameters can't be embedded, otherwise something like
// this would help cut a lot of code:
//
// func x[A actions.Action](step Step) ([]byte, error) {
//   type StepAlias Step
//   return json.Marshal(struct {
//     StepAlias
//     A // I'm sorry Dave, I'm afraid I can't do that
//   }{
//     StepAlias: StepAlias(step),
//     A:         step.Action.(A),
//   })
// }

func (step Step) MarshalJSON() ([]byte, error) {
	switch step.ActionName {
	case None{}.Name():
		fallthrough
	case Gate{}.Name():
		fallthrough
	case Wrapper{}.Name():
		return json.Marshal(StepAlias(step))

	case Build{}.Name():
		return step.Action.MarshalJSONx(step)

	case Push{}.Name():
		return step.Action.MarshalJSONx(step)

	case DeployBoot{}.Name():
		return step.Action.MarshalJSONx(step)

	case DeployDryActivate{}.Name():
		return step.Action.MarshalJSONx(step)

	case DeploySwitch{}.Name():
		return step.Action.MarshalJSONx(step)

	case DeployTest{}.Name():
		return step.Action.MarshalJSONx(step)

	case IsOnline{}.Name():
		return step.Action.MarshalJSONx(step)

	case Reboot{}.Name():
		return step.Action.MarshalJSONx(step)

	case LocalCommand{}.Name():
		return step.Action.MarshalJSONx(step)

	case RemoteCommand{}.Name():
		return step.Action.MarshalJSONx(step)

	case LocalRequest{}.Name():
		return step.Action.MarshalJSONx(step)

	case RemoteRequest{}.Name():
		return step.Action.MarshalJSONx(step)

	default:
		return nil, errors.New("unmarshall: unknown action: " + step.ActionName)
	}
}

func (step *Step) UnmarshalJSON(b []byte) error {
	// A step is unmarshalled twice:
	// 1) into an alias for the Step struct
	// 2) into the type matching the action name of the step
	// (2) is then added as the action to the step

	// This alias keeps the original methods implemented
	// on Step, in this case the default UnmarshalJSON method.

	type StepAlias Step

	// (1) Unmarshal everything in to the step alias, and assign it to *step
	// *step will then be populated according to the Step-struct but without the Action

	// Safe defaults for Step
	step_ := StepAlias{
		ActionName: "none",
		Parallel:   false,
		CanResume:  false,
		DependsOn:  make([]string, 0),
		Steps:      make([]Step, 0),
	}

	err := json.Unmarshal(b, &step_)
	if err != nil {
		return err
	}
	*step = Step(step_)

	//_ = map[string]func() interface{}{
	//	"build": func() interface{} { return &Build{} },
	//	"push":  func() interface{} { return &Push{} },
	//}

	// (2) Unmarshal the same into the corresponding Action and
	// add it to the step
	switch step.ActionName {
	case None{}.Name():
		fallthrough
	case Gate{}.Name():
		fallthrough
	case Wrapper{}.Name():
		// do nothing
		fmt.Println("action: none")

	case Build{}.Name():
		fmt.Println("action: build")
		var build Build
		err = build.UnmarshalJSON(b)
		if err != nil {
			return err
		}

		step.Action = &build

	case Push{}.Name():
		fmt.Println("action: push")
		var push Push
		err = push.UnmarshalJSON(b)
		if err != nil {
			return err
		}

		step.Action = &push

	default:
		return errors.New("unmarshal: unknown action: " + step.ActionName)
	}

	fmt.Println("action: " + step.Action.Name())
	var push Push
	err = push.UnmarshalJSON(b)
	if err != nil {
		return err
	}

	return nil
}
