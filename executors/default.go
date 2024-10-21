package executors

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/DBCDK/morph/common"
	"github.com/DBCDK/morph/cruft"
	"github.com/DBCDK/morph/nix"
	"github.com/DBCDK/morph/planner"
	"github.com/DBCDK/morph/ssh"
)

// FIXME: IDEA: Deployment simulation - make a fake MorphContext where things like SSH-calls are faked and logged instead

type StepSettings struct {
	Parallel bool
}

type StepX interface {
	SetOptions(options StepSettings)
	Run(mctx *common.MorphContext, cache *planner.Cache) error
}

type Build struct {
	hosts []nix.Host
}

func (step Build) Run(mctx *common.MorphContext, cache *planner.Cache) error {
	resultPath, err := cruft.ExecBuild(mctx, step.hosts)
	if err != nil {
		return err
	}

	fmt.Println(resultPath)

	for _, host := range step.hosts {
		hostPathSymlink := path.Join(resultPath, host.Name)
		hostPath, err := filepath.EvalSymlinks(hostPathSymlink)
		if err != nil {
			return err
		}

		fmt.Println(hostPathSymlink)
		fmt.Println(hostPath)

		cache.Update(planner.StepData{Key: "closure:" + host.Name, Value: hostPath})
	}

	return err
}

type Push struct {
	host nix.Host
}

func (step Push) Run(mctx *common.MorphContext, cache *planner.Cache) error {
	cacheKey := "closure:" + step.host.Name
	fmt.Println("cache key: " + cacheKey)
	closure, err := cache.Get(cacheKey)
	if err != nil {
		return err
	}

	fmt.Printf("Pushing %s to %s\n", closure, step.host.TargetHost)

	err = nix.Push(mctx.SSHContext, step.host, closure)

	return err
}

// func (step DeploySwitch) Run(mctx *common.MorphContext, cache *planner.Cache) error {

// }

func deployAction(mctx *common.MorphContext, cache *planner.Cache, host nix.Host, deployAction string) error {
	fmt.Fprintf(os.Stderr, "Executing %s on %s", deployAction, host.Name)

	closure, err := cache.Get("closure:" + host.Name)
	if err != nil {
		return err
	}

	err = mctx.SSHContext.ActivateConfiguration(&host, closure, deployAction)
	if err != nil {
		return err
	}

	return nil
}

type DeployBoot struct {
	host nix.Host
}

type DeployDryActivate struct {
	host nix.Host
}

type DeploySwitch struct {
	host nix.Host
}

type DeployTest struct {
	host nix.Host
}

func (step DeployBoot) Run(mctx *common.MorphContext, cache *planner.Cache) error {
	return deployAction(mctx, cache, step.host, "boot")
}

func (step DeployDryActivate) Run(mctx *common.MorphContext, cache *planner.Cache) error {
	return deployAction(mctx, cache, step.host, "dry-activate")
}

func (step DeploySwitch) Run(mctx *common.MorphContext, cache *planner.Cache) error {
	return deployAction(mctx, cache, step.host, "switch")
}

func (step DeployTest) Run(mctx *common.MorphContext, cache *planner.Cache) error {
	return deployAction(mctx, cache, step.host, "test")
}

type DefaultPlanExecutor struct {
	Hosts        map[string]nix.Host
	MorphContext *common.MorphContext
	SSHContext   *ssh.SSHContext
	NixContext   *nix.NixContext
	Cache        planner.Cache
}

func (ex DefaultPlanExecutor) GetHosts() map[string]nix.Host {
	return ex.Hosts
}

func (ex DefaultPlanExecutor) GetMorphContext() *common.MorphContext {
	return ex.MorphContext
}

func (ex DefaultPlanExecutor) GetSSHContext() *ssh.SSHContext {
	return ex.SSHContext
}

func (ex DefaultPlanExecutor) GetNixContext() *nix.NixContext {
	return ex.NixContext
}

// func (ex DefaultPlanExecutor) SetCache(cache planner.Cache) {
// 	ex.Cache = cache
// 	problably something goes wrong here ^
// }

func (executor DefaultPlanExecutor) Build(step planner.Step) error {
	hostsByName := step.Options["hosts"].([]string)

	nixHosts := make([]nix.Host, 0)

	fmt.Println("Building hosts:")
	for _, hostByName := range hostsByName {
		fmt.Printf("- %s\n", hostByName)
		nixHosts = append(nixHosts, executor.GetHosts()[hostByName])
	}

	resultPath, err := cruft.ExecBuild(executor.MorphContext, nixHosts)
	if err != nil {
		return err
	}

	fmt.Println(resultPath)

	for _, host := range nixHosts {
		hostPathSymlink := path.Join(resultPath, host.Name)
		hostPath, err := filepath.EvalSymlinks(hostPathSymlink)
		if err != nil {
			return err
		}

		fmt.Println(hostPathSymlink)
		fmt.Println(hostPath)

		executor.Cache.Update(planner.StepData{Key: "closure:" + host.Name, Value: hostPath})

		// store hostPath to be fetched by other steps
	}

	return err
}

func (executor DefaultPlanExecutor) Push(step planner.Step) error {
	cacheKey := "closure:" + step.Host.Name
	fmt.Println("cache key: " + cacheKey)
	closure, err := executor.Cache.Get(cacheKey)
	if err != nil {
		return err
	}

	fmt.Printf("Pushing %s to %s\n", closure, step.Host.TargetHost)

	sshContext := executor.GetSSHContext()
	err = nix.Push(sshContext, *step.Host, closure)

	return err
}

func (executor DefaultPlanExecutor) deployAction(action string, step planner.Step) error {
	fmt.Fprintf(os.Stderr, "Executing %s on %s", action, step.Host.Name)

	closure, err := executor.Cache.Get("closure:" + step.Host.Name)
	if err != nil {
		return err
	}

	err = executor.MorphContext.SSHContext.ActivateConfiguration(step.Host, closure, action)
	if err != nil {
		return err
	}

	return nil
}

func (executor DefaultPlanExecutor) DeploySwitch(step planner.Step) error {
	return executor.deployAction("switch", step)
}

func (executor DefaultPlanExecutor) DeployBoot(step planner.Step) error {
	return executor.deployAction("boot", step)
}

func (executor DefaultPlanExecutor) DeployDryActivate(step planner.Step) error {
	return executor.deployAction("dry-activate", step)
}

func (executor DefaultPlanExecutor) DeployTest(step planner.Step) error {
	return executor.deployAction("test", step)
}

func (executor DefaultPlanExecutor) Reboot(step planner.Step) error {

	return nil
}

func (executor DefaultPlanExecutor) CommandCheckLocal(step planner.Step) error {

	return nil
}

func (executor DefaultPlanExecutor) CommandCheckRemote(step planner.Step) error {

	return nil
}

func (executor DefaultPlanExecutor) HttpCheckLocal(step planner.Step) error {

	return nil
}

func (executor DefaultPlanExecutor) HttpCheckRemote(step planner.Step) error {

	return nil
}
