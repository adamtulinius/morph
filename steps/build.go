package steps

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/DBCDK/morph/cache"
	"github.com/DBCDK/morph/common"
	"github.com/DBCDK/morph/cruft"
	"github.com/DBCDK/morph/nix"
	"github.com/rs/zerolog/log"
	"path"
	"path/filepath"
)

type Build struct {
	Hosts []string `json:"hosts"`
}

func (build Build) Name() string {
	return "build"
}

func (build *Build) MarshalJSONx(step Step) ([]byte, error) {
	return json.Marshal(struct {
		StepAlias
		Build
	}{
		StepAlias: StepAlias(step),
		Build:     *build,
	})
}

func (build Build) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, &build)
}

func filterHosts(needles []string, allHosts map[string]nix.Host) ([]nix.Host, error) {
	result := make([]nix.Host, 0)

	for _, hostByName := range needles {
		host, ok := allHosts[hostByName]
		if ok {
			result = append(result, host)
		} else {
			return nil, errors.New(fmt.Sprintf("host %s not in deployment", hostByName))
		}
	}

	return result, nil
}

func (build *Build) Run(ctx context.Context, mctx *common.MorphContext, allHosts map[string]nix.Host, cache_ *cache.LockedMap[string]) error {
	hosts, err := filterHosts(build.Hosts, allHosts)
	if err != nil {
		return err
	}

	// FIXME: Build errors does not bubble up correctly (try setting `services.haproxy.enable = true;`, it'll cause the build to fall and morph to hang

	resultPath, err := cruft.ExecBuild(mctx, hosts)
	if err != nil {
		return err
	}

	log.Info().Msg(resultPath)

	for _, host := range hosts {
		hostPathSymlink := path.Join(resultPath, host.Name)
		hostPath, err := filepath.EvalSymlinks(hostPathSymlink)
		if err != nil {
			return err
		}

		cache_.Update("closure:"+host.Name, hostPath)
	}

	return err
}
