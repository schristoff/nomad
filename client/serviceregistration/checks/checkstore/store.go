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
		allocID checks.AllocID,
		checkID checks.CheckID,
		result *checks.QueryResult,
	) error

	// List the latest results for a specific allocation.
	List(allocID checks.AllocID) map[checks.CheckID]*checks.QueryResult
}

type StatusMap map[checks.AllocID]map[checks.CheckID]*checks.QueryResult

type store struct {
	log hclog.Logger

	db state.StateDB

	lock    sync.RWMutex
	current StatusMap
}

func NewStore(log hclog.Logger, db state.StateDB) Store {
	return &store{
		log: log.Named("check_store"),
		db:  db,
	}
}

func (s *store) restore() {
	// todo restore state from db
}

func (s *store) Set(allocID checks.AllocID, checkID checks.CheckID, qr *checks.QueryResult) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.log.Trace("setting check status", "alloc_id", allocID, "check_id", checkID, "result", qr.Result)

	if _, exists := s.current[allocID]; !exists {
		s.current[allocID] = make(map[checks.CheckID]*checks.QueryResult)
	}

	s.current[allocID][checkID] = qr

	return s.db.PutCheckStatus(allocID, qr)
}

func (s *store) List(allocID checks.AllocID) map[checks.CheckID]*checks.QueryResult {
	s.lock.RLock()
	defer s.lock.RUnlock()

	m, exists := s.current[allocID]
	if !exists {
		return nil
	}

	return helper.CopyMap(m)
}
