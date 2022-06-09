package checks

import (
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/state"
)

type (
	AllocID string
	CheckID string
)

type Store interface {
	Set(
		allocID AllocID,
		checkID CheckID,
		result *QueryResult,
	)

	// used to populate heartbeat with minimal data

	// used to answer /client/checks endpoint
	// - by alloc id
}

type store struct {
	log hclog.Logger

	db state.StateDB

	lock    sync.RWMutex
	current map[AllocID]map[CheckID]*QueryResult

	// group checks
	// task checks
	//
	// or
	//
	// id -> all

}

func NewStore(log hclog.Logger, db state.StateDB) Store {
	return &store{
		log: log.Named("store"),
		db:  db,
	}
}

func (s *store) restore() {
	// todo restore state from db
}

func (s *store) Set(allocID AllocID, checkID CheckID, qr *QueryResult) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if _, exists := s.current[allocID]; !exists {
		s.current[allocID] = make(map[CheckID]*QueryResult)
	}

	s.current[allocID][checkID] = qr

	// todo update db
}

func (s *store) List(allocID AllocID) map[CheckID]*QueryResult {
	s.lock.RLock()
	defer s.lock.RUnlock()

	m, exists := s.current[allocID]
	if !exists {
		return nil
	}

	// copy m
}
