package checkstore

import (
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/serviceregistration/checks"
	"github.com/hashicorp/nomad/client/state"
	"github.com/hashicorp/nomad/helper"
)

// Store is used to record the latest check status information.
type Store interface {
	// Set the latest result for a specific check.
	Set(
		allocID string,
		checkID checks.ID,
		result *checks.QueryResult,
	) error

	// List the latest results for a specific allocation.
	List(allocID string) map[checks.ID]*checks.QueryResult
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
func NewStore(log hclog.Logger, db state.StateDB) Store {
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

	// todo store in batches maybe
	return s.db.PutCheckStatus(allocID, qr)
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
