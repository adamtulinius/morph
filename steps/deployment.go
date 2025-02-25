package steps

import (
	"context"
	"errors"
	"github.com/DBCDK/morph/cache"
	"github.com/DBCDK/morph/common"
	"github.com/DBCDK/morph/nix"
)

type EvalDeployment struct {
	Deployment string `json:"deployment"`
}

func (_ EvalDeployment) Name() string { return "eval-deployment" }

func (step EvalDeployment) Run(ctx context.Context, mctx *common.MorphContext, allHosts map[string]nix.Host, cache_ *cache.LockedMap[string]) error {
	return errors.New("not implemented: " + step.Name())
}
