package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/mattn/go-sqlite3" // SQLite driver
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// AgentDBAdapter provides a high-level API for interacting with the underlying SQLite database.
type AgentDBAdapter struct {
	db        *sql.DB
	inMemory  map[string]*MemoryEntry
	useMemory bool
	mu        sync.RWMutex
}

// NewAgentDBAdapter creates a new AgentDBAdapter, connects to the database, and ensures tables are set up.
func NewAgentDBAdapter(dbPath string) (*AgentDBAdapter, error) {
	if dbPath == "" {
		// Default path in local app data
		appdata := os.Getenv("LOCALAPPDATA")
		if appdata == "" {
			return nil, fmt.Errorf("LOCALAPPDATA environment variable not set")
		}
		dbPath = filepath.Join(appdata, "MultiSend", "agentdb.sqlite")
	}

	// Ensure the directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Connect to the database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Create table if it doesn't exist
	// Note: Embeddings are not stored here directly, just the content and metadata.
	// The vector index will handle the embeddings.
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS memory_entries (
		id TEXT PRIMARY KEY,
		content TEXT,
		metadata_json TEXT
	);`

	if _, err := db.Exec(createTableSQL); err != nil {
		// CGO-less environments return a sqlite3 stub error.
		// Fallback keeps behavior deterministic for tests and local tooling.
		if strings.Contains(err.Error(), "go-sqlite3 requires cgo") {
			_ = db.Close()
			return &AgentDBAdapter{
				inMemory:  make(map[string]*MemoryEntry),
				useMemory: true,
			}, nil
		}
		return nil, fmt.Errorf("failed to create memory_entries table: %w", err)
	}

	return &AgentDBAdapter{db: db}, nil
}

// Store saves a MemoryEntry to the database.
func (a *AgentDBAdapter) Store(entry *MemoryEntry) error {
	if entry == nil {
		return fmt.Errorf("entry is nil")
	}
	if strings.TrimSpace(entry.ID) == "" {
		return fmt.Errorf("entry id is required")
	}
	if a.useMemory {
		a.mu.Lock()
		defer a.mu.Unlock()
		cp := *entry
		if cp.Metadata == nil {
			cp.Metadata = map[string]interface{}{}
		}
		a.inMemory[cp.ID] = &cp
		return nil
	}
	meta := entry.Metadata
	if meta == nil {
		meta = map[string]interface{}{}
	}
	b, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	_, err = a.db.Exec(`
	INSERT INTO memory_entries (id, content, metadata_json)
	VALUES (?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
	  content = excluded.content,
	  metadata_json = excluded.metadata_json
	`, entry.ID, entry.Content, string(b))
	if err != nil {
		return fmt.Errorf("failed to store memory entry: %w", err)
	}
	return nil
}

// Query retrieves MemoryEntry items from the database based on filters.
func (a *AgentDBAdapter) Query(query *MemoryQuery) ([]*MemoryEntry, error) {
	if query == nil {
		query = &MemoryQuery{}
	}
	limit := query.Limit
	if limit <= 0 {
		limit = 10
	}
	if a.useMemory {
		a.mu.RLock()
		defer a.mu.RUnlock()
		out := make([]*MemoryEntry, 0, limit)
		ids := make([]string, 0, len(a.inMemory))
		for id := range a.inMemory {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			e := a.inMemory[id]
			if matchesFilters(e, query.Filters) {
				cp := *e
				out = append(out, &cp)
				if len(out) >= limit {
					break
				}
			}
		}
		return out, nil
	}
	rows, err := a.db.Query(`
	SELECT id, content, metadata_json
	FROM memory_entries
	ORDER BY id ASC
	LIMIT ?
	`, limit*10)
	if err != nil {
		return nil, fmt.Errorf("failed to query memory entries: %w", err)
	}
	defer rows.Close()

	out := make([]*MemoryEntry, 0, limit)
	for rows.Next() {
		var id, content, metadataJSON string
		if err := rows.Scan(&id, &content, &metadataJSON); err != nil {
			return nil, fmt.Errorf("failed to scan memory entry: %w", err)
		}
		meta := map[string]interface{}{}
		if strings.TrimSpace(metadataJSON) != "" {
			if err := json.Unmarshal([]byte(metadataJSON), &meta); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}
		e := &MemoryEntry{
			ID:       id,
			Content:  content,
			Metadata: meta,
		}
		if matchesFilters(e, query.Filters) {
			out = append(out, e)
			if len(out) >= limit {
				break
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func matchesFilters(entry *MemoryEntry, filters map[string]interface{}) bool {
	if len(filters) == 0 {
		return true
	}
	for k, want := range filters {
		got, ok := entry.Metadata[k]
		if !ok {
			return false
		}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			return false
		}
	}
	return true
}

// Close closes the database connection.
func (a *AgentDBAdapter) Close() error {
	if a.useMemory {
		return nil
	}
	if a.db != nil {
		return a.db.Close()
	}
	return nil
}
