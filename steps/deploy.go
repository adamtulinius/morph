package steps

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/DBCDK/morph/cache"
	"github.com/DBCDK/morph/common"
	"github.com/DBCDK/morph/nix"
	"github.com/rs/zerolog/log"
)

type DeployBoot struct {
	Host string `json:"host"`
}

type DeployDryActivate struct {
	Host string `json:"host"`
}

type DeploySwitch struct {
	Host string `json:"host"`
}

type DeployTest struct {
	Host string `json:"host"`
}

func (_ DeployBoot) Name() string        { return "deploy-boot" }
func (_ DeployDryActivate) Name() string { return "deploy-dry-activate" }
func (_ DeploySwitch) Name() string      { return "deploy-switch" }
func (_ DeployTest) Name() string        { return "deploy-test" }

func (action *DeployBoot) MarshalJSONx(step Step) ([]byte, error) {
	type StepAlias Step

	return json.Marshal(struct {
		StepAlias
		DeployBoot
	}{
		StepAlias:  StepAlias(step),
		DeployBoot: *action,
	})
}

func (action *DeployDryActivate) MarshalJSONx(step Step) ([]byte, error) {
	type StepAlias Step

	return json.Marshal(struct {
		StepAlias
		DeployDryActivate
	}{
		StepAlias:         StepAlias(step),
		DeployDryActivate: *action,
	})
}

func (action *DeploySwitch) MarshalJSONx(step Step) ([]byte, error) {
	type StepAlias Step

	return json.Marshal(struct {
		StepAlias
		DeploySwitch
	}{
		StepAlias:    StepAlias(step),
		DeploySwitch: *action,
	})
}

func (action *DeployTest) MarshalJSONx(step Step) ([]byte, error) {
	type StepAlias Step

	return json.Marshal(struct {
		StepAlias
		DeployTest
	}{
		StepAlias:  StepAlias(step),
		DeployTest: *action,
	})
}

func (action *DeployBoot) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, &action)
}

func (action *DeployDryActivate) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, &action)
}

func (action *DeploySwitch) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, &action)
}

func (action *DeployTest) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, &action)
}

func (action DeployBoot) Run(ctx context.Context, mctx *common.MorphContext, allHosts map[string]nix.Host, cache_ *cache.LockedMap[string]) error {
	host, ok := allHosts[action.Host]
	if !ok {
		return errors.New(fmt.Sprintf("host '%s' not in deployment", action.Host))
	}

	return deploy(ctx, mctx, cache_, host, "boot")
}

func (action DeployDryActivate) Run(ctx context.Context, mctx *common.MorphContext, allHosts map[string]nix.Host, cache_ *cache.LockedMap[string]) error {
	host, ok := allHosts[action.Host]
	if !ok {
		return errors.New(fmt.Sprintf("host '%s' not in deployment", action.Host))
	}

	return deploy(ctx, mctx, cache_, host, "dry-activate")
}

func (action DeploySwitch) Run(ctx context.Context, mctx *common.MorphContext, allHosts map[string]nix.Host, cache_ *cache.LockedMap[string]) error {
	host, ok := allHosts[action.Host]
	if !ok {
		return errors.New(fmt.Sprintf("host '%s' not in deployment", action.Host))
	}

	return deploy(ctx, mctx, cache_, host, "switch")
}

func (action DeployTest) Run(ctx context.Context, mctx *common.MorphContext, allHosts map[string]nix.Host, cache_ *cache.LockedMap[string]) error {
	host, ok := allHosts[action.Host]
	if !ok {
		return errors.New(fmt.Sprintf("host '%s' not in deployment", action.Host))
	}

	return deploy(ctx, mctx, cache_, host, "test")
}

func deploy(ctx context.Context, mctx *common.MorphContext, cache_ *cache.LockedMap[string], host nix.Host, deployAction string) error {
	log.Info().Msg(fmt.Sprintf("Executing %s on %s", deployAction, host.Name))

	closure, err := cache_.Get("closure:" + host.Name)
	if err != nil {
		return err
	}

	err = mctx.SSHContext.ActivateConfiguration(&host, closure, deployAction)
	if err != nil {
		return err
	}

	return nil
}
