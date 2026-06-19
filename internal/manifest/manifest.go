package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const (
	StatusPending  = "pending"
	StatusSending  = "sending"
	StatusDone     = "done"
	StatusFailed   = "failed"
	StatusCanceled = "canceled"
)

type SourceInfo struct {
	Type     string `json:"type,omitempty"`
	Address  string `json:"address,omitempty"`
	URL      string `json:"url,omitempty"`
	Identity string `json:"identity,omitempty"`
	AbsPath  string `json:"abs_path,omitempty"`
	ModTime  int64  `json:"mod_time_unix,omitempty"`
}

type TargetInfo struct {
	Type    string `json:"type,omitempty"`
	Address string `json:"address,omitempty"`
	Path    string `json:"path,omitempty"`
}

type ChunkState struct {
	Index       int64      `json:"index"`
	Offset      int64      `json:"offset"`
	Size        int64      `json:"size"`
	Status      string     `json:"status"`
	Attempts    int        `json:"attempts"`
	LastError   string     `json:"last_error,omitempty"`
	Channel     string     `json:"channel,omitempty"`
	BytesDone   int64      `json:"bytes_done"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	HashSHA256  string     `json:"hash_sha256,omitempty"`
}

type Manifest struct {
	SchemaVersion int          `json:"schema_version"`
	TransferID    string       `json:"transfer_id"`
	Type          string       `json:"type"`
	FileName      string       `json:"file_name"`
	TotalBytes    int64        `json:"total_bytes"`
	ChunkSize     int64        `json:"chunk_size"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
	Source        SourceInfo   `json:"source"`
	Target        TargetInfo   `json:"target"`
	Chunks        []ChunkState `json:"chunks"`
}

func Load(path string) (*Manifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	m.normalizeForResume(3)
	return &m, nil
}

func SaveAtomic(path string, m *Manifest) error {
	if m == nil {
		return fmt.Errorf("manifest is nil")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	m.UpdatedAt = time.Now().UTC()
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, werr := f.Write(b)
	serr := f.Sync()
	cerr := f.Close()
	if werr != nil {
		return werr
	}
	if serr != nil {
		return serr
	}
	if cerr != nil {
		return cerr
	}
	return os.Rename(tmp, path)
}

func (m *Manifest) MarkAndGetSending(index int64, channel string) *ChunkState {
	for i := range m.Chunks {
		if m.Chunks[i].Index == index {
			now := time.Now().UTC()
			m.Chunks[i].Status = StatusSending
			m.Chunks[i].Channel = channel
			m.Chunks[i].StartedAt = &now
			m.Chunks[i].CompletedAt = nil
			m.Chunks[i].LastError = ""
			c := m.Chunks[i]
			return &c
		}
	}
	return nil
}

func (m *Manifest) GetChunk(index int64) *ChunkState {
	for i := range m.Chunks {
		if m.Chunks[i].Index == index {
			return &m.Chunks[i]
		}
	}
	return nil
}

func (m *Manifest) MarkSending(index int64, channel string) {
	_ = m.MarkAndGetSending(index, channel)
}

func (m *Manifest) MarkDone(index int64, bytes int64, hash string) {
	if c := m.chunk(index); c != nil {
		now := time.Now().UTC()
		c.Status = StatusDone
		c.BytesDone = bytes
		c.HashSHA256 = hash
		c.CompletedAt = &now
		if c.StartedAt == nil {
			c.StartedAt = &now
		}
	}
}

func (m *Manifest) MarkFailed(index int64, err string) {
	if c := m.chunk(index); c != nil {
		c.Status = StatusFailed
		c.Attempts++
		c.LastError = err
	}
}

func (m *Manifest) PendingChunks() []ChunkState {
	out := make([]ChunkState, 0)
	for _, c := range m.Chunks {
		if c.Status == StatusPending || c.Status == StatusFailed {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Index < out[j].Index })
	return out
}

func (m *Manifest) DoneBytes() int64 {
	var done int64
	for _, c := range m.Chunks {
		if c.Status == StatusDone {
			done += c.BytesDone
		}
	}
	return done
}

func (m *Manifest) Percent() float64 {
	if m.TotalBytes <= 0 {
		return 0
	}
	return (float64(m.DoneBytes()) * 100) / float64(m.TotalBytes)
}

func (m *Manifest) normalizeForResume(maxAttempts int) {
	for i := range m.Chunks {
		c := &m.Chunks[i]
		switch c.Status {
		case StatusSending:
			c.Status = StatusPending
		case StatusFailed:
			if c.Attempts < maxAttempts {
				c.Status = StatusPending
			}
		}
	}
}

func (m *Manifest) HasCanceledChunks() bool {
	for _, c := range m.Chunks {
		if c.Status == StatusCanceled {
			return true
		}
	}
	return false
}

func (m *Manifest) ActivateResume(maxAttempts int) {
	for i := range m.Chunks {
		c := &m.Chunks[i]
		switch c.Status {
		case StatusCanceled, StatusSending, StatusPending:
			c.Status = StatusPending
		case StatusFailed:
			c.Status = StatusPending
			if c.Attempts >= maxAttempts {
				c.Attempts = 0
			}
		}
	}
}

func (m *Manifest) MarkCanceledPending() {
	for i := range m.Chunks {
		c := &m.Chunks[i]
		if c.Status == StatusPending || c.Status == StatusSending {
			c.Status = StatusCanceled
		}
	}
}

func (m *Manifest) chunk(index int64) *ChunkState {
	for i := range m.Chunks {
		if m.Chunks[i].Index == index {
			return &m.Chunks[i]
		}
	}
	return nil
}
