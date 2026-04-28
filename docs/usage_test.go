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
