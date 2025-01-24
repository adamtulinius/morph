package planner

import (
	"bufio"
	"fmt"
	"github.com/DBCDK/morph/actions"
	"github.com/DBCDK/morph/nix"
	"github.com/google/uuid"
)

type Step struct {
	Id            string            `json:"id"`
	Description   string            `json:"description"`
	ActionName    string            `json:"action"`
	Action        actions.Action    `json:"-"`
	Parallel      bool              `json:"parallel"`
	OnFailure     string            `json:"on-failure"` // retry, exit, ignore
	Timeout       int               `json:"timeout"`
	RetryInterval int               `json:"retry-interval"` // interval between retries in case of OnFailure = "retry"
	Steps         []Step            `json:"steps"`
	DependsOn     []string          `json:"dependencies"`
	CanResume     bool              `json:"can-resume"`
	Labels        map[string]string `json:"labels,omitempty"`
}

type XSchedule struct {
	Period  int
	Timeout int
}

type StepStatus struct {
	Id     string
	Status string
}

type Command struct {
	Description string
	Command     []string
}

type CommandPlus struct {
	Command Command
	Period  int
	Timeout int
}

type RequestPlus struct {
	Request actions.Request
	Period  int
	Timeout int
}

func CreateStep(description string, actionName string, action actions.Action, parallel bool, steps []Step, onFailure string, dependencies []string) Step {
	step := Step{
		Id:          uuid.New().String(),
		Description: description,
		ActionName:  actionName,
		Action:      action,
		Parallel:    parallel,
		Steps:       steps,
		OnFailure:   onFailure,
		DependsOn:   dependencies,
		CanResume:   true,
	}

	return step
}

func EmptyStep() Step {
	step := Step{
		Id:          uuid.New().String(),
		Description: "",
		ActionName:  "none",
		Action:      actions.None{},
		Parallel:    false,
		Steps:       make([]Step, 0),
		OnFailure:   "",
		DependsOn:   make([]string, 0),
		CanResume:   true,
	}

	return step
}

func AddSteps(plan Step, steps ...Step) Step {
	plan.Steps = append(plan.Steps, steps...)

	// for _, step := range steps {
	// 	plan.DependsOn = append(plan.DependsOn, step.Id)
	// 	plan.Steps = append(plan.Steps, step)
	// }

	return plan
}

func AddStepsSeq(plan Step, steps ...Step) Step {
	for _, step := range steps {
		if len(plan.Steps) > 0 {
			// If tthere's existing steps, get the ID of the last one and add it as dependency to the current one
			step.DependsOn = append(step.DependsOn, plan.Steps[len(plan.Steps)-1].Id)
		}

		plan.Steps = append(plan.Steps, step)
	}

	return plan
}

func EmptySteps() []Step {
	return make([]Step, 0)
}

func MakeDependencies(dependencies ...string) []string {
	deps := make([]string, 0)
	deps = append(deps, dependencies...)

	return deps
}

func CreateBuildPlan(hosts []nix.Host) Step {
	hostNames := make([]string, 0)
	for _, host := range hosts {
		hostNames = append(hostNames, host.Name)
	}

	action := actions.Build{
		Hosts: hostNames,
	}

	return CreateStep("build hosts", "build", action, false, EmptySteps(), "exit", make([]string, 0))
}

func pushId(host nix.Host) string {
	return "push:" + host.TargetHost
}

func deployId(host nix.Host) string {
	return "deploy:" + host.TargetHost
}

func CreateStepGetSudoPasswd() Step {
	step := EmptyStep()
	step.Description = "Get sudo password"
	step.ActionName = "get-sudo-passwd"
	step.Action = actions.GetSudoPasswd{}
	step.CanResume = false

	return step
}

func CreateStepSkip(skippedStep Step) Step {
	step := EmptyStep()
	step.Description = skippedStep.ActionName + ": " + skippedStep.Description
	step.ActionName = "skip"

	return step
}

func CreateStepPush(host nix.Host) Step {
	push := actions.Push{
		Host: host.Name,
	}

	step := CreateStep(fmt.Sprintf("push to %s", host.Name), "push", push, false, EmptySteps(), "exit", make([]string, 0))
	step.Id = pushId(host)

	return step
}

