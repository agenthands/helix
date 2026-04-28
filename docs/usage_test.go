package docs_test

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestUSAGEObservability(t *testing.T) {
	data, err := os.ReadFile("../USAGE.md")
	if err != nil {
		t.Fatalf("read USAGE.md: %v", err)
	}
	text := string(data)

	for _, want := range []string{
		"### In-Binary Metrics Page",
		"http://127.0.0.1:9100/",
		"docs/runbooks",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("USAGE.md missing required substring %q", want)
		}
	}

	// Extract the In-Binary Metrics Page subsection body (between its heading
	// and the next ### or ## heading, or end-of-file).
	re := regexp.MustCompile("(?ms)^### In-Binary Metrics Page\\s*\\n(.*?)(?:^### |^## |\\z)")
	match := re.FindStringSubmatch(text)
	if len(match) < 2 {
		t.Fatalf("could not isolate ### In-Binary Metrics Page body in USAGE.md")
	}
	body := strings.ToLower(match[1])
	for _, bad := range []string{"prometheus", "grafana", "docker", "podman"} {
		if strings.Contains(body, bad) {
			t.Errorf("In-Binary Metrics Page paragraph mentions third-party software %q (criterion #4 forbids prerequisites)", bad)
		}
	}
}

// TestUsageDocumentsTracingSampling is the Phase 55-03 / OBS-04 #3
// regression gate that USAGE.md continues to document the tracing
// sampling story (head-vs-tail honesty, smoke-test recipe, and
// reference to the TRACE-AUDIT.md certification artifact). Substring
// checks are scoped to subtests so failures name the missing piece.
func TestUsageDocumentsTracingSampling(t *testing.T) {
	data, err := os.ReadFile("../USAGE.md")
	if err != nil {
		t.Fatalf("read USAGE.md: %v", err)
	}
	text := string(data)

	for _, want := range []string{
		"tracing_sample_ratio",
		"1.0",
		"ObservabilityConfig",
		"otel/opentelemetry-collector-contrib",
		"TRACE-AUDIT.md",
	} {
		want := want
		t.Run(want, func(t *testing.T) {
			if !strings.Contains(text, want) {
				t.Errorf("USAGE.md missing required substring %q (Phase 55-03 sampling docs regression)", want)
			}
		})
	}

	// Head-vs-tail-sampling honesty: both keywords MUST appear in the
	// observability prose (not only in headings) so an operator reading
	// the section understands the in-binary sampler is head-only.
	lower := strings.ToLower(text)
	for _, want := range []string{"head", "tail"} {
		want := want
		t.Run("honesty:"+want, func(t *testing.T) {
			if !strings.Contains(lower, want) {
				t.Errorf("USAGE.md tracing section missing %q-sampling discussion", want)
			}
		})
	}
}
