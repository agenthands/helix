//go:build fixturegen

// generate_fixtures.go regenerates the sigstore test trust root and bundle
// fixtures consumed by bench/container/verify_test.go.
//
// Run from the repo root:
//
//	go run -tags fixturegen ./bench/container/testdata/generate_fixtures.go
//
// NOTE: the build tag is `fixturegen` (not the conventional `ignore`) because
// Go 1.26's `go run -tags ignore` re-enables the `//go:build ignore` gen-tool
// files scattered through the stdlib and x/* modules, producing an
// "import cycle / program-not-importable" cascade. A unique tag isolates this
// generator from that collision.
//
// Output files (overwritten in place):
//   - bench/container/testdata/trusted_root.json                (test trust root)
//   - bench/container/testdata/manifest.json                    (sample OCI manifest bytes — the signed artifact)
//   - bench/container/testdata/manifest.canonical.sigstore.json (canonical bundle: mirror SAN + canonical issuer)
//   - bench/container/testdata/manifest.wrong-org.sigstore.json (wrong-org SAN bundle)
//   - bench/container/testdata/manifest.wrong-issuer.sigstore.json (wrong-issuer bundle)
//
// The bundles are produced by sigstore-go's in-process VirtualSigstore CA
// (pkg/testing/ca), which generates ephemeral Fulcio + TSA + Rekor keys; the
// test trust root is a JSON serialization of that ephemeral material. None of
// these files share trust material with any production trust root.
//
// The two "adversarial" bundles (wrong-org SAN, wrong-issuer) are signed
// against the SAME ephemeral CA as the canonical bundle so the verifier
// rejects them on identity-policy grounds, NOT on chain-of-trust grounds. The
// "tampered" and "unsigned" adversarial cases are synthesized in the test
// itself (byte-flip of the canonical bundle / garbage bytes) and need no
// committed fixture.
//
// The canonical SAN is pinned to the bench-mirror.yml publish workflow on the
// main branch — it MUST match pinnedSANRegexLiteral in
// bench/container/verify.go and Plan 04's bench-mirror.yml publish ref.
package main

import (
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"

	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	protocommon "github.com/sigstore/protobuf-specs/gen/pb-go/common/v1"
	protorekor "github.com/sigstore/protobuf-specs/gen/pb-go/rekor/v1"
	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/testing/ca"
	"github.com/sigstore/sigstore-go/pkg/tlog"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	canonicalSAN    = "https://github.com/agenthands/helix/.github/workflows/bench-mirror.yml@refs/heads/main"
	wrongOrgSAN     = "https://github.com/some-other-org/helix/.github/workflows/bench-mirror.yml@refs/heads/main"
	canonicalIssuer = "https://token.actions.githubusercontent.com"
	wrongIssuer     = "https://token.actions.example.invalid"
)

// sampleManifest is a representative OCI image manifest JSON blob. The verifier
// signs over the raw bytes, so the exact contents are not load-bearing — only
// that the same bytes are presented at verify time (the test passes these and
// asserts a byte-flipped copy is rejected).
const sampleManifest = `{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json","config":{"mediaType":"application/vnd.oci.image.config.v1+json","digest":"sha256:0000000000000000000000000000000000000000000000000000000000000000","size":7},"layers":[{"mediaType":"application/vnd.oci.image.layer.v1.tar+gzip","digest":"sha256:1111111111111111111111111111111111111111111111111111111111111111","size":42}]}`

