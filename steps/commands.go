package steps

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/DBCDK/morph/cache"
	"github.com/DBCDK/morph/common"
	"github.com/DBCDK/morph/nix"
)

type LocalCommand struct {
	Command []string `json:"command"`
	Timeout int      `json:"timeout"`
}

type RemoteCommand struct {
	Command []string `json:"command"`
	Timeout int      `json:"timeout"`
}

func (_ LocalCommand) Name() string  { return "local-command" }
func (_ RemoteCommand) Name() string { return "remote-command" }

func (action *LocalCommand) MarshalJSONx(step Step) ([]byte, error) {
	return json.Marshal(struct {
		StepAlias
		LocalCommand
	}{
		StepAlias:    StepAlias(step),
		LocalCommand: *action,
	})
}

func (action *RemoteCommand) MarshalJSONx(step Step) ([]byte, error) {
	return json.Marshal(struct {
		StepAlias
		RemoteCommand
	}{
		StepAlias:     StepAlias(step),
		RemoteCommand: *action,
	})
}

func (action *LocalCommand) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, action)
}

func (action *RemoteCommand) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, action)
}

func (action LocalCommand) Run(ctx context.Context, mctx *common.MorphContext, allHosts map[string]nix.Host, cache_ *cache.LockedMap[string]) error {
	return errors.New("not implemented: " + action.Name())
}

func (action RemoteCommand) Run(ctx context.Context, mctx *common.MorphContext, allHosts map[string]nix.Host, cache_ *cache.LockedMap[string]) error {
	return errors.New("not implemented: " + action.Name())
}
