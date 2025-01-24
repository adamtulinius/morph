package main

// TODO: Morph NixOS integration tests
// TODO: turn all `panic`'s into proper error handling
// TODO: remove --passwd since morph can then ignore stdin and doesn't have to figure out how to share it between steps
// TODO: 12:14AM ERR error marshalling plan to JSON error="json: error calling MarshalJSON for type planner.Step: json: error calling MarshalJSON for type planner.Step: json: error calling MarshalJSON for type planner.Step: unmarshall: unknown action: wait-for-online"
//     ^ drop wait-for-online and let it be a repeating gate/step kinda thing that calls IsOnline
import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/DBCDK/kingpin"
	"github.com/DBCDK/morph/actions"
	"github.com/DBCDK/morph/cache"
	"github.com/DBCDK/morph/common"
	"github.com/DBCDK/morph/cruft"
	"github.com/DBCDK/morph/nix"
	"github.com/DBCDK/morph/planner"
	"github.com/DBCDK/morph/ssh"
	"github.com/DBCDK/morph/ui"
	"github.com/DBCDK/morph/utils"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"os"
	"strings"
	"time"
)

// These are set at build time via -ldflags magic
var version string
var assetRoot string

var switchActions = []string{"dry-activate", "test", "switch", "boot"}
var planActions = []string{"run", "resume"}

var (
	app                 = kingpin.New("morph", "NixOS host manager").Version(version)
	dryRun              = app.Flag("dry-run", "Don't do anything, just eval and print changes").Default("False").Bool()
	jsonOutput          = app.Flag("i-know-kung-fu", "Output as JSON").Default("False").Bool()
	selectGlob          string
	selectTags          string
	selectEvery         int
	selectSkip          int
	selectLimit         int
	orderingTags        string
	deployment          string
	timeout             int
	askForSudoPasswd    bool
	passCmd             string
	nixBuildArg         []string
	nixBuildTarget      string
	nixBuildTargetFile  string
	build               = buildCmd(app.Command("build", "Evaluate and build deployment configuration to the local Nix store"))
	eval                = evalCmd(app.Command("eval", "Inspect value of an attribute without building"))
	push                = pushCmd(app.Command("push", "Build and transfer items from the local Nix store to target machines"))
	deploy              = deployCmd(app.Command("deploy", "Build, push and activate new configuration on machines according to switch-action"))
	_planRoot           = app.Command("plan", "Create, run and resume plans")
	_planRun            = runPlanCmd(_planRoot.Command("run", "Run an existing plan"))
	_planResume         = resumePlanCmd(_planRoot.Command("resume", "Resume an existing plan"))
	planAction          string
	planFile            string
	deploySwitchAction  string
	deployUploadSecrets bool
	deployReboot        bool
	skipHealthChecks    bool
	skipPreDeployChecks bool
	showTrace           bool
	healthCheck         = healthCheckCmd(app.Command("check-health", "Run health checks"))
	uploadSecrets       = uploadSecretsCmd(app.Command("upload-secrets", "Upload secrets"))
	listSecrets         = listSecretsCmd(app.Command("list-secrets", "List secrets"))
	asJson              bool
	attrkey             string
	execute             = executeCmd(app.Command("exec", "Execute arbitrary commands on machines"))
	executeCommand      []string
	keepGCRoot          = app.Flag("keep-result", "Keep latest build in .gcroots to prevent it from being garbage collected").Default("False").Bool()
	allowBuildShell     = app.Flag("allow-build-shell", "Allow using `network.buildShell` to build in a nix-shell which can execute arbitrary commands on the local system").Default("False").Bool()
	planOnly            = app.Flag("plan-only", "Print the execution plan and exit").Default("False").Bool()
	hostsMap            = make(map[string]nix.Host)
	dotFile             = app.Flag("dot-file", "file to write plan to as a Graphviz dot-file").String()
)

