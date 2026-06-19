package chunk

import "fmt"

const (
	MiB int64 = 1024 * 1024
)

type Chunk struct {
	Index  int64 `json:"index"`
	Offset int64 `json:"offset"`
	Size   int64 `json:"size"`
	Start  int64 `json:"start"`
	End    int64 `json:"end"`
}

type Plan struct {
	TotalBytes int64   `json:"total_bytes"`
	ChunkSize  int64   `json:"chunk_size"`
	Chunks     []Chunk `json:"chunks"`
}

func AutoChunkSize(totalBytes int64) int64 {
	// Com o Pipelining ativo (InFlight >= 2), o overhead de requests HTTP cai a zero.
	// Reduzimos o tamanho dos blocos (Micro-chunking) para garantir uma distribuição
	// de carga assimétrica perfeita entre adaptadores de velocidades diferentes (ex: 57Mbps vs 20Mbps),
	// eliminando o travamento do download em 99%.
	switch {
	case totalBytes < 100*MiB:
		return 2 * MiB // Arquivos pequenos: blocos de 2MB
	case totalBytes <= 1024*MiB:
		return 4 * MiB // Até 1GB: blocos de 4MB
	case totalBytes <= 4*1024*MiB:
		return 8 * MiB // Até 4GB: blocos de 8MB
	default:
		return 16 * MiB // Arquivos massivos: teto de 16MB por bloco
	}
}

func ChunkCount(totalBytes int64, chunkSize int64) int64 {
	if totalBytes <= 0 || chunkSize <= 0 {
		return 0
	}
	count := totalBytes / chunkSize
	if totalBytes%chunkSize != 0 {
		count++
	}
	return count
}

func BuildPlan(totalBytes int64, chunkSize int64) (Plan, error) {
	if totalBytes < 0 {
		return Plan{}, fmt.Errorf("invalid total bytes")
	}
	if chunkSize <= 0 {
		return Plan{}, fmt.Errorf("invalid chunk size")
	}
	plan := Plan{TotalBytes: totalBytes, ChunkSize: chunkSize}
	chunks := make([]Chunk, 0, ChunkCount(totalBytes, chunkSize))
	for i, offset := int64(0), int64(0); offset < totalBytes; i, offset = i+1, offset+chunkSize {
		size := chunkSize
		if remain := totalBytes - offset; remain < size {
			size = remain
		}
		chunks = append(chunks, Chunk{Index: i, Offset: offset, Size: size, Start: offset, End: offset + size - 1})
	}
	plan.Chunks = chunks
	return plan, nil
}