func main() {
	outDir := "bench/container/testdata"
	if _, err := os.Stat(outDir); err != nil {
		// Allow running from inside testdata/ too.
		if _, err2 := os.Stat("generate_fixtures.go"); err2 == nil {
			outDir = "."
		} else {
			log.Fatalf("could not locate bench/container/testdata; run from repo root: %v", err)
		}
	}

	manifestBytes := []byte(sampleManifest)
	writeFile(filepath.Join(outDir, "manifest.json"), manifestBytes)

	vs, err := ca.NewVirtualSigstore()
	if err != nil {
		log.Fatalf("NewVirtualSigstore: %v", err)
	}

	// VirtualSigstore quirk: it stores transparency-log IDs as the ASCII bytes
	// of the hex-encoded SHA256 of the public key, but Bundle.LogId.KeyId is the
	// raw 32-byte hash. The verifier looks up the trust-root entry by
	// hex(rawBytes), so the ID in the trust root must be raw bytes.
	rekorLogs := map[string]*root.TransparencyLog{}
	for hexID, tl := range vs.RekorLogs() {
		raw, err := hex.DecodeString(hexID)
		if err != nil {
			log.Fatalf("hex-decoding rekor log id: %v", err)
		}
		fixed := *tl
		fixed.ID = raw
		rekorLogs[hexID] = &fixed
	}
	ctLogs := map[string]*root.TransparencyLog{}
	for hexID, tl := range vs.CTLogs() {
		raw, err := hex.DecodeString(hexID)
		if err != nil {
			log.Fatalf("hex-decoding ct log id: %v", err)
		}
		fixed := *tl
		fixed.ID = raw
		ctLogs[hexID] = &fixed
	}

	tr, err := root.NewTrustedRoot(
		root.TrustedRootMediaType01,
		vs.FulcioCertificateAuthorities(),
		ctLogs,
		vs.TimestampingAuthorities(),
		rekorLogs,
	)
	if err != nil {
		log.Fatalf("NewTrustedRoot: %v", err)
	}
	trJSON, err := tr.MarshalJSON()
	if err != nil {
		log.Fatalf("TrustedRoot.MarshalJSON: %v", err)
	}
	writeFile(filepath.Join(outDir, "trusted_root.json"), trJSON)

	// Canonical happy-path bundle (mirror SAN + canonical issuer).
	writeBundle(filepath.Join(outDir, "manifest.canonical.sigstore.json"),
		vs, canonicalSAN, canonicalIssuer, manifestBytes)

	// Wrong-org SAN — exercises the org-pinning portion of the SAN regex.
	writeBundle(filepath.Join(outDir, "manifest.wrong-org.sigstore.json"),
		vs, wrongOrgSAN, canonicalIssuer, manifestBytes)

	// Wrong-issuer — exercises the OIDC issuer pin.
	writeBundle(filepath.Join(outDir, "manifest.wrong-issuer.sigstore.json"),
		vs, canonicalSAN, wrongIssuer, manifestBytes)

	fmt.Println("fixtures regenerated")
}

func writeBundle(path string, vs *ca.VirtualSigstore, identity, issuer string, artifact []byte) {
	entity, err := vs.Sign(identity, issuer, artifact)
	if err != nil {
		log.Fatalf("vs.Sign(%s,%s): %v", identity, issuer, err)
	}
	pb, err := testEntityToProtoBundle(vs, entity)
	if err != nil {
		log.Fatalf("convert entity → bundle: %v", err)
	}
	if _, err := bundle.NewBundle(pb); err != nil {
		log.Fatalf("bundle.NewBundle: %v", err)
	}
	out, err := protojson.Marshal(pb)
	if err != nil {
		log.Fatalf("protojson.Marshal: %v", err)
	}
	writeFile(path, out)
}

