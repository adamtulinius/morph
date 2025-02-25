package steps

import (
	"context"
	"encoding/json"
	"github.com/DBCDK/morph/cache"
	"github.com/DBCDK/morph/common"
	"github.com/DBCDK/morph/nix"
)

type None struct{}
type Gate None
type Wrapper None

func (_ None) Name() string    { return "none" }
func (_ Gate) Name() string    { return "gate" }
func (_ Wrapper) Name() string { return "gate" }

func (_ *None) MarshalJSONx(step Step) ([]byte, error) {
	return json.Marshal(StepAlias(step))
}

func (_ *Gate) MarshalJSONx(step Step) ([]byte, error) {
	return json.Marshal(StepAlias(step))
}

func (_ *Wrapper) MarshalJSONx(step Step) ([]byte, error) {
	return json.Marshal(StepAlias(step))
}

func (action *None) UnmarshalJSON(b []byte) error {
	// FIXME: Make this do nothing instead
	return json.Unmarshal(b, action)
}

func (action *Gate) UnmarshalJSON(b []byte) error {
	// FIXME: Make this do nothing instead
	return json.Unmarshal(b, action)
}

func (action *Wrapper) UnmarshalJSON(b []byte) error {
	// FIXME: Make this do nothing instead
	return json.Unmarshal(b, action)
}

func (_ None) Run(ctx context.Context, mctx *common.MorphContext, allHosts map[string]nix.Host, cache_ *cache.LockedMap[string]) error {
	return nil
}

func (step Gate) Run(ctx context.Context, mctx *common.MorphContext, allHosts map[string]nix.Host, cache_ *cache.LockedMap[string]) error {
	return nil
}

func (step Wrapper) Run(ctx context.Context, mctx *common.MorphContext, allHosts map[string]nix.Host, cache_ *cache.LockedMap[string]) error {
	return nil
}