func deploymentArg(cmd *kingpin.CmdClause) {
	cmd.Arg("deployment", "File containing the nix deployment expression").
		HintFiles("nix").
		Required().
		ExistingFileVar(&deployment)
}

func attributeArg(cmd *kingpin.CmdClause) {
	cmd.Arg("attribute", "Name of attribute to inspect").
		Required().
		StringVar(&attrkey)
}

func timeoutFlag(cmd *kingpin.CmdClause) {
	cmd.Flag("timeout", "Seconds to wait for commands/healthchecks on a host to complete").
		Default("0").
		IntVar(&timeout)
}

func askForSudoPasswdFlag(cmd *kingpin.CmdClause) {
	cmd.
		Flag("passwd", "Whether to ask interactively for remote sudo password when needed").
		Default("False").
		BoolVar(&askForSudoPasswd)
}

func getSudoPasswdCommand(cmd *kingpin.CmdClause) {
	cmd.
		Flag("passcmd", "Specify command to run for sudo password").
		Default("").
		StringVar(&passCmd)
}

func selectorFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("on", "Glob for selecting servers in the deployment").
		Default("*").
		StringVar(&selectGlob)
	cmd.Flag("tagged", "Select hosts with these tags").
		Default("").
		StringVar(&selectTags)
	cmd.Flag("every", "Select every n hosts").
		Default("1").
		IntVar(&selectEvery)
	cmd.Flag("skip", "Skip first n hosts").
		Default("0").
		IntVar(&selectSkip)
	cmd.Flag("limit", "Select at most n hosts").
		IntVar(&selectLimit)
	cmd.Flag("order-by-tags", "Order hosts by tags (comma separated list)").
		Default("").
		StringVar(&orderingTags)
}

func nixBuildArgFlag(cmd *kingpin.CmdClause) {
	cmd.Flag("build-arg", "Extra argument to pass on to nix-build command. **DEPRECATED**").
		StringsVar(&nixBuildArg)
}

func nixBuildTargetFlag(cmd *kingpin.CmdClause) {
	cmd.Flag("target", "A Nix lambda defining the build target to use instead of the default").
		StringVar(&nixBuildTarget)
}

func nixBuildTargetFileFlag(cmd *kingpin.CmdClause) {
	cmd.Flag("target-file", "File containing a Nix attribute set, defining build targets to use instead of the default").
		HintFiles("nix").
		ExistingFileVar(&nixBuildTargetFile)
}

func skipHealthChecksFlag(cmd *kingpin.CmdClause) {
	cmd.
		Flag("skip-health-checks", "Whether to skip all health checks").
		Default("False").
		BoolVar(&skipHealthChecks)
}

func skipPreDeployChecksFlag(cmd *kingpin.CmdClause) {
	cmd.
		Flag("skip-pre-deploy-checks", "Whether to skip all pre-deploy checks").
		Default("False").
		BoolVar(&skipPreDeployChecks)
}

func showTraceFlag(cmd *kingpin.CmdClause) {
	cmd.
		Flag("show-trace", "Whether to pass --show-trace to all nix commands").
		Default("False").
		BoolVar(&showTrace)
}

func asJsonFlag(cmd *kingpin.CmdClause) {
	cmd.
		Flag("json", "Whether to format the output as JSON instead of plaintext").
		Default("False").
		BoolVar(&asJson)
}

func evalCmd(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	deploymentArg(cmd)
	attributeArg(cmd)
	return cmd
}

func buildCmd(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	selectorFlags(cmd)
	showTraceFlag(cmd)
	nixBuildArgFlag(cmd)
	nixBuildTargetFlag(cmd)
	nixBuildTargetFileFlag(cmd)
	deploymentArg(cmd)
	return cmd
}

func pushCmd(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	selectorFlags(cmd)
	showTraceFlag(cmd)
	deploymentArg(cmd)
	return cmd
}

