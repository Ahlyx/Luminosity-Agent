package memory
 
import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)
 
// EmbedFunc is the function signature for embedding text to a vector.
// Matches LMStudioClient.Embed so it can be passed in without an import cycle.
type EmbedFunc func(text string) ([]float32, error)
 
// Chunk represents a single memory file with its embedding.
type Chunk struct {
	// Path is the full path to the source markdown file.
	Path string
	// Name is the relative path from the memory root — used as display label.
	Name string
	// Content is the full text content of the file.
	Content string
	// Vector is the semantic embedding of Content.
	Vector []float32
	// ModTime is the file modification time at last embed — used to detect stale vectors.
	ModTime time.Time
}
 
// VectorStore holds embedded memory chunks and provides similarity search.
type VectorStore struct {
	mu     sync.RWMutex
	chunks []Chunk
	root   string
	embed  EmbedFunc
}
 
// NewVectorStore creates a VectorStore rooted at dir.
// dir is the base memory directory, e.g. ~/.luminosity/memory/
// embedFn is called to produce vectors — pass lm.Embed directly.
func NewVectorStore(dir string, embedFn EmbedFunc) *VectorStore {
	return &VectorStore{
		root:  dir,
		embed: embedFn,
	}
}
 
// Load scans the memory directory, reads all markdown files, and embeds
// any that are new or have been modified since last load.
// Safe to call multiple times — stale chunks are re-embedded, fresh ones are kept.
func (vs *VectorStore) Load() error {
	vs.mu.Lock()
	defer vs.mu.Unlock()
 
	// Build a map of existing chunks by path for stale detection
	existing := make(map[string]Chunk, len(vs.chunks))
	for _, c := range vs.chunks {
		existing[c.Path] = c
	}
 
	var updated []Chunk
 
	err := filepath.WalkDir(vs.root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			return nil
		}
 
		// Only process markdown files
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".md" && ext != ".txt" {
			return nil
		}
 
		info, err := d.Info()
		if err != nil {
			return nil
		}
 
		// Check if we have a fresh cached chunk
		if cached, ok := existing[path]; ok {
			if !info.ModTime().After(cached.ModTime) {
				// File unchanged — keep existing chunk
				updated = append(updated, cached)
				return nil
			}
		}
 
		// Read file content
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		text := strings.TrimSpace(string(content))
		if text == "" {
			return nil
		}
 
		// Embed the content
		vec, err := vs.embed(text)
		if err != nil {
			// Skip files that fail to embed — don't abort the whole load
			return nil
		}
 
		// Compute relative name for display
		rel, err := filepath.Rel(vs.root, path)
		if err != nil {
			rel = filepath.Base(path)
		}
 
		updated = append(updated, Chunk{
			Path:    path,
			Name:    rel,
			Content: text,
			Vector:  vec,
			ModTime: info.ModTime(),
		})
 
		return nil
	})
 
	if err != nil {
		return err
	}
 
	vs.chunks = updated
	return nil
}
 
// Search returns the top k chunks most semantically similar to query.
// If k <= 0 it defaults to 3.
// The caller should embed the query text before calling — pass the result as queryVec.
func (vs *VectorStore) Search(queryVec []float32, k int) []Chunk {
	if k <= 0 {
		k = 3
	}
 
	vs.mu.RLock()
	defer vs.mu.RUnlock()
 
	if len(vs.chunks) == 0 {
		return nil
	}
 
	type scored struct {
		chunk Chunk
		score float32
	}
 
	scores := make([]scored, 0, len(vs.chunks))
	for _, c := range vs.chunks {
		if len(c.Vector) == 0 {
			continue
		}
		sim := cosineSimilarity(queryVec, c.Vector)
		scores = append(scores, scored{chunk: c, score: sim})
	}
 
	// Sort descending by score — insertion sort is fine for small N
	for i := 1; i < len(scores); i++ {
		for j := i; j > 0 && scores[j].score > scores[j-1].score; j-- {
			scores[j], scores[j-1] = scores[j-1], scores[j]
		}
	}
 
	if k > len(scores) {
		k = len(scores)
	}
 
	result := make([]Chunk, k)
	for i := range result {
		result[i] = scores[i].chunk
	}
	return result
}
 
// SearchText embeds the query and returns the top k matching chunks.
// Convenience wrapper around Search for callers that have raw text.
func (vs *VectorStore) SearchText(query string, k int) ([]Chunk, error) {
	vec, err := vs.embed(query)
	if err != nil {
		return nil, err
	}
	return vs.Search(vec, k), nil
}
 
// All returns all loaded chunks. Useful for debugging or forced full injection.
func (vs *VectorStore) All() []Chunk {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	out := make([]Chunk, len(vs.chunks))
	copy(out, vs.chunks)
	return out
}
 
// Count returns the number of loaded chunks.
func (vs *VectorStore) Count() int {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	return len(vs.chunks)
}
 
// Reload forces a full re-scan and re-embed of all files.
// Use after manually editing memory files.
func (vs *VectorStore) Reload() error {
	vs.mu.Lock()
	vs.chunks = nil
	vs.mu.Unlock()
	return vs.Load()
}
 
// BuildInjection formats the top k chunks matching query into a string
// suitable for injection into the context window.
// alwaysInclude is a list of chunk names (relative paths) that are always
// injected regardless of similarity — use for core.md.
func (vs *VectorStore) BuildInjection(query string, k int, alwaysInclude []string, maxTokens int) string {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
 
	included := make(map[string]bool)
	var parts []string
 
	// Always-include chunks first
	for _, name := range alwaysInclude {
		for _, c := range vs.chunks {
			if c.Name == name || filepath.Base(c.Name) == name {
				if !included[c.Path] {
					included[c.Path] = true
					parts = append(parts, formatChunk(c))
				}
			}
		}
	}
 
	// Vector search for the rest
	if query != "" && vs.embed != nil {
		vec, err := vs.embed(query)
		if err == nil {
			type scored struct {
				chunk Chunk
				score float32
			}
			var scores []scored
			for _, c := range vs.chunks {
				if included[c.Path] || len(c.Vector) == 0 {
					continue
				}
				scores = append(scores, scored{chunk: c, score: cosineSimilarity(vec, c.Vector)})
			}
			for i := 1; i < len(scores); i++ {
				for j := i; j > 0 && scores[j].score > scores[j-1].score; j-- {
					scores[j], scores[j-1] = scores[j-1], scores[j]
				}
			}
			added := 0
			for _, s := range scores {
				if added >= k {
					break
				}
				// Only include chunks with meaningful similarity (threshold 0.3)
				if s.score < 0.3 {
					break
				}
				included[s.chunk.Path] = true
				parts = append(parts, formatChunk(s.chunk))
				added++
			}
		}
	}
 
	if len(parts) == 0 {
		return ""
	}
 
	result := strings.Join(parts, "\n\n---\n\n")
 
	// Trim to token budget (rough estimate: 1 token ≈ 4 chars)
	maxChars := maxTokens * 4
	if len(result) > maxChars {
		result = result[:maxChars]
		// Don't cut mid-word
		if idx := strings.LastIndex(result, " "); idx > 0 {
			result = result[:idx]
		}
		result += "\n[memory truncated]"
	}
 
	return result
}
 
// ── helpers ───────────────────────────────────────────────────────────────────
 
func formatChunk(c Chunk) string {
	return "# " + c.Name + "\n" + c.Content
}
 
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		av := float64(a[i])
		bv := float64(b[i])
		dot += av * bv
		normA += av * av
		normB += bv * bv
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return float32(dot / (math.Sqrt(normA) * math.Sqrt(normB)))
}