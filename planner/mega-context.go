package planner

import (
	"context"
	"github.com/DBCDK/morph/cache"
	"github.com/DBCDK/morph/common"
	"github.com/DBCDK/morph/events"
	"github.com/DBCDK/morph/nix"
	"github.com/DBCDK/morph/ssh"
	"github.com/DBCDK/morph/steps"
	"sync"
)

// FIXME: This entire thing needs to die

type MegaContext struct {
	Hosts        map[string]nix.Host
	MorphContext *common.MorphContext
	SSHContext   *ssh.SSHContext
	NixContext   *nix.NixContext
	Cache        *cache.LockedMap[string]
	StepStatus   *cache.LockedMap[string]
	Steps        *cache.LockedMap[steps.Step]
	Constraints  []nix.Constraint
	EventManager *events.Manager

	context     context.Context
	tickChan    chan bool
	queueLock   sync.RWMutex
	queuedSteps []steps.Step // steps awaiting processing
	retryCounts *cache.LockedMap[int]
}

func NewMegaContext(eventMgr *events.Manager, hosts map[string]nix.Host, morphContext *common.MorphContext, constraints []nix.Constraint) *MegaContext {
	return &MegaContext{
		Hosts:        hosts,
		MorphContext: morphContext,
		SSHContext:   morphContext.SSHContext,
		NixContext:   morphContext.NixContext,

		Constraints: constraints,

		EventManager: eventMgr,

		Cache:       cache.NewLockedMap[string]("cache"),
		StepStatus:  cache.NewLockedMap[string]("steps-done"),
		Steps:       cache.NewLockedMap[steps.Step]("steps"),
		retryCounts: cache.NewLockedMap[int]("retries"),
	}
}
