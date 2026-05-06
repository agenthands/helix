// Phase 62 P04 — RunClusterDetection orchestrator.
//
// Reads the effective graph (snapshot ⊕ overlay − tombstones) for
// (repoID, projection), computes weak components via WeakComponents, and
// persists them under the current graph_version through a single
// store.OverlayTx. The per-workspace mutex is held by BeginOverlayTx
// (Phase 60 D-04 — T-62-04-T4 mitigation): there is NO second mutex map
// in this package.
//
// Sequencing per detection run (single tx):
//   1. CurrentGraphVersion(repoID) → gv
//   2. QueryEffectiveGraph(repoID, projection) → (nodes, edges)
//   3. WeakComponents(nodes, edges) → []Cluster
//   4. BeginOverlayTx(repoID) → tx (acquires per-workspace mutex)
//   5. tx.DeleteClustersForGraphVersion(projection, gv)
//   6. tx.UpsertClusters(projection, gv, summaries)
//   7. tx.UpsertClusterMembers(projection, gv, memberRows)
//   8. tx.Commit()  (releases mutex)
//
// Any error after step 4 triggers tx.Rollback in a defer (rollback is
// safe to call after a successful Commit because OverlayTx tracks its own
// completion via unlockOnce).

package cluster

import (
	"context"
	"fmt"

	"github.com/agenthands/helix/internal/semantic/graph"
	"github.com/agenthands/helix/internal/semantic/store"
)

// ClusterStore is the persistence dependency RunClusterDetection takes.
// It is satisfied by *store.Store; tests substitute a fake.
type ClusterStore interface {
	BeginOverlayTx(ctx context.Context, repoID string) (ClusterTx, error)
	CurrentGraphVersion(ctx context.Context, repoID string) (uint64, error)
	QueryEffectiveGraph(ctx context.Context, repoID, projection string) (
		[]graph.NodeID, map[graph.NodeID]map[graph.NodeID]float64, error)
}

// ClusterTx is the per-tx surface RunClusterDetection writes through. It
// is satisfied by *store.OverlayTx (the concrete type returned by
// store.Store.BeginOverlayTx). Tests substitute a fake.
type ClusterTx interface {
	DeleteClustersForGraphVersion(ctx context.Context, projection string, graphVersion uint64) error
	UpsertClusters(ctx context.Context, projection string, graphVersion uint64, clusters []store.ClusterSummary) error
	UpsertClusterMembers(ctx context.Context, projection string, graphVersion uint64, rows []store.ClusterMemberRow) error
	Commit() error
	Rollback() error
}

// RunClusterDetection reads the effective graph for (repoID, projection),
// computes weak components, and persists them under the current
// graph_version. Returns (count, graphVersion, err) where count is the
// number of clusters written. On error any partial work is rolled back.
func RunClusterDetection(ctx context.Context, repoID, projection string,
	st ClusterStore,
) (count int, graphVersion uint64, err error) {

	gv, err := st.CurrentGraphVersion(ctx, repoID)
	if err != nil {
		return 0, 0, fmt.Errorf("read graph_version: %w", err)
	}

	nodes, edges, err := st.QueryEffectiveGraph(ctx, repoID, projection)
	if err != nil {
		return 0, gv, fmt.Errorf("read effective graph: %w", err)
	}

	clusters := WeakComponents(nodes, edges)

	tx, err := st.BeginOverlayTx(ctx, repoID)
	if err != nil {
		return 0, gv, fmt.Errorf("begin tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if err := tx.DeleteClustersForGraphVersion(ctx, projection, gv); err != nil {
		return 0, gv, fmt.Errorf("delete prior clusters: %w", err)
	}

	summaries := toSummaries(clusters)
	memberRows := toMemberRows(clusters)

	if err := tx.UpsertClusters(ctx, projection, gv, summaries); err != nil {
		return 0, gv, fmt.Errorf("upsert clusters: %w", err)
	}
	if err := tx.UpsertClusterMembers(ctx, projection, gv, memberRows); err != nil {
		return 0, gv, fmt.Errorf("upsert cluster members: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, gv, fmt.Errorf("commit: %w", err)
	}
	committed = true
	return len(clusters), gv, nil
}

// toSummaries flattens []Cluster into the boundary type the store package
// accepts. Length is preserved; ID and member-cardinality copied.
func toSummaries(clusters []Cluster) []store.ClusterSummary {
	if len(clusters) == 0 {
		return nil
	}
	out := make([]store.ClusterSummary, 0, len(clusters))
	for _, c := range clusters {
		out = append(out, store.ClusterSummary{
			ID:          uint64(c.ID),
			MemberCount: len(c.Members),
		})
	}
	return out
}

// toMemberRows fans out []Cluster into one store.ClusterMemberRow per
// (cluster, member). Total length = sum of len(Members).
func toMemberRows(clusters []Cluster) []store.ClusterMemberRow {
	total := 0
	for _, c := range clusters {
		total += len(c.Members)
	}
	if total == 0 {
		return nil
	}
	out := make([]store.ClusterMemberRow, 0, total)
	for _, c := range clusters {
		for _, m := range c.Members {
			out = append(out, store.ClusterMemberRow{
				ClusterID: uint64(c.ID),
				NodeID:    uint64(m),
			})
		}
	}
	return out
}
