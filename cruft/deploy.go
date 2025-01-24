package cruft

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"os"
	"path/filepath"
	"strings"

	"github.com/DBCDK/morph/common"
	"github.com/DBCDK/morph/filter"
	"github.com/DBCDK/morph/healthchecks"
	"github.com/DBCDK/morph/nix"
	"github.com/DBCDK/morph/secrets"
	"github.com/DBCDK/morph/ssh"
	"github.com/DBCDK/morph/utils"
)

func execHealthCheck(mctx common.MorphContext, hosts []nix.Host) error {
	var err error
	for _, host := range hosts {
		if host.BuildOnly {
			fmt.Fprintf(os.Stderr, "Healthchecks are disabled for build-only host: %s\n", host.Name)
			continue
		}
		err = healthchecks.PerformHealthChecks(mctx.SSHContext, &host, mctx.Timeout)
	}

	if err != nil {
		err = errors.New("One or more errors occurred during host healthchecks")
	}

	return err
}

func execUploadSecrets(mctx *common.MorphContext, hosts []nix.Host, phase *string) error {
	for _, host := range hosts {
		if host.BuildOnly {
			fmt.Fprintf(os.Stderr, "Secret upload is disabled for build-only host: %s\n", host.Name)
			continue
		}
		singleHostInList := []nix.Host{host}

		err := secretsUpload(mctx, singleHostInList, phase)
		if err != nil {
			return err
		}

		if !mctx.SkipHealthChecks {
			err = healthchecks.PerformHealthChecks(mctx.SSHContext, &host, mctx.Timeout)
			if err != nil {
				fmt.Fprintln(os.Stderr)
				fmt.Fprintln(os.Stderr, "Not uploading to additional hosts, since a host health check failed.")
				return err
			}
		}
	}

	return nil
}

func execListSecrets(hosts []nix.Host) {
	for _, host := range hosts {
		singleHostInList := []nix.Host{host}
		for _, host := range singleHostInList {
			fmt.Fprintf(os.Stdout, "Secrets for host %s:\n", host.Name)
			for name, secret := range host.Secrets {
				fmt.Fprintf(os.Stdout, "%s:\n- %v\n", name, &secret)
			}
		}
	}
}

func execListSecretsAsJson(mctx *common.MorphContext, hosts []nix.Host) error {
	deploymentDir, err := filepath.Abs(filepath.Dir(mctx.Deployment))
	if err != nil {
		return err
	}
	secretsByHost := make(map[string](map[string]secrets.Secret))

	for _, host := range hosts {
		singleHostInList := []nix.Host{host}
		for _, host := range singleHostInList {
			canonicalSecrets := make(map[string]secrets.Secret)
			for name, secret := range host.Secrets {
				sourcePath := utils.GetAbsPathRelativeTo(secret.Source, deploymentDir)
				secret.Source = sourcePath
				canonicalSecrets[name] = secret
			}
			secretsByHost[host.Name] = canonicalSecrets
		}
	}

	jsonSecrets, err := json.MarshalIndent(secretsByHost, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "%s\n", jsonSecrets)

	return nil
}

func GetHosts(mctx *common.MorphContext, deploymentPath string) (deploymentMetadata nix.DeploymentMetadata, hosts []nix.Host, err error) {

	deploymentFile, err := os.Open(deploymentPath)
	if err != nil {
		return deploymentMetadata, hosts, err
	}

	deploymentAbsPath, err := filepath.Abs(deploymentFile.Name())
	if err != nil {
		return deploymentMetadata, hosts, err
	}

	deployment, err := mctx.NixContext.GetMachines(deploymentAbsPath)
	if err != nil {
		return deploymentMetadata, hosts, err
	}

	matchingHosts, err := filter.MatchHosts(deployment.Hosts, mctx.SelectGlob)
	if err != nil {
		return deploymentMetadata, hosts, err
	}

	var selectedTags []string
	if mctx.SelectTags != "" {
		selectedTags = strings.Split(mctx.SelectTags, ",")
	}

	matchingHosts2 := filter.FilterHostsTags(matchingHosts, selectedTags)

	ordering := deployment.Meta.Ordering
	if mctx.OrderingTags != "" {
		ordering = nix.HostOrdering{Tags: strings.Split(mctx.OrderingTags, ",")}
	}

	sortedHosts := filter.SortHosts(matchingHosts2, ordering)

	filteredHosts := filter.FilterHosts(sortedHosts, mctx.SelectSkip, mctx.SelectEvery, mctx.SelectLimit)

	zLogHostsDict := zerolog.Dict()
	for _, host := range filteredHosts {
		zLogTagsArray := zerolog.Arr()
		for _, tag := range host.GetTags() {
			zLogTagsArray.Str(tag)
		}

		zLogHostsDict.Dict(
			host.Name,
			zerolog.Dict().
				Int("secrets", len(host.Secrets)).
				Int("health_checks", len(host.HealthChecks.Cmd)+len(host.HealthChecks.Http)).
				Array("tags", zLogTagsArray))

	}

	log.Info().
		Str("event", "deployment").
		Dict("hosts", zLogHostsDict).
		Msg("read deployment")

	return deployment.Meta, filteredHosts, nil
}

