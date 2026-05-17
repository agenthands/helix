// cluster_id.go — opaque cluster_id codec for Phase 72 P1 cluster tools.
//
// D1 (Phase 72): cluster_id is an opaque composite token "{projection}:{graph_version}:{id}"
// that binds a cluster integer id to the (projection, graph_version) that produced it.
// Handler entry decodes the token and refuses with a structured stale_cluster_id error
// when the embedded graph_version is no longer current.
package semantic

import (
	"fmt"
	"strconv"
	"strings"

	serr "github.com/agenthands/helix/internal/errors"
)

// decodedClusterID holds the three components of a decoded cluster_id token.
type decodedClusterID struct {
	Projection   string
	GraphVersion uint64
	ClusterIntID uint64
}

// encodeClusterID encodes (projection, graphVersion, clusterIntID) into the
// opaque token format "{projection}:{graphVersion}:{clusterIntID}".
// Tokens are safe for MCP surface: they contain only alphanumeric characters,
// underscores, colons, and digits.
func encodeClusterID(projection string, graphVersion, clusterIntID uint64) string {
	return fmt.Sprintf("%s:%d:%d", projection, graphVersion, clusterIntID)
}

// decodeClusterID parses an opaque cluster_id token into its three components.
// Returns serr.InvalidArgs when:
//   - The token does not contain exactly two colons (i.e., len(parts) != 3).
//   - The graph_version component is not a valid uint64.
//   - The cluster_int_id component is not a valid uint64.
//
// Thread-safe: pure function with no shared state.
func decodeClusterID(token string) (decodedClusterID, error) {
	parts := strings.SplitN(token, ":", 3)
	if len(parts) != 3 {
		return decodedClusterID{}, serr.New(serr.InvalidArgs,
			fmt.Sprintf("cluster_id %q: expected format '{projection}:{graph_version}:{cluster_int_id}', got %d part(s)",
				token, len(parts)))
	}
	gv, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return decodedClusterID{}, serr.New(serr.InvalidArgs,
			fmt.Sprintf("cluster_id %q: graph_version %q is not a valid uint64: %v",
				token, parts[1], err))
	}
	cid, err := strconv.ParseUint(parts[2], 10, 64)
	if err != nil {
		return decodedClusterID{}, serr.New(serr.InvalidArgs,
			fmt.Sprintf("cluster_id %q: cluster_int_id %q is not a valid uint64: %v",
				token, parts[2], err))
	}
	return decodedClusterID{
		Projection:   parts[0],
		GraphVersion: gv,
		ClusterIntID: cid,
	}, nil
}
