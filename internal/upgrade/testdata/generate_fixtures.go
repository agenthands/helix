//go:build ignore

// generate_fixtures.go regenerates the sigstore test trust root and bundle
// fixtures consumed by internal/upgrade/verify_test.go.
//
// Run from the repo root:
//
//	go run -tags ignore ./internal/upgrade/testdata/generate_fixtures.go
//
// Output files (overwritten in place):
//   - internal/upgrade/testdata/trusted_root.json          (test trust root)
//   - internal/upgrade/testdata/sample-archive.tar.gz.sigstore.json  (canonical bundle)
//   - internal/upgrade/testdata/sample-archive.tar.gz.rc.sigstore.json (RC-tag bundle)
//   - internal/upgrade/testdata/sample-archive.tar.gz.wrong-org.sigstore.json (wrong-org SAN bundle)
//   - internal/upgrade/testdata/sample-archive.tar.gz.wrong-issuer.sigstore.json (wrong-issuer bundle)
//
// The bundles are produced by sigstore-go's in-process VirtualSigstore CA
// (pkg/testing/ca), which generates ephemeral Fulcio + TSA + Rekor keys; the
// test trust root is a JSON serialization of that ephemeral material. None of
// these files share trust material with the production trust root at
// internal/upgrade/trusted_root.json — that file is sourced from the upstream
// sigstore-go public-good snapshot and refreshed via `make update-trust-root`.
//
// Phase 58 D-02 / R-2 plan reference. The four "adversarial" bundles
// (wrong-org SAN, wrong-issuer) are signed against the SAME ephemeral CA as
// the canonical bundle so the verifier rejects them on identity-policy
// grounds, NOT on chain-of-trust grounds.
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
	canonicalSAN   = "https://github.com/agenthands/helix/.github/workflows/release.yml@refs/tags/v1.10.0"
	rcSAN          = "https://github.com/agenthands/helix/.github/workflows/release.yml@refs/tags/v1.10.0-rc1"
	wrongOrgSAN    = "https://github.com/some-other-org/helix/.github/workflows/release.yml@refs/tags/v1.10.0"
	canonicalIssuer = "https://token.actions.githubusercontent.com"
	wrongIssuer    = "https://token.actions.example.invalid"
)

func main() {
	outDir := "internal/upgrade/testdata"
	if _, err := os.Stat(filepath.Join(outDir, "sample-archive.tar.gz")); err != nil {
		// Allow running from inside testdata/ too.
		if _, err2 := os.Stat("sample-archive.tar.gz"); err2 == nil {
			outDir = "."
		} else {
			log.Fatalf("could not locate sample-archive.tar.gz; run from repo root: %v", err)
		}
	}

	archivePath := filepath.Join(outDir, "sample-archive.tar.gz")
	archiveBytes, err := os.ReadFile(archivePath)
	if err != nil {
		log.Fatalf("reading archive %s: %v", archivePath, err)
	}

	vs, err := ca.NewVirtualSigstore()
	if err != nil {
		log.Fatalf("NewVirtualSigstore: %v", err)
	}

	// VirtualSigstore quirk: it stores transparency-log IDs as the ASCII bytes
	// of the hex-encoded SHA256 of the public key, but Bundle.LogId.KeyId is
	// the raw 32-byte hash. The verifier looks up the trust-root entry by
	// hex(rawBytes), so the ID in the trust root must be raw bytes, not the
	// ASCII-hex representation. Decode them on the way in.
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

	// Trust root JSON — the verifier consumes this via testTrustedRootOverride.
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

	// Canonical happy-path bundle (final-tag SAN).
	writeBundle(filepath.Join(outDir, "sample-archive.tar.gz.sigstore.json"),
		vs, canonicalSAN, canonicalIssuer, archiveBytes)

	// RC-tag bundle for the broader-regex regression guard.
	writeBundle(filepath.Join(outDir, "sample-archive.tar.gz.rc.sigstore.json"),
		vs, rcSAN, canonicalIssuer, archiveBytes)

	// Wrong-org SAN — exercises the org-pinning portion of the regex.
	writeBundle(filepath.Join(outDir, "sample-archive.tar.gz.wrong-org.sigstore.json"),
		vs, wrongOrgSAN, canonicalIssuer, archiveBytes)

	// Wrong-issuer — exercises the OIDC issuer pin.
	writeBundle(filepath.Join(outDir, "sample-archive.tar.gz.wrong-issuer.sigstore.json"),
		vs, canonicalSAN, wrongIssuer, archiveBytes)

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
	// Sanity: ensure the bundle round-trips through bundle.NewBundle (validates
	// shape against the verify-side parser before we write it).
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
// suitable for protojson.Marshal. It encodes the leaf cert chain (form (2),
// v0.1 bundle layout) plus all timestamps and tlog entries.
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

	// Use v0.1 media type so an InclusionPromise is sufficient (the test
	// CA emits a signedEntryTimestamp but no inclusion proof). v0.2+ would
	// require a full inclusion proof, which VirtualSigstore.Sign() does
	// not generate.
	pb := &protobundle.Bundle{
		MediaType: "application/vnd.dev.sigstore.bundle+json;version=0.1",
	}

	// Leaf cert. v0.1 bundles use X509CertificateChain (form 2). The chain
	// from VirtualSigstore is leaf, intermediate, root — drop the root per
	// the spec's "Signers MUST NOT include root CA certificates in bundles"
	// guidance to avoid validation noise on roundtrip.
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

	// Tlog entries. The VirtualSigstore-generated TransparencyLogEntry omits
	// KindVersion and InclusionPromise (its tlog.NewEntry path leaves both
	// fields unset on the protobuf and stores the SET on a private struct
	// field with no accessor). KindVersion is required by
	// tlog.ParseTransparencyLogEntry; the SET must round-trip via
	// InclusionPromise.SignedEntryTimestamp for the verifier to find it. We
	// regenerate the SET by re-signing the canonical RekorPayload through
	// VirtualSigstore.RekorSignPayload — same key, same payload, same
	// signature ECDSA-modulo-nonce (any valid signature verifies).
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

	// RFC3161 signed timestamps (TSA).
	if len(tsBytes) > 0 {
		tvd := &protobundle.TimestampVerificationData{}
		for _, ts := range tsBytes {
			tvd.Rfc3161Timestamps = append(tvd.Rfc3161Timestamps, &protocommon.RFC3161SignedTimestamp{SignedTimestamp: ts})
		}
		pb.VerificationMaterial.TimestampVerificationData = tvd
	}

	// Signature content — message signature (digest + sig). DSSE envelope is
	// not used by sample-archive.tar.gz signing.
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
