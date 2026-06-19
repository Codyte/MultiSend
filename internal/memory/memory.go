package memory

import "fmt"

// IMemoryBackend defines the interface for a memory system.
// It abstracts the storage and retrieval of memory entries.
type IMemoryBackend interface {
	Store(entry *MemoryEntry) error
	Query(query *MemoryQuery) ([]*MemoryEntry, error)
}

// UnifiedMemoryService is the central implementation of IMemoryBackend, orchestrating
// between the database adapter and the vector indexer.
type UnifiedMemoryService struct {
	agentdb *AgentDBAdapter
	indexer *HNSWIndexer
}

// NewUnifiedMemoryService creates a new instance of the UnifiedMemoryService.
func NewUnifiedMemoryService(agentdb *AgentDBAdapter, indexer *HNSWIndexer) *UnifiedMemoryService {
	return &UnifiedMemoryService{
		agentdb: agentdb,
		indexer: indexer,
	}
}

// Store saves a memory entry to the database and updates the search index.
func (s *UnifiedMemoryService) Store(entry *MemoryEntry) error {
	if entry == nil {
		return fmt.Errorf("entry is nil")
	}
	if err := s.agentdb.Store(entry); err != nil {
		return err
	}
	s.indexer.Index(entry)
	return nil
}

// Query retrieves memory entries based on the query, using the indexer for
// semantic searches and the database for other queries.
func (s *UnifiedMemoryService) Query(query *MemoryQuery) ([]*MemoryEntry, error) {
	if query != nil && query.Semantic {
		return s.indexer.Search(query), nil
	}
	return s.agentdb.Query(query)
}

// MemoryEntry represents a single piece of memory, including its content and metadata.
type MemoryEntry struct {
	ID        string
	Content   string
	Embedding []float32
	Metadata  map[string]interface{}
}

// MemoryQuery represents a query for retrieving memory entries.
type MemoryQuery struct {
	Content  string
	Filters  map[string]interface{}
	Semantic bool
	Limit    int
}
