package semantic

import (
	"testing"
)

// TestClusterID tests encode/decode round-trips and error cases for the
// cluster_id codec. See cluster_id.go for implementation.

func TestClusterID_EncodeRoundTrip(t *testing.T) {
	tests := []struct {
		name         string
		projection   string
		graphVersion uint64
		clusterIntID uint64
		wantToken    string
	}{
		{"basic", "weak_components", 42, 7, "weak_components:42:7"},
		{"zero_ids", "call_graph", 0, 0, "call_graph:0:0"},
		{"large_ids", "my_proj", 999999, 123456789, "my_proj:999999:123456789"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			token := encodeClusterID(tc.projection, tc.graphVersion, tc.clusterIntID)
			if token != tc.wantToken {
				t.Errorf("encodeClusterID(%q, %d, %d) = %q, want %q",
					tc.projection, tc.graphVersion, tc.clusterIntID, token, tc.wantToken)
			}
		})
	}
}

func TestClusterID_DecodeValid(t *testing.T) {
	tests := []struct {
		name  string
		token string
		want  decodedClusterID
	}{
		{
			name:  "basic",
			token: "weak_components:42:7",
			want:  decodedClusterID{Projection: "weak_components", GraphVersion: 42, ClusterIntID: 7},
		},
		{
			name:  "zero_ids",
			token: "call_graph:0:0",
			want:  decodedClusterID{Projection: "call_graph", GraphVersion: 0, ClusterIntID: 0},
		},
		{
			name:  "projection_with_underscores",
			token: "my_projection:100:200",
			want:  decodedClusterID{Projection: "my_projection", GraphVersion: 100, ClusterIntID: 200},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodeClusterID(tc.token)
			if err != nil {
				t.Fatalf("decodeClusterID(%q) unexpected error: %v", tc.token, err)
			}
			if got != tc.want {
				t.Errorf("decodeClusterID(%q) = %+v, want %+v", tc.token, got, tc.want)
			}
		})
	}
}

func TestClusterID_DecodeErrors(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{"too_few_parts", "bad"},
		{"only_one_colon", "proj:42"},
		{"empty_token", ""},
		{"graph_version_not_uint", "proj:notanint:7"},
		{"cluster_id_not_uint", "proj:42:notanint"},
		{"negative_not_allowed", "proj:-1:7"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeClusterID(tc.token)
			if err == nil {
				t.Errorf("decodeClusterID(%q) expected error, got nil", tc.token)
			}
		})
	}
}

func TestClusterID_RoundTrip(t *testing.T) {
	// decodeClusterID(encodeClusterID(proj, gv, id)) must return exact same values.
	cases := []struct {
		proj string
		gv   uint64
		id   uint64
	}{
		{"weak_components", 42, 7},
		{"call_graph", 0, 0},
		{"my_proj", 999999, 123456789},
	}
	for _, tc := range cases {
		token := encodeClusterID(tc.proj, tc.gv, tc.id)
		got, err := decodeClusterID(token)
		if err != nil {
			t.Errorf("round-trip decode error for (%q,%d,%d): %v", tc.proj, tc.gv, tc.id, err)
			continue
		}
		if got.Projection != tc.proj || got.GraphVersion != tc.gv || got.ClusterIntID != tc.id {
			t.Errorf("round-trip mismatch: got %+v, want {%q,%d,%d}", got, tc.proj, tc.gv, tc.id)
		}
	}
}
