package ragindex

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"

	chromem "github.com/philippgille/chromem-go"
)

// collectionName is the single chromem collection every per-corpus index uses.
const collectionName = "corpus"

// Index is a per-corpus embedding index backed by a persistent chromem DB at
// <cacheDir>/bench-rag-index/<corpus_sha>/. It is built once per
// (corpus_sha, embedder_id) and reused on warm reopen with zero re-embeds.
type Index struct {
	coll       *chromem.Collection
	embedderID string
}

// Result is a single k-NN hit: the chunk ID, its source-relative path, content,
// and cosine similarity score.
type Result struct {
	ChunkID    string
	RelPath    string
	Content    string
	Similarity float32
}

// Open builds (cold) or loads (warm) the embedding index for the corpus rooted
// at root, selecting the embedder via selectEmbedder (OpenAI -> Ollama -> stub).
// On a cold cache path it chunks every corpus file and embeds the chunks; on a
// warm path chromem auto-loads the persisted docs+embeddings and NO re-embedding
// occurs. The selected embedder_id is recorded in the collection metadata and
// returned via EmbedderID.
func Open(ctx context.Context, root string) (*Index, error) {
	ef, embedderID, err := selectEmbedder()
	if err != nil {
		return nil, fmt.Errorf("ragindex: select embedder: %w", err)
	}
	return openWith(ctx, root, ef, embedderID)
}

// openWith is the embedder-injectable core of Open. Tests supply a deterministic
// (and call-counting) embedder so the package is fully exercisable with zero
// network. The SAME embedder MUST be supplied on every reopen because chromem
// does not persist the EmbeddingFunc (Pitfall 1).
func openWith(ctx context.Context, root string, ef chromem.EmbeddingFunc, embedderID string) (*Index, error) {
	path, err := IndexPath(root)
	if err != nil {
		return nil, fmt.Errorf("ragindex: resolve index path: %w", err)
	}

	db, err := chromem.NewPersistentDB(path, true /* gzip */)
	if err != nil {
		return nil, fmt.Errorf("ragindex: open persistent db: %w", err)
	}

	// GetOrCreateCollection re-attaches the embedder (which is never persisted)
	// to whatever docs chromem auto-loaded from disk. embedder_id is recorded in
	// metadata so a model mismatch on reopen is detectable.
	coll, err := db.GetOrCreateCollection(collectionName, map[string]string{"embedder": embedderID}, ef)
	if err != nil {
		return nil, fmt.Errorf("ragindex: get-or-create collection: %w", err)
	}

	idx := &Index{coll: coll, embedderID: embedderID}

	// Warm path: chromem already loaded persisted docs => skip embedding.
	if coll.Count() > 0 {
		return idx, nil
	}

	// Cold path: chunk every corpus file and embed.
	docs, err := buildDocuments(root)
	if err != nil {
		return nil, err
	}
	if len(docs) > 0 {
		if err := coll.AddDocuments(ctx, docs, runtime.NumCPU()); err != nil {
			return nil, fmt.Errorf("ragindex: add documents: %w", err)
		}
	}
	return idx, nil
}

// buildDocuments walks the corpus and turns every file's chunks into chromem
// Documents (without embeddings; AddDocuments embeds them concurrently). The
// chunk ID is the document ID and the relative path is carried in metadata so
// query results can report it.
func buildDocuments(root string) ([]chromem.Document, error) {
	var docs []chromem.Document
	walkErr := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		b, readErr := os.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		for _, piece := range Chunk(string(b), rel) {
			docs = append(docs, chromem.Document{
				ID:       piece.ID,
				Metadata: map[string]string{"rel_path": piece.RelPath},
				Content:  piece.Content,
			})
		}
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("ragindex: walk corpus: %w", walkErr)
	}
	return docs, nil
}

// Query returns the top-k chunks nearest to queryText by cosine similarity.
func (i *Index) Query(ctx context.Context, queryText string, k int) ([]Result, error) {
	if k < 1 {
		k = 1
	}
	// chromem errors if nResults exceeds the document count; clamp to be safe.
	if n := i.coll.Count(); k > n {
		k = n
	}
	if k == 0 {
		return nil, nil
	}
	hits, err := i.coll.Query(ctx, queryText, k, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("ragindex: query: %w", err)
	}
	out := make([]Result, 0, len(hits))
	for _, h := range hits {
		out = append(out, Result{
			ChunkID:    h.ID,
			RelPath:    h.Metadata["rel_path"],
			Content:    h.Content,
			Similarity: h.Similarity,
		})
	}
	return out, nil
}

// EmbedderID reports the embedder_id this index was built/opened with.
func (i *Index) EmbedderID() string { return i.embedderID }

// Count reports the number of documents (chunks) in the index.
func (i *Index) Count() int { return i.coll.Count() }