func buildHosts(mctx *common.MorphContext, hosts []nix.Host) (resultPath string, err error) {
	if len(hosts) == 0 {
		err = errors.New("No hosts selected")
		return
	}

	deploymentPath, err := filepath.Abs(mctx.Deployment)
	if err != nil {
		return
	}

	nixBuildTargets := ""
	if mctx.NixBuildTargetFile != "" {
		if path, err := filepath.Abs(mctx.NixBuildTargetFile); err == nil {
			nixBuildTargets = fmt.Sprintf("import \"%s\"", path)
		}
	} else if mctx.NixBuildTarget != "" {
		nixBuildTargets = fmt.Sprintf("{ \"out\" = %s; }", mctx.NixBuildTarget)
	}

	resultPath, err = mctx.NixContext.BuildMachines(deploymentPath, hosts, mctx.NixBuildArg, nixBuildTargets)

	if err != nil {
		return
	}

	log.Info().Msg("nix result path: " + resultPath)
	return
}

func pushPaths(sshContext *ssh.SSHContext, filteredHosts []nix.Host, resultPath string) error {
	for _, host := range filteredHosts {
		if host.BuildOnly {
			fmt.Fprintf(os.Stderr, "Push is disabled for build-only host: %s\n", host.Name)
			continue
		}

		paths, err := nix.GetPathsToPush(host, resultPath)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Pushing paths to %v (%v@%v):\n", host.Name, host.TargetUser, host.TargetHost)
		for _, path := range paths {
			fmt.Fprintf(os.Stderr, "\t* %s\n", path)
		}
		err = nix.Push(sshContext, host, paths...)
		if err != nil {
			return err
		}
	}

	return nil
}

func secretsUpload(mctx *common.MorphContext, filteredHosts []nix.Host, phase *string) error {
	// upload secrets
	// relative paths are resolved relative to the deployment file (!)
	deploymentDir := filepath.Dir(mctx.Deployment)
	for _, host := range filteredHosts {
		fmt.Fprintf(os.Stderr, "Uploading secrets to %s (%s):\n", host.Name, host.TargetHost)
		postUploadActions := make(map[string][]string, 0)
		for secretName, secret := range host.Secrets {
			// if phase is nil, upload the secrets no matter what phase it wants
			// if phase is non-nil, upload the secrets that match the specified phase
			if phase != nil && secret.UploadAt != *phase {
				continue
			}

			secretSize, err := secrets.GetSecretSize(secret, deploymentDir)
			if err != nil {
				return err
			}

			secretErr := secrets.UploadSecret(mctx.SSHContext, &host, secret, deploymentDir)
			fmt.Fprintf(os.Stderr, "\t* %s (%d bytes).. ", secretName, secretSize)
			if secretErr != nil {
				if secretErr.Fatal {
					fmt.Fprintln(os.Stderr, "Failed")
					return secretErr
				} else {
					fmt.Fprintln(os.Stderr, "Partial")
					fmt.Fprint(os.Stderr, secretErr.Error())
				}
			} else {
				fmt.Fprintln(os.Stderr, "OK")
			}
			if len(secret.Action) > 0 {
				// ensure each action is only run once
				postUploadActions[strings.Join(secret.Action, " ")] = secret.Action
			}
		}
		// Execute post-upload secret actions one-by-one after all secrets have been uploaded
		for _, action := range postUploadActions {
			fmt.Fprintf(os.Stderr, "\t- executing post-upload command: "+strings.Join(action, " ")+"\n")
			// Errors from secret actions will be printed on screen, but we won't stop the flow if they fail
			mctx.SSHContext.CmdInteractive(&host, mctx.Timeout, action...)
		}
	}

	return nil
}

