package planner

import (
	"bufio"
	"fmt"
	"github.com/DBCDK/morph/nix"
	"github.com/DBCDK/morph/steps"
	"github.com/google/uuid"
)

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
	Request steps.Request
	Period  int
	Timeout int
}

func CreateStep(description string, actionName string, action steps.Action, parallel bool, steps_ []steps.Step, onFailure string, dependencies []string) steps.Step {
	step := steps.Step{
		Id:          uuid.New().String(),
		Description: description,
		ActionName:  actionName,
		Action:      action,
		Parallel:    parallel,
		Steps:       steps_,
		OnFailure:   onFailure,
		DependsOn:   dependencies,
		CanResume:   true,
	}

	return step
}

func EmptyStep() steps.Step {
	step := steps.Step{
		Id:          uuid.New().String(),
		Description: "",
		ActionName:  "none",
		Action:      &steps.None{},
		Parallel:    false,
		Steps:       make([]steps.Step, 0),
		OnFailure:   "",
		DependsOn:   make([]string, 0),
		CanResume:   true,
		Labels:      make(map[string]string),
	}

	return step
}

func AddSteps(plan steps.Step, steps ...steps.Step) steps.Step {
	plan.Steps = append(plan.Steps, steps...)

	// for _, step := range steps {
	// 	plan.DependsOn = append(plan.DependsOn, step.Id)
	// 	plan.Steps = append(plan.Steps, step)
	// }

	return plan
}

func AddStepsSeq(plan steps.Step, steps ...steps.Step) steps.Step {
	for _, step := range steps {
		if len(plan.Steps) > 0 {
			// If there's existing steps, get the ID of the last one and add it as dependency to the current one
			step.DependsOn = append(step.DependsOn, plan.Steps[len(plan.Steps)-1].Id)
		}

		plan.Steps = append(plan.Steps, step)
	}

	return plan
}

func EmptySteps() []steps.Step {
	return make([]steps.Step, 0)
}

func MakeDependencies(dependencies ...string) []string {
	deps := make([]string, 0)
	deps = append(deps, dependencies...)

	return deps
}

func CreateBuildPlan(hosts []nix.Host) steps.Step {
	hostNames := make([]string, 0)
	for _, host := range hosts {
		hostNames = append(hostNames, host.Name)
	}

	action := steps.Build{
		Hosts: hostNames,
	}

	buildStep := CreateStep("build hosts", "build", &action, false, EmptySteps(), "exit", make([]string, 0))
	buildStep.Id = "build:" + buildStep.Id
	return buildStep
}

func pushId(host nix.Host) string {
	return "push:" + host.Name
}

func deployId(host nix.Host) string {
	return "deploy:" + host.TargetHost
}

func CreateStepGetSudoPasswd() steps.Step {
	step := EmptyStep()
	step.Description = "Get sudo password"
	step.ActionName = "get-sudo-passwd"
	step.Action = &steps.GetSudoPasswd{}
	step.CanResume = false

	return step
}

func CreateStepSkip(skippedStep steps.Step) steps.Step {
	step := EmptyStep()
	step.Description = skippedStep.ActionName + ": " + skippedStep.Description
	step.ActionName = "skip"

	return step
}

func CreateStepPush(host nix.Host) steps.Step {
	push := steps.Push{
		Host: host.Name,
	}

	step := CreateStep(fmt.Sprintf("push to %s", host.Name), "push", &push, false, EmptySteps(), "exit", make([]string, 0))
	step.Id = pushId(host)

	return step
}