func CreatePushPlan(buildId string, hosts []nix.Host) Step {
	pushParent := CreateStep("push to hosts", "none", actions.None{}, true, EmptySteps(), "exit", MakeDependencies(buildId))

	for _, host := range hosts {
		pushParent = AddSteps(
			pushParent,
			CreateStepPush(host),
		)
	}

	return pushParent
}

// func CreateStepHealthChecks(host nix.Host, checks healthchecks.HealthChecks) Step {
// 	gate := CreateStepGate("healthchecks for " + host.Name)

// 	for _, check := range checks.Cmd {
// 		req := CreateStepRemoteCommand(host, check)
// 		runner := CreateStepRepeatUntilSuccess(check.Period, check.Timeout)
// 		runner = AddSteps(runner, req)

// 		gate.Steps = append(gate.Steps, runner)
// 	}

// 	for _, check := range checks.Http {
// 		req := CreateStepRemoteHttpRequest(host, check)
// 		runner := CreateStepRepeatUntilSuccess(check.Period, check.Timeout)
// 		runner = AddSteps(runner, req)

// 		gate.Steps = append(gate.Steps, runner)
// 	}

// 	return gate
// }

func CreateStepChecks(host nix.Host, localCommands []CommandPlus, remoteCommands []CommandPlus, localRequests []RequestPlus, remoteRequests []RequestPlus) Step {
	gate := CreateStepGate("healthchecks for " + host.Name)

	for _, commandPlus := range localCommands {
		step := CreateStepLocalCommand(host, commandPlus.Command)
		step.Timeout = commandPlus.Timeout
		step.RetryInterval = commandPlus.Period

		gate.Steps = append(gate.Steps, step)
	}

	for _, commandPlus := range remoteCommands {
		step := CreateStepRemoteCommand(host, commandPlus.Command)
		step.Timeout = commandPlus.Timeout
		step.RetryInterval = commandPlus.Period

		gate.Steps = append(gate.Steps, step)
	}

	for _, reqPlus := range localRequests {
		step := CreateStepLocalHttpRequest(host, reqPlus.Request)
		step.Timeout = reqPlus.Timeout
		step.RetryInterval = reqPlus.Period

		gate.Steps = append(gate.Steps, step)
	}

	for _, reqPlus := range remoteRequests {
		step := CreateStepRemoteHttpRequest(host, reqPlus.Request)
		step.Timeout = reqPlus.Timeout
		step.RetryInterval = reqPlus.Period

		gate.Steps = append(gate.Steps, step)

	}

	return gate
}

func CreateStepReboot(host nix.Host) Step {
	step := EmptyStep()
	step.Description = "reboot " + host.Name
	step.ActionName = "reboot"
	step.Action = actions.Reboot{Host: host.Name}

	return step
}

// FIXME: change to remote command
func CreateStepIsOnline(host nix.Host) Step {
	step := EmptyStep()
	step.ActionName = "is-online"
	step.Action = actions.IsOnline{Host: host.Name}
	step.Description = "test if " + host.Name + " is online"

	return step
}

func CreateStepWaitForOnline(host nix.Host) Step {
	step := CreateStepIsOnline(host)
	step.OnFailure = "retry"
	step.RetryInterval = 2

	return step
}

func CreateStepRebootAndWait(host nix.Host) Step {
	step := EmptyStep()
	step.Description = fmt.Sprintf("reboot '%s' and wait for it to come online", host.Name)

	reboot := CreateStepReboot(host)
	waitForOnline := CreateStepWaitForOnline(host)

	return AddStepsSeq(step, reboot, waitForOnline)
}

func CreateStepGate(description string) Step {
	step := EmptyStep()
	step.ActionName = "gate"
	step.Action = actions.Gate{}
	step.Description = description
	step.Parallel = true
	step.OnFailure = "retry"

	return step
}

// commands

func createStepCommand(location string, host nix.Host, command Command) Step {
	step := EmptyStep()

	switch location {
	case "local":
		step.ActionName = "local-command"
	case "remote":
		step.ActionName = "remote-command"
	default:
		panic("Unknown location type")
	}

	step.Description = command.Description
	step.Action = actions.RemoteCommand{
		Command: command.Command,
		Timeout: 0,
	}

	return step
}

func CreateStepLocalCommand(host nix.Host, command Command) Step {
	return createStepCommand("local", host, command)
}

