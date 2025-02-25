package steps

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/DBCDK/morph/cache"
	"github.com/DBCDK/morph/common"
	"github.com/DBCDK/morph/nix"
)

// FIXME: send this to some common thing
type Request struct {
	Description string            `json:"description"`
	Headers     map[string]string `json:"headers"`
	Host        *string           `json:"host"`
	InsecureSSL bool              `json:"insecureSSL"`
	Path        string            `json:"path"`
	Port        int               `json:"port"`
	Scheme      string            `json:"scheme"`
}

type LocalRequest struct {
	Request Request `json:"request"`
	Timeout int     `json:"timeout"`
}

type RemoteRequest struct {
	Request Request `json:"request"`
	Timeout int     `json:"timeout"`
}

func (_ LocalRequest) Name() string  { return "local-request" }
func (_ RemoteRequest) Name() string { return "remote-request" }

func (action *LocalRequest) MarshalJSONx(step Step) ([]byte, error) {
	return json.Marshal(struct {
		StepAlias
		LocalRequest
	}{
		StepAlias:    StepAlias(step),
		LocalRequest: *action,
	})
}

func (action *RemoteRequest) MarshalJSONx(step Step) ([]byte, error) {
	return json.Marshal(struct {
		StepAlias
		RemoteRequest
	}{
		StepAlias:     StepAlias(step),
		RemoteRequest: *action,
	})
}

func (action *LocalRequest) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, action)
}

func (action *RemoteRequest) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, action)
}

func (action LocalRequest) Run(ctx context.Context, mctx *common.MorphContext, allHosts map[string]nix.Host, cache_ *cache.LockedMap[string]) error {
	return errors.New("not implemented: " + action.Name())
}

func (action RemoteRequest) Run(ctx context.Context, mctx *common.MorphContext, allHosts map[string]nix.Host, cache_ *cache.LockedMap[string]) error {
	return errors.New("not implemented: " + action.Name())
}
