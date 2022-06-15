package allocrunner

import (
	"context"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/serviceregistration/checks"
	"github.com/hashicorp/nomad/client/serviceregistration/checks/checkstore"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
	"gophers.dev/pkgs/netlog"
)

const (
	// checksHookName is the name of this hook as appears in logs
	checksHookName = "checks_hook"
)

// observers maintains a map from check_id -> observer for that check. Each
// observer in the map should be tied to the same context.
type observers map[checks.ID]*observer

// An observer is used to execute checks on their interval and update the check
// store with those results.
type observer struct {
	ctx     context.Context
	check   *structs.ServiceCheck
	shim    checkstore.Shim
	checker checks.Checker
	allocID string
	checkID checks.ID
}

func (o *observer) start() {
	timer, cancel := helper.NewSafeTimer(0)
	defer cancel()

	netlog.Cyan("observer started for check: %s", o.check.Name)

	for {
		select {
		case <-o.ctx.Done():
			netlog.Cyan("observer exit, check: %s", o.check.Name)
			return
		case <-timer.C:
			// do check
			result := o.checker.Check(checks.GetQuery(o.check))
			netlog.Cyan("observer result: %s ...", result)
			netlog.Cyan("%s", result.Output)

			// and put the results into the store
			result.ID = o.checkID
			_ = o.shim.Set(o.allocID, result)

			timer.Reset(o.check.Interval)
		}
	}
}

// checksHook manages checks of Nomad service registrations, at both the group and
// task level, by storing / removing them from the Client state store.
type checksHook struct {
	logger  hclog.Logger
	allocID string
	shim    checkstore.Shim
	checker checks.Checker

	// ctx is the context of the current set of checks. on an allocation update
	// everything is replaced - the checks, observers, ctx, etc.
	ctx  context.Context
	stop func()

	lock      sync.RWMutex
	observers map[checks.ID]*observer
}

func newChecksHook(
	logger hclog.Logger,
	alloc *structs.Allocation,
	shim checkstore.Shim,
) *checksHook {
	h := &checksHook{
		logger:  logger.Named(checksHookName),
		allocID: alloc.ID,
		shim:    shim,
		checker: checks.New(logger),
	}
	h.ctx, h.stop = context.WithCancel(context.Background())
	h.observers = h.observersFor(findChecks(alloc))
	return h
}

func (h *checksHook) observersFor(m map[checks.ID]*structs.ServiceCheck) observers {
	obs := make(map[checks.ID]*observer, len(m))
	for id, check := range m {
		obs[id] = &observer{
			ctx:     h.ctx,
			check:   check,
			shim:    h.shim,
			checker: h.checker,
			allocID: h.allocID,
			checkID: id,
		}
	}
	return obs
}

func findChecks(alloc *structs.Allocation) map[checks.ID]*structs.ServiceCheck {
	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	if tg == nil {
		return nil
	}

	result := make(map[checks.ID]*structs.ServiceCheck)

	// gather up checks of group services
	for _, service := range tg.Services {
		for _, check := range service.Checks {
			id := checks.MakeID(alloc.ID, alloc.TaskGroup, "group", check.Name)
			result[id] = check.Copy()
		}
	}

	// gather up checks of task services
	for _, task := range tg.Tasks {
		for _, service := range task.Services {
			for _, check := range service.Checks {
				id := checks.MakeID(alloc.ID, alloc.TaskGroup, task.Name, check.Name)
				result[id] = check.Copy()
			}
		}
	}

	return result
}

func (h *checksHook) Name() string {
	return checksHookName
}

func (h *checksHook) getChecks() map[checks.ID]*structs.ServiceCheck {
	h.lock.RLock()
	defer h.lock.RUnlock()

	m := make(map[checks.ID]*structs.ServiceCheck, len(h.observers))
	for id, obs := range h.observers {
		m[id] = obs.check
	}
	return m
}

func (h *checksHook) Prerun() error {
	now := time.Now().UTC().Unix()
	netlog.Yellow("ch.PreRun, now: %v", now)

	current := h.getChecks()

	// insert a pending result into state store for each check
	for id, check := range current {
		result := checks.Stub(id, checks.GetKind(check), now)
		if err := h.shim.Set(h.allocID, result); err != nil {
			return err
		}
	}

	// start the observers
	for _, obs := range h.observers {
		go obs.start()
	}

	return nil
}

func (h *checksHook) Update(request *interfaces.RunnerUpdateRequest) error {
	netlog.Yellow("checksHook.Update, id: %s", request.Alloc.ID)

	netlog.Yellow("ch.Update: issue stop")

	// todo: need to reconcile check store, may be checks to remove

	return nil
}

func (h *checksHook) PreKill() {
	netlog.Yellow("ch.PreKill")

	// terminate the background thing
	netlog.Yellow("ch.PreKill: issue stop")
	h.stop()

	if err := h.shim.Purge(h.allocID); err != nil {
		h.logger.Error("failed to purge check results", "alloc_id", h.allocID, "error", err)
	}
}
