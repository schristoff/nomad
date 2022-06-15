package checkstore

import (
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/serviceregistration/checks"
	"github.com/hashicorp/nomad/client/state"
	"github.com/hashicorp/nomad/helper"
	"golang.org/x/exp/slices"
)

// Shim is used to record the latest check status information, one layer above
// the client persistent store so we can do efficient indexing, etc.
type Shim interface {
	// Set the latest result for a specific check.
	Set(
		allocID string,
		checkID checks.ID,
		result *checks.QueryResult,
	) error

	// List the latest results for a specific allocation.
	List(allocID string) map[checks.ID]*checks.QueryResult

	// Keep will reconcile the current set of stored check results with the
	// list of checkIDs for check results that should be kept.
	Keep(allocID string, checkIDs []checks.ID) error

	// Purge results for a specific allocation.
	Purge(allocID string) error
}

// AllocResultMap is a view of the check_id -> latest result for group and task
// checks in an allocation.
type AllocResultMap map[checks.ID]*checks.QueryResult

// ClientResultMap is a holistic view of alloc_id -> check_id -> latest result
// group and task checks across all allocations on a client.
type ClientResultMap map[string]AllocResultMap

type store struct {
	log hclog.Logger

	db state.StateDB

	lock    sync.RWMutex
	current ClientResultMap
}

// NewStore creates a new store.
//
// (todo: and will initialize from db)
func NewStore(log hclog.Logger, db state.StateDB) Shim {
	return &store{
		log:     log.Named("check_store"),
		db:      db,
		current: make(ClientResultMap),
	}
}

func (s *store) restore() {
	// todo restore state from db
}

func (s *store) Set(allocID string, checkID checks.ID, qr *checks.QueryResult) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.log.Trace("setting check status", "alloc_id", allocID, "check_id", checkID, "result", qr.Result)

	if _, exists := s.current[allocID]; !exists {
		s.current[allocID] = make(map[checks.ID]*checks.QueryResult)
	}

	s.current[allocID][checkID] = qr

	return s.db.PutCheckResult(allocID, qr)
}

func (s *store) List(allocID string) map[checks.ID]*checks.QueryResult {
	s.lock.RLock()
	defer s.lock.RUnlock()

	m, exists := s.current[allocID]
	if !exists {
		return nil
	}

	return helper.CopyMap(m)
}

func (s *store) Purge(allocID string) error {
	s.lock.RLock()
	defer s.lock.RUnlock()

	// remove from our map
	delete(s.current, allocID)

	// remove from persistent store
	return s.db.PurgeCheckResults(allocID)
}

func (s *store) Keep(allocID string, checkIDs []checks.ID) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	// remove from our map and record which ids to remove from persistent store
	var remove []checks.ID
	for id := range s.current[allocID] {
		if !slices.Contains(checkIDs, id) {
			delete(s.current[allocID], id)
			remove = append(remove, id)
		}
	}

	// remove from persistent store
	return s.db.DeleteCheckResults(allocID, remove)
}
