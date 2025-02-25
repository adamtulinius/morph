package common

import (
	"github.com/DBCDK/morph/nix"
	"github.com/DBCDK/morph/ssh"
	"github.com/DBCDK/morph/utils"
	"github.com/rs/zerolog/log"
)

type MorphContext struct {
	SSHContext *ssh.SSHContext
	NixContext *nix.NixContext

	// Globals we need to get rid off
	AssetRoot           string
	AttrKey             string
	Deployment          string
	DeploySwitchAction  string
	DeployReboot        bool
	DeployUploadSecrets bool
	DryRun              bool
	ExecuteCommand      []string
	NixBuildArg         []string
	NixBuildTarget      string
	NixBuildTargetFile  string
	OrderingTags        string
	SelectEvery         int
	SelectGlob          string
	SelectLimit         int
	SelectSkip          int
	SelectTags          string
	SkipHealthChecks    bool
	SkipPreDeployChecks bool
	Timeout             int
}

func HandleError(err error) {
	//Stupid handling of catch-all errors for now
	if err != nil {
		log.Fatal().Err(err).Msg("Fatal error, shutting down")
		utils.Exit(1)
	}
}
