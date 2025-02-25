package steps

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/DBCDK/morph/cache"
	"github.com/DBCDK/morph/common"
	"github.com/DBCDK/morph/nix"
)

type GetSudoPasswd struct{}

func (_ GetSudoPasswd) Name() string { return "get-sudo-password" }

func (action *GetSudoPasswd) MarshalJSONx(step Step) ([]byte, error) {
	return json.Marshal(struct {
		StepAlias
		GetSudoPasswd
	}{
		StepAlias:     StepAlias(step),
		GetSudoPasswd: *action,
	})
}

func (action *GetSudoPasswd) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, action)
}

func (step GetSudoPasswd) Run(ctx context.Context, mctx *common.MorphContext, allHosts map[string]nix.Host, cache_ *cache.LockedMap[string]) error {
	return errors.New("not implemented: " + step.Name())
}