func executeCmd(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	selectorFlags(cmd)
	showTraceFlag(cmd)
	askForSudoPasswdFlag(cmd)
	getSudoPasswdCommand(cmd)
	timeoutFlag(cmd)
	deploymentArg(cmd)
	cmd.
		Arg("command", "Command to execute").
		Required().
		StringsVar(&executeCommand)
	cmd.NoInterspersed = true
	return cmd
}

func deployCmd(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	selectorFlags(cmd)
	showTraceFlag(cmd)
	nixBuildArgFlag(cmd)
	deploymentArg(cmd)
	timeoutFlag(cmd)
	askForSudoPasswdFlag(cmd)
	getSudoPasswdCommand(cmd)
	skipHealthChecksFlag(cmd)
	skipPreDeployChecksFlag(cmd)
	cmd.
		Flag("upload-secrets", "Upload secrets as part of the host deployment").
		Default("False").
		BoolVar(&deployUploadSecrets)
	cmd.
		Flag("reboot", "Reboots the host after system activation, but before healthchecks has executed.").
		Default("False").
		BoolVar(&deployReboot)
	cmd.
		Arg("switch-action", "Either of "+strings.Join(switchActions, "|")).
		Required().
		HintOptions(switchActions...).
		EnumVar(&deploySwitchAction, switchActions...)
	return cmd
}

func planFileArg(cmd *kingpin.CmdClause) {
	cmd.Arg("plan", "File containing the deployment plan").
		HintFiles("json").
		Required().
		ExistingFileVar(&planFile)
}

func runPlanCmd(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	planFileArg(cmd)

	return cmd
}

func resumePlanCmd(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	planFileArg(cmd)

	return cmd
}

func healthCheckCmd(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	selectorFlags(cmd)
	showTraceFlag(cmd)
	deploymentArg(cmd)
	timeoutFlag(cmd)
	return cmd
}

func uploadSecretsCmd(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	selectorFlags(cmd)
	showTraceFlag(cmd)
	askForSudoPasswdFlag(cmd)
	getSudoPasswdCommand(cmd)
	skipHealthChecksFlag(cmd)
	deploymentArg(cmd)
	return cmd
}

func listSecretsCmd(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	selectorFlags(cmd)
	showTraceFlag(cmd)
	deploymentArg(cmd)
	asJsonFlag(cmd)
	return cmd
}

func setup() {
	utils.ValidateEnvironment("nix")

	utils.SignalHandler()

	if assetRoot == "" {
		common.HandleError(errors.New("Morph must be compiled with \"-ldflags=-X main.assetRoot=<path-to-installed-data/>\"."))
	}
}

