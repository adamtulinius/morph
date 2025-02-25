package steps

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/DBCDK/morph/cache"
	"github.com/DBCDK/morph/common"
	"github.com/DBCDK/morph/logging"
	"github.com/DBCDK/morph/nix"
)

type IsOnline struct {
	Host string `json:"host"`
}

type Reboot struct {
	Host string `json:"host"`
}

func (_ IsOnline) Name() string { return "is-online" }
func (_ Reboot) Name() string   { return "reboot" }

func (action *IsOnline) MarshalJSONx(step Step) ([]byte, error) {
	return json.Marshal(struct {
		StepAlias
		IsOnline
	}{
		StepAlias: StepAlias(step),
		IsOnline:  *action,
	})
}

func (action *Reboot) MarshalJSONx(step Step) ([]byte, error) {
	return json.Marshal(struct {
		StepAlias
		Reboot
	}{
		StepAlias: StepAlias(step),
		Reboot:    *action,
	})
}

func (action *IsOnline) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, action)
}

func (action *Reboot) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, action)
}

func (action IsOnline) Run(ctx context.Context, mctx *common.MorphContext, allHosts map[string]nix.Host, cache_ *cache.LockedMap[string]) error {
	host, ok := allHosts[action.Host]
	if !ok {
		return errors.New(fmt.Sprintf("host '%s' not in deployment", action.Host))
	}

	cmd, err := mctx.SSHContext.CmdContext(ctx, &host, "/bin/sh", "-c", "true")
	if err != nil {
		return err
	}

	logging.LogCmd(action.Host, cmd)

	err = cmd.Run()

	return err
}
func (action Reboot) Run(ctx context.Context, mctx *common.MorphContext, allHosts map[string]nix.Host, cache_ *cache.LockedMap[string]) error {
	host, exists := allHosts[action.Host]
	if !exists {
		return errors.New("unknown host: " + action.Host)
	}

	err := host.Reboot(mctx.SSHContext)

	return err
}