func CreatePushPlan(buildId string, hosts []nix.Host) steps.Step {
	pushParent := CreateStep("push to hosts", "none", &steps.None{}, true, EmptySteps(), "exit", MakeDependencies(buildId))

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

func CreateStepChecks(hint string, host nix.Host, localCommands []CommandPlus, remoteCommands []CommandPlus, localRequests []RequestPlus, remoteRequests []RequestPlus) steps.Step {
	gate := CreateStepGate(hint + " for " + host.Name)
	gate.Id = hint + ":" + host.Name

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

func CreateStepReboot(host nix.Host) steps.Step {
	step := EmptyStep()
	step.Description = "reboot " + host.Name
	step.ActionName = "reboot"
	step.Action = &steps.Reboot{Host: host.Name}

	return step
}

// FIXME: change to remote command
func CreateStepIsOnline(host nix.Host) steps.Step {
	step := EmptyStep()
	step.ActionName = "is-online"
	step.Action = &steps.IsOnline{Host: host.Name}
	step.Description = "test if " + host.Name + " is online"

	return step
}

func CreateStepWaitForOnline(host nix.Host) steps.Step {
	step := CreateStepIsOnline(host)
	step.OnFailure = "retry"
	step.RetryInterval = 2

	return step
}

func CreateStepRebootAndWait(host nix.Host) steps.Step {
	step := EmptyStep()
	step.Description = fmt.Sprintf("reboot '%s' and wait for it to come online", host.Name)

	reboot := CreateStepReboot(host)
	waitForOnline := CreateStepWaitForOnline(host)

	return AddStepsSeq(step, reboot, waitForOnline)
}

func CreateStepGate(description string) steps.Step {
	step := EmptyStep()
	step.ActionName = "gate"
	step.Action = &steps.Gate{}
	step.Description = description
	step.Parallel = true
	step.OnFailure = "retry"

	return step
}

// commands

func createStepCommand(location string, host nix.Host, command Command) steps.Step {
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
	step.Action = &steps.RemoteCommand{
		Command: command.Command,
		Timeout: 0,
	}

	return step
}

func CreateStepLocalCommand(host nix.Host, command Command) steps.Step {
	return createStepCommand("local", host, command)
}

func CreateStepRemoteCommand(host nix.Host, command Command) steps.Step {
	return createStepCommand("remote", host, command)
}

// HTTP requests

// FIXME: Get rid of the health check types
func CreateStepLocalHttpRequest(host nix.Host, req steps.Request) steps.Step {
	step := EmptyStep()

	step.Description = req.Description
	step.Action = &steps.LocalRequest{
		Request: req,
		Timeout: 0,
	}
	step.ActionName = step.Action.Name()

	return step
}

// FIXME: Get rid of the health check types
func CreateStepRemoteHttpRequest(host nix.Host, req steps.Request) steps.Step {
	step := EmptyStep()

	step.Description = req.Description
	step.Action = &steps.RemoteRequest{
		Request: req,
		Timeout: 0,
	}
	step.ActionName = step.Action.Name()

	return step
}

// deploy wrappers

func createStepDeploy(deployAction steps.Action, host nix.Host, dependencies ...steps.Step) steps.Step {
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

func CreateStepDeployBoot(host nix.Host, dependencies ...steps.Step) steps.Step {
	return createStepDeploy(&steps.DeployBoot{Host: host.Name}, host, dependencies...)
}

func CreateStepDeployDryActivate(host nix.Host, dependencies ...steps.Step) steps.Step {
	return createStepDeploy(&steps.DeployDryActivate{Host: host.Name}, host, dependencies...)
}

func CreateStepDeploySwitch(host nix.Host, dependencies ...steps.Step) steps.Step {
	return createStepDeploy(&steps.DeploySwitch{Host: host.Name}, host, dependencies...)
}

func CreateStepDeployTest(host nix.Host, dependencies ...steps.Step) steps.Step {
	return createStepDeploy(&steps.DeployTest{Host: host.Name}, host, dependencies...)
}

// dot file output

func WriteDotFile(writer *bufio.Writer, plan steps.Step) {
	writer.WriteString("digraph G {\n")
	// writer.WriteString("\tlayout=fdp;")
	writer.WriteString("\trankdir=LR;")
	writer.WriteString("\tranksep=0.8;")
	defer writer.WriteString("}\n")

	CreateDotBla(writer, plan)
}

func CreateDotBla(writer *bufio.Writer, step steps.Step) {
	fmt.Println(step.Description)

	switch step.ActionName {
	case "":
		fallthrough
	case "none":
		writer.WriteString(fmt.Sprintf("\t\"%s\"[label = \"<f0> wrapper | <f1> %s\", shape=record, color=grey64, fontcolor=grey64, style=\"rounded,dashed\"]\n", step.Id, step.Description))
	case "skip":
		writer.WriteString(fmt.Sprintf("\t\"%s\"[label = \"<f0> skipped | <f1> %s\", shape=record, color=grey64, style=\"rounded,dashed\"]\n", step.Id, step.Description))
	case "build":
		hostsByName := step.Action.(*steps.Build).Hosts

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
