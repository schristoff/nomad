package allocrunner

import (
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
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

// checksHook manages checks of Nomad service registrations, at both the group and
// task level, by storing / removing them from the Client state store.
type checksHook struct {
	logger  hclog.Logger
	allocID string
	store   checkstore.Store

	lock   sync.RWMutex
	checks map[checks.ID]*structs.ServiceCheck
}

func newChecksHook(
	logger hclog.Logger,
	alloc *structs.Allocation,
	store checkstore.Store,
) *checksHook {
	h := &checksHook{
		logger:  logger.Named(checksHookName),
		allocID: alloc.ID,
		store:   store,
	}
	h.checks = h.findChecks(alloc)
	return h
}

func (h *checksHook) findChecks(alloc *structs.Allocation) map[checks.ID]*structs.ServiceCheck {
	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	if tg == nil {
		h.logger.Error("failed to find group", "alloc_id", alloc.ID, "group", alloc.TaskGroup)
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

func (h *checksHook) current() map[checks.ID]*structs.ServiceCheck {
	h.lock.RLock()
	defer h.lock.RUnlock()
	return helper.CopyMap(h.checks)
}

func (h *checksHook) Prerun() error {
	now := time.Now().UTC().Unix()
	netlog.Yellow("checkHook PreRun, now: %v", now)

	current := h.current()

	// insert a pending result into state store for each check
	for id, check := range current {
		result := checks.Stub(id, checks.GetKind(check), now)
		netlog.Yellow("set id: %s", id)
		if err := h.store.Set(h.allocID, id, result); err != nil {
			return err
		}
	}

	// startup the check goroutine

	return nil
}

func (h *checksHook) PreKill() {
	netlog.Yellow("checksHook PreKill")

	// stop the check goroutine

	// purge checks from state store

	current := h.current()
	for id := range current {
		netlog.Yellow("remove id: %s", id)
	}

}