func main() {

	// TODO: Implement "context" part of a plan is running on:
	// Default context = local
	// A context can change the host something is running on
	// nix copy morph itself to the new host, and execute it with the sub-plan
	// eg context:local -> repeat-until-success -> context:$host -> exec command (health check)
	// Running the deploy from Matrix can also be a context, and the local morph will wait for it to finish
	// Try substitute on remote host before building locally (to avoid downloaded things that will be substituted from cache
	// Wrap exec.Command to unify logging the command and to give each command a unique ID that can be used to reconstruct what is being logged
	// Find a way to log commands where host is the machine executing morph
	// Detect when a process wants STDIN-input and find a way to handle that - mostly relevant for SSH TOFU prompts
	// Tyvstjæl hvordan K8s deployments virker med min og max men brug det på tags. Når dette aktiveres skal morph først health checke alt så morph ved hvor meget der er oppe og nede
	// Replace tags with labels

	// Metrics that can be reacted on

	clause := kingpin.MustParse(app.Parse(os.Args[1:]))

	// Don't actually run the UI unless activated
	tui := ui.DoTea()

	if !*jsonOutput {
		go func() {
			if _, err := tui.Run(); err != nil {
				fmt.Printf("morph failed: %v", err)
				os.Exit(1)
			}
		}()

		tui.Send(ui.LogEvent{Data: "ping"})
		go func() {
			time.Sleep(time.Second)
			tui.Send(ui.LogEvent{Data: "ping"})
		}()

		time.Sleep(time.Second)

		log.Logger = log.Output(zerolog.ConsoleWriter{
			Out: ui.LogWriter{Program: tui},
			//NoColor: true,
		})

		//load the UI here and redirect logs to a bubbletea pane instead
	} else {
		//log.Logger = log.Output(zerolog.New(os.Stdout).With().Timestamp().Logger())
		log.Logger = log.Output(os.Stdout)
	}

	//TODO: Remove deprecation warning when removing --build-arg flag
	if len(nixBuildArg) > 0 {
		log.Warn().Msg("Deprecation: The --build-arg flag will be removed in a future release.")
	}

	defer utils.RunFinalizers()
	setup()

	mctx := &common.MorphContext{
		SSHContext:          ssh.CreateSSHContext(askForSudoPasswd, passCmd),
		NixContext:          nix.GetNixContext(assetRoot, showTrace, *keepGCRoot, *allowBuildShell),
		AssetRoot:           assetRoot,
		AttrKey:             attrkey,
		Deployment:          deployment,
		DeploySwitchAction:  deploySwitchAction,
		DeployReboot:        deployReboot,
		DeployUploadSecrets: deployUploadSecrets,
		DryRun:              *dryRun,
		ExecuteCommand:      executeCommand,
		NixBuildArg:         nixBuildArg,
		NixBuildTarget:      nixBuildTarget,
		NixBuildTargetFile:  nixBuildTargetFile,
		OrderingTags:        orderingTags, // FIXME: should these be split already here?
		SelectEvery:         selectEvery,
		SelectGlob:          selectGlob,
		SelectLimit:         selectLimit,
		SelectSkip:          selectSkip,
		SelectTags:          selectTags, // FIXME: should these be split already here?
		SkipHealthChecks:    skipHealthChecks,
		SkipPreDeployChecks: skipPreDeployChecks,
		Timeout:             timeout,
	}

	// evaluate without building hosts
	switch clause {
	case eval.FullCommand():
		_, err := cruft.ExecEval(mctx)
		common.HandleError(err)
		return
	}

	switch clause {
	case _planRun.FullCommand():
		// FIXME: embed plan instead of file path
		log.Info().
			Str("plan", planFile).
			Msg("running plan")

	case _planResume.FullCommand():
		// FIXME: embed plan instead of file path
		log.Info().
			Str("plan", planFile).
			Msg("resuming plan")

	default:
		// setup hosts
		// FIXME: Should this be its own step instead?
		// But then what about the generated plan? It can no longer contain the lists of hosts, but only the deployment and filters users, which will make resume difficult..
		deploymentMetadata, hosts, err := cruft.GetHosts(mctx, deployment)
		common.HandleError(err)

		for _, host := range hosts {
			hostsMap[host.Name] = host
		}

		plan := createPlan(hosts, clause)
		if !*jsonOutput {
			tui.Send(plan)
		}

		planJson, err := json.Marshal(plan)
		if err != nil {
			log.Error().Err(err).Msg("error marshalling plan to JSON")
			return
		}

		log.Info().
			Str("event", "plan").
			RawJSON("plan", planJson).
			Msg("Generated plan")

		if dotFile != nil && *dotFile != "" {
			f, err := os.Create(*dotFile)
			if err != nil {
				panic(err)
			}
			defer f.Close()

			writer := bufio.NewWriter(f)
			planner.WriteDotFile(writer, plan)
			writer.Flush()
		}

		if *planOnly { // FIXME: Is this really *dryRun instead? Not sure
			// Don't execute the plan
			return
		}

		cache_ := cache.NewLockedMap[string]("cache")
		stepsDone := cache.NewLockedMap[string]("steps-done")
		stepsDb := cache.NewLockedMap[planner.Step]("steps")

		megaContext := planner.MegaContext{
			Hosts:        hostsMap, // FIXME: Either get rid of this, or set the hosts from a new EvalDeployment step. Each deployment will need its own megaContext probably.
			MorphContext: mctx,
			SSHContext:   ssh.CreateSSHContext(askForSudoPasswd, passCmd),
			NixContext:   nix.GetNixContext(assetRoot, showTrace, *keepGCRoot, *allowBuildShell),
			Cache:        &cache_,
			StepsDone:    &stepsDone,
			Steps:        &stepsDb,
			UIActive:     !*jsonOutput,
			UI:           tui,
			Constraints:  deploymentMetadata.Constraints,
		}

		go planner.StepMonitor(&stepsDb, &stepsDone)

		err = planner.ExecuteStep(context.TODO(), megaContext, plan)
		if err != nil {
			log.Error().Err(err).Msg("Error while running step") // FIXME: Log the offending step/action somehow
			// FIXME: Dump the plan with status on what was done, and what wasn't, so it can be resumed
			if *jsonOutput {
				// don't os.Exit if running with UI
				os.Exit(1)
			}

		}

		if !*jsonOutput {
			// Let the user terminate morph from the UI
			tui.Wait()
		}
	}

	// switch clause {
	// case build.FullCommand():
	// 	_, err = execBuild(hosts)
	// case push.FullCommand():
	// 	_, err = execPush(hosts)
	// case deploy.FullCommand():
	// 	_, err = execDeploy(hosts)
	// case healthCheck.FullCommand():
	// 	err = execHealthCheck(hosts)
	// case uploadSecrets.FullCommand():
	// 	err = execUploadSecrets(createSSHContext(), hosts, nil)
	// case listSecrets.FullCommand():
	// 	if asJson {
	// 		err = execListSecretsAsJson(hosts)
	// 	} else {
	// 		execListSecrets(hosts)
	// 	}
	// case execute.FullCommand():
	// 	err = execExecute(hosts)
	// }

	// common.HandleError(err)
}