// testEntityToProtoBundle converts a *ca.TestEntity into a *protobundle.Bundle
// suitable for protojson.Marshal. Copied from
// internal/upgrade/testdata/generate_fixtures.go (v0.1 bundle layout: leaf cert
// chain + timestamps + tlog entries with a regenerated SET).
func testEntityToProtoBundle(vs *ca.VirtualSigstore, e *ca.TestEntity) (*protobundle.Bundle, error) {
	sigContent, err := e.SignatureContent()
	if err != nil {
		return nil, fmt.Errorf("SignatureContent: %w", err)
	}
	verContent, err := e.VerificationContent()
	if err != nil {
		return nil, fmt.Errorf("VerificationContent: %w", err)
	}
	tsBytes, err := e.Timestamps()
	if err != nil {
		return nil, fmt.Errorf("Timestamps: %w", err)
	}
	tlogEntries, err := e.TlogEntries()
	if err != nil {
		return nil, fmt.Errorf("TlogEntries: %w", err)
	}

	pb := &protobundle.Bundle{
		MediaType: "application/vnd.dev.sigstore.bundle+json;version=0.1",
	}

	certIface, ok := verContent.(interface{ Certificate() *x509.Certificate })
	if !ok {
		return nil, fmt.Errorf("VerificationContent does not expose certificate")
	}
	leaf := certIface.Certificate()
	pb.VerificationMaterial = &protobundle.VerificationMaterial{
		Content: &protobundle.VerificationMaterial_X509CertificateChain{
			X509CertificateChain: &protocommon.X509CertificateChain{
				Certificates: []*protocommon.X509Certificate{
					{RawBytes: leaf.Raw},
				},
			},
		},
	}

	for _, tle := range tlogEntries {
		proto := tle.TransparencyLogEntry()
		if proto.KindVersion == nil {
			proto.KindVersion = &protorekor.KindVersion{
				Kind:    "hashedrekord",
				Version: "0.0.1",
			}
		}
		if proto.InclusionPromise == nil {
			payload := tlog.RekorPayload{
				Body:           tle.Body(),
				IntegratedTime: proto.IntegratedTime,
				LogIndex:       tle.LogIndex(),
				LogID:          hex.EncodeToString([]byte(tle.LogKeyID())),
			}
			set, err := vs.RekorSignPayload(payload)
			if err != nil {
				log.Fatalf("RekorSignPayload: %v", err)
			}
			proto.InclusionPromise = &protorekor.InclusionPromise{
				SignedEntryTimestamp: set,
			}
		}
		pb.VerificationMaterial.TlogEntries = append(pb.VerificationMaterial.TlogEntries, proto)
	}

	if len(tsBytes) > 0 {
		tvd := &protobundle.TimestampVerificationData{}
		for _, ts := range tsBytes {
			tvd.Rfc3161Timestamps = append(tvd.Rfc3161Timestamps, &protocommon.RFC3161SignedTimestamp{SignedTimestamp: ts})
		}
		pb.VerificationMaterial.TimestampVerificationData = tvd
	}

	type msgSigGetter interface {
		Signature() []byte
		Digest() []byte
		DigestAlgorithm() string
	}
	if ms, ok := sigContent.(msgSigGetter); ok {
		var alg protocommon.HashAlgorithm
		switch ms.DigestAlgorithm() {
		case "SHA2_256":
			alg = protocommon.HashAlgorithm_SHA2_256
		case "SHA2_384":
			alg = protocommon.HashAlgorithm_SHA2_384
		case "SHA2_512":
			alg = protocommon.HashAlgorithm_SHA2_512
		default:
			return nil, fmt.Errorf("unsupported digest algorithm: %s", ms.DigestAlgorithm())
		}
		pb.Content = &protobundle.Bundle_MessageSignature{
			MessageSignature: &protocommon.MessageSignature{
				MessageDigest: &protocommon.HashOutput{
					Algorithm: alg,
					Digest:    ms.Digest(),
				},
				Signature: ms.Signature(),
			},
		}
	} else {
		return nil, fmt.Errorf("SignatureContent shape unsupported by generator: %T", sigContent)
	}

	return pb, nil
}

func writeFile(path string, data []byte) {
	if err := os.WriteFile(path, data, 0o644); err != nil {
		log.Fatalf("writeFile %s: %v", path, err)
	}
	fmt.Printf("wrote %s (%d bytes)\n", path, len(data))
}