func CreateStepRemoteCommand(host nix.Host, command Command) Step {
	return createStepCommand("remote", host, command)
}

// HTTP requests

// FIXME: Get rid of the health check types
func CreateStepLocalHttpRequest(host nix.Host, req actions.Request) Step {
	step := EmptyStep()

	step.Description = req.Description
	step.Action = actions.LocalRequest{
		Request: req,
		Timeout: 0,
	}
	step.ActionName = step.Action.Name()

	return step
}

// FIXME: Get rid of the health check types
func CreateStepRemoteHttpRequest(host nix.Host, req actions.Request) Step {
	step := EmptyStep()

	step.Description = req.Description
	step.Action = actions.RemoteRequest{
		Request: req,
		Timeout: 0,
	}
	step.ActionName = step.Action.Name()

	return step
}

// deploy wrappers

func createStepDeploy(deployAction actions.Action, host nix.Host, dependencies ...Step) Step {
	step := EmptyStep()
	step.Id = deployId(host)
	step.Description = "deploy " + host.Name
	step.ActionName = deployAction.Name()
	step.Action = deployAction
	step.OnFailure = ""

	for _, dependency := range dependencies {
		step.DependsOn = append(step.DependsOn, dependency.Id)
	}

	return step
}

func CreateStepDeployBoot(host nix.Host, dependencies ...Step) Step {
	return createStepDeploy(actions.DeployBoot{Host: host.Name}, host, dependencies...)
}

func CreateStepDeployDryActivate(host nix.Host, dependencies ...Step) Step {
	return createStepDeploy(actions.DeployDryActivate{Host: host.Name}, host, dependencies...)
}

func CreateStepDeploySwitch(host nix.Host, dependencies ...Step) Step {
	return createStepDeploy(actions.DeploySwitch{Host: host.Name}, host, dependencies...)
}

func CreateStepDeployTest(host nix.Host, dependencies ...Step) Step {
	return createStepDeploy(actions.DeployTest{Host: host.Name}, host, dependencies...)
}

// dot file output

func WriteDotFile(writer *bufio.Writer, plan Step) {
	writer.WriteString("digraph G {\n")
	// writer.WriteString("\tlayout=fdp;")
	writer.WriteString("\trankdir=LR;")
	writer.WriteString("\tranksep=0.8;")
	defer writer.WriteString("}\n")

	CreateDotBla(writer, plan)
}

func CreateDotBla(writer *bufio.Writer, step Step) {
	fmt.Println(step.Description)

	switch step.ActionName {
	case "":
		fallthrough
	case "none":
		writer.WriteString(fmt.Sprintf("\t\"%s\"[label = \"<f0> wrapper | <f1> %s\", shape=record, color=grey64, fontcolor=grey64, style=\"rounded,dashed\"]\n", step.Id, step.Description))
	case "skip":
		writer.WriteString(fmt.Sprintf("\t\"%s\"[label = \"<f0> skipped | <f1> %s\", shape=record, color=grey64, style=\"rounded,dashed\"]\n", step.Id, step.Description))
	case "build":
		hostsByName := step.Action.(actions.Build).Hosts

		writer.WriteString(fmt.Sprintf("\t\"%s\"[label = \"<f0> build | <f1> %s", step.Id, step.Description))
		for i, host := range hostsByName {
			writer.WriteString(fmt.Sprintf(" | <f%d> %s", i+2, host))
		}
		writer.WriteString("\", shape=record, style=rounded]\n")
	default:
		writer.WriteString(fmt.Sprintf("\t\"%s\"[label = \"<f0> %s | <f1> %s\", shape=record, style=rounded]\n", step.Id, step.ActionName, step.Description))
	}

	for _, dependency := range step.DependsOn {
		writer.WriteString(fmt.Sprintf("\t\"%s\" -> \"%s\" [dir=back, color=deepskyblue, penwidth=1.0, style=dashed];\n", dependency, step.Id))
	}

	for _, subStep := range step.Steps {
		// writer.WriteString(fmt.Sprintf("\t\"%s\" -> \"%s\" [style=dotted];\n", step.Id, subStep.Id))
		writer.WriteString(fmt.Sprintf("\t\"%s\" -> \"%s\" [color=grey64, penwidth=0.8];\n", step.Id, subStep.Id))
		CreateDotBla(writer, subStep)
	}
}