func createPlan(hosts []nix.Host, clause string) planner.Step {
	plan := planner.EmptyStep()
	plan.Id = "root"
	plan.Description = "Root of execution plan"
	plan.Parallel = true

	buildPlan := planner.CreateBuildPlan(hosts)

	hostSpecificPlans := make(map[string]planner.Step, 0)

	for _, host := range hosts {
		hostSpecificPlan := planner.EmptyStep()
		hostSpecificPlan.Description = "host: " + host.Name
		hostSpecificPlan.Action = actions.None{}
		hostSpecificPlan.Parallel = false
		hostSpecificPlan.DependsOn = []string{buildPlan.Id}
		hostSpecificPlan.Labels = host.Labels

		hostSpecificPlans[host.Name] = hostSpecificPlan
	}

	stepGetSudoPasswd := planner.CreateStepGetSudoPasswd()

	if askForSudoPasswd {
		plan = planner.AddSteps(plan, stepGetSudoPasswd)
	}

	switch clause {
	case build.FullCommand():

		plan = planner.AddSteps(plan, buildPlan)

	case push.FullCommand():

		plan = planner.AddSteps(plan, buildPlan)

		for _, host := range hosts {
			push := planner.CreateStepPush(host)
			push.DependsOn = append(push.DependsOn, buildPlan.Id)

			hostSpecificPlans[host.Name] = planner.AddStepsSeq(
				hostSpecificPlans[host.Name],
				push,
			)
		}

	case deploy.FullCommand():
		plan = planner.AddSteps(plan, buildPlan)

		for _, host := range hosts {
			push := planner.CreateStepPush(host)
			push.DependsOn = append(push.DependsOn, buildPlan.Id)

			deployDryActivate := planner.CreateStepDeployDryActivate(host)
			deploySwitch := planner.CreateStepDeploySwitch(host)
			deployTest := planner.CreateStepDeployTest(host)
			deployBoot := planner.CreateStepDeployBoot(host)

			stepReboot := planner.CreateStepReboot(host)

			stepWaitForOnline := planner.CreateStepWaitForOnline(host)

			preDeployChecks := planner.CreateStepChecks(
				host,
				make([]planner.CommandPlus, 0),
				planner.HealthChecksToCommands(host.PreDeployChecks.Cmd),
				make([]planner.RequestPlus, 0),
				planner.HealthChecksToRequests(host.PreDeployChecks.Http),
			)

			healthChecks := planner.CreateStepChecks(
				host,
				make([]planner.CommandPlus, 0),
				planner.HealthChecksToCommands(host.HealthChecks.Cmd),
				make([]planner.RequestPlus, 0),
				planner.HealthChecksToRequests(host.HealthChecks.Http),
			)

			if skipPreDeployChecks {
				preDeployChecks = planner.CreateStepSkip(preDeployChecks)
			}

			if skipHealthChecks {
				healthChecks = planner.CreateStepSkip(healthChecks)
			}

			if askForSudoPasswd {
				deployDryActivate.DependsOn = append(deployDryActivate.DependsOn, stepGetSudoPasswd.Id)
				deploySwitch.DependsOn = append(deploySwitch.DependsOn, stepGetSudoPasswd.Id)
				deployTest.DependsOn = append(deployTest.DependsOn, stepGetSudoPasswd.Id)
				deployBoot.DependsOn = append(deployBoot.DependsOn, stepGetSudoPasswd.Id)
				stepReboot.DependsOn = append(stepReboot.DependsOn, stepGetSudoPasswd.Id)
			}

			switch deploySwitchAction {
			case "dry-activate":
				hostSpecificPlans[host.Name] = planner.AddStepsSeq(
					hostSpecificPlans[host.Name],
					push,
					deployDryActivate,
				)

			case "test":
				// FIXME: requires upload secrets
				hostSpecificPlans[host.Name] = planner.AddStepsSeq(
					hostSpecificPlans[host.Name],
					push,
					preDeployChecks,
					deployTest,
					healthChecks,
				)

			case "switch":
				hostSpecificPlans[host.Name] = planner.AddStepsSeq(
					hostSpecificPlans[host.Name],
					push,
					preDeployChecks,
					deploySwitch,
					healthChecks,
				)

			case "boot":
				// FIXME: requires upload secrets
				hostSpecificPlans[host.Name] = planner.AddStepsSeq(
					hostSpecificPlans[host.Name],
					push,
					preDeployChecks,
					deployBoot,
				)
			}

			// reboot can be added to any action, even if weird..
			if deployReboot {
				hostSpecificPlans[host.Name] = planner.AddStepsSeq(
					hostSpecificPlans[host.Name],
					stepReboot,
					stepWaitForOnline,
					healthChecks,
				)
			}
		}

	case healthCheck.FullCommand():

		plan = planner.AddSteps(plan, buildPlan)

		for _, host := range hosts {
			push := planner.CreateStepPush(host)
			push.DependsOn = append(push.DependsOn, buildPlan.Id)

			healthChecks := planner.CreateStepChecks(
				host,
				make([]planner.CommandPlus, 0),
				planner.HealthChecksToCommands(host.HealthChecks.Cmd),
				make([]planner.RequestPlus, 0),
				planner.HealthChecksToRequests(host.HealthChecks.Http),
			)

			hostSpecificPlans[host.Name] = planner.AddStepsSeq(
				hostSpecificPlans[host.Name],
				push,
				healthChecks,
			)
		}

	case uploadSecrets.FullCommand():
		log.Error().Msg("Execution plan: deploy: Not implemented")

	case listSecrets.FullCommand():
		log.Error().Msg("Execution plan: deploy: Not implemented")

	case execute.FullCommand():
		log.Error().Msg("Execution plan: execute: Not implemented")

	}

	for _, serverPlan := range hostSpecificPlans {
		// FIXME: This is a bit too much of a hack just to
		// avoid build including empty plans
		if len(serverPlan.Steps) > 0 {
			plan = planner.AddSteps(plan, serverPlan)
		}
	}

	return plan
}