func activateConfiguration(mctx *common.MorphContext, filteredHosts []nix.Host, resultPath string) error {
	fmt.Fprintln(os.Stderr, "Executing '"+mctx.DeploySwitchAction+"' on matched hosts:")
	fmt.Fprintln(os.Stderr)
	for _, host := range filteredHosts {

		fmt.Fprintln(os.Stderr, "** "+host.Name)

		configuration, err := nix.GetNixSystemPath(host, resultPath)
		if err != nil {
			return err
		}

		err = mctx.SSHContext.ActivateConfiguration(&host, configuration, mctx.DeploySwitchAction)
		if err != nil {
			return err
		}

		fmt.Fprintln(os.Stderr)
	}

	return nil
}

func execExecute(mctx *common.MorphContext, hosts []nix.Host) error {
	for _, host := range hosts {
		if host.BuildOnly {
			fmt.Fprintf(os.Stderr, "Exec is disabled for build-only host: %s\n", host.Name)
			continue
		}
		fmt.Fprintln(os.Stderr, "** "+host.Name)
		mctx.SSHContext.CmdInteractive(&host, mctx.Timeout, mctx.ExecuteCommand...)
		fmt.Fprintln(os.Stderr)
	}

	return nil
}

func ExecBuild(mctx *common.MorphContext, hosts []nix.Host) (string, error) {
	resultPath, err := buildHosts(mctx, hosts)
	if err != nil {
		return "", err
	}
	return resultPath, nil
}

func ExecEval(mctx *common.MorphContext) (string, error) {
	deploymentFile, err := os.Open(mctx.Deployment)
	deploymentPath, err := filepath.Abs(deploymentFile.Name())
	if err != nil {
		return "", err
	}

	path, err := mctx.NixContext.EvalHosts(deploymentPath, mctx.AttrKey)

	return path, err
}

func execPush(mctx *common.MorphContext, hosts []nix.Host) (string, error) {
	resultPath, err := ExecBuild(mctx, hosts)
	if err != nil {
		return "", err
	}

	fmt.Fprintln(os.Stderr)
	return resultPath, pushPaths(mctx.SSHContext, hosts, resultPath)
}

func execDeploy(mctx *common.MorphContext, hosts []nix.Host) (string, error) {
	doPush := false
	doUploadSecrets := false
	doActivate := false

	if !mctx.DryRun {
		switch mctx.DeploySwitchAction {
		case "dry-activate":
			doPush = true
			doActivate = true
		case "test":
			fallthrough
		case "switch":
			fallthrough
		case "boot":
			doPush = true
			doUploadSecrets = mctx.DeployUploadSecrets
			doActivate = true
		}
	}

	resultPath, err := buildHosts(mctx, hosts)
	if err != nil {
		return "", err
	}

	fmt.Fprintln(os.Stderr)

	for _, host := range hosts {
		if host.BuildOnly {
			fmt.Fprintf(os.Stderr, "Deployment steps are disabled for build-only host: %s\n", host.Name)
			continue
		}

		singleHostInList := []nix.Host{host}

		if doPush {
			err = pushPaths(mctx.SSHContext, singleHostInList, resultPath)
			if err != nil {
				return "", err
			}
		}
		fmt.Fprintln(os.Stderr)

		if doUploadSecrets {
			phase := "pre-activation"
			err = execUploadSecrets(mctx, singleHostInList, &phase)
			if err != nil {
				return "", err
			}

			fmt.Fprintln(os.Stderr)
		}

		if !mctx.SkipPreDeployChecks {
			err := healthchecks.PerformPreDeployChecks(mctx.SSHContext, &host, mctx.Timeout)
			if err != nil {
				fmt.Fprintln(os.Stderr)
				fmt.Fprintln(os.Stderr, "Not deploying to additional hosts, since a host pre-deploy check failed.")
				utils.Exit(1)
			}
		}

		if doActivate {
			err = activateConfiguration(mctx, singleHostInList, resultPath)
			if err != nil {
				return "", err
			}
		}

		if mctx.DeployReboot {
			err = host.Reboot(mctx.SSHContext)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Reboot failed")
				return "", err
			}
		}

		if doUploadSecrets {
			phase := "post-activation"
			err = execUploadSecrets(mctx, singleHostInList, &phase)
			if err != nil {
				return "", err
			}

			fmt.Fprintln(os.Stderr)
		}

		if !mctx.SkipHealthChecks {
			err := healthchecks.PerformHealthChecks(mctx.SSHContext, &host, mctx.Timeout)
			if err != nil {
				fmt.Fprintln(os.Stderr)
				fmt.Fprintln(os.Stderr, "Not deploying to additional hosts, since a host health check failed.")
				utils.Exit(1)
			}
		}

		fmt.Fprintln(os.Stderr, "Done:", host.Name)
	}

	return resultPath, nil
}
