package memory

import (
	"fmt"
	"strings"
)

// ChunkText splits text into overlapping word-count chunks for RAG ingestion.
// chunkSize controls words per chunk, overlap controls word overlap between chunks.
// source is used as the display label; each chunk is named "source[i]".
// Returned chunks have Content and Name populated; Vector is nil (caller embeds).
func ChunkText(text string, chunkSize int, overlap int, source string) []Chunk {
	if chunkSize <= 0 {
		chunkSize = 400
	}
	if overlap < 0 {
		overlap = 0
	}
	if overlap >= chunkSize {
		overlap = chunkSize / 2
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	var chunks []Chunk
	step := chunkSize - overlap
	if step <= 0 {
		step = 1
	}

	for i := 0; i < len(words); i += step {
		end := i + chunkSize
		if end > len(words) {
			end = len(words)
		}
		content := strings.Join(words[i:end], " ")
		chunks = append(chunks, Chunk{
			Name:    fmt.Sprintf("%s[%d]", source, len(chunks)),
			Content: content,
		})
		if end == len(words) {
			break
		}
	}

	return chunks
}
