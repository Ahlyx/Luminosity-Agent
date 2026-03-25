package builtin

import (
	"fmt"

	"github.com/ahlyx/luminosity-agent/internal/memory"
)

// SaveMemoryTool ingests arbitrary content into the vector store with deduplication.
// Chunks are embedded immediately (InjectChunks) and persisted to disk (PersistChunk)
// so they survive restarts.
type SaveMemoryTool struct {
	VS          *memory.VectorStore
	Embed       func(string) ([]float32, error)
	IngestedDir string // e.g. ~/.luminosity/memory/ingested/
}

func (t SaveMemoryTool) Name() string { return "save_memory" }

func (t SaveMemoryTool) Description() string {
	return "Ingest text content into vector memory for future retrieval. Provide a descriptive source label and the text to remember."
}

func (t SaveMemoryTool) Schema() string {
	return `{"path": "source label", "content": "text to ingest"}`
}

const (
	defaultChunkSize = 400
	defaultOverlap   = 50
	dupThreshold     = float32(0.87)
)

func (t SaveMemoryTool) Execute(params map[string]string) (string, error) {
	source := params["path"]
	if source == "" {
		source = "ingested"
	}
	content := params["content"]
	if content == "" {
		return "", fmt.Errorf("content is required")
	}

	chunks := memory.ChunkText(content, defaultChunkSize, defaultOverlap, source)
	if len(chunks) == 0 {
		return "No content to store.", nil
	}

	stored := 0
	skipped := 0

	for i := range chunks {
		vec, err := t.Embed(chunks[i].Content)
		if err != nil {
			skipped++
			continue
		}
		if t.VS.IsDuplicate(vec, dupThreshold) {
			skipped++
			continue
		}
		chunks[i].Vector = vec
		t.VS.InjectChunks([]memory.Chunk{chunks[i]})
		if err := t.VS.PersistChunk(chunks[i], t.IngestedDir); err != nil {
			// Non-fatal: chunk is live in memory, just won't survive restart
			_ = err
		}
		stored++
	}

	return fmt.Sprintf("Stored %d chunks from '%s', skipped %d duplicates.", stored, source, skipped), nil
}
