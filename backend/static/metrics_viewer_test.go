package static

import (
	"strings"
	"testing"
)

func TestMetricsViewerEnUpdateTableUsesDOMAPIs(t *testing.T) {
	html := string(MetricsViewerEnHTML)
	start := strings.Index(html, "function updateTable")
	if start == -1 {
		t.Fatal("function updateTable not found")
	}

	endOffset := strings.Index(html[start:], "// Fetch and process metrics data")
	if endOffset == -1 {
		t.Fatal("// Fetch and process metrics data not found after function updateTable")
	}

	updateTable := html[start : start+endOffset]
	if strings.Contains(updateTable, "innerHTML") {
		t.Error("updateTable must not use innerHTML")
	}

	for _, required := range []string{
		"createDocumentFragment",
		"createElement('td')",
		"textContent",
		"append",
		"replaceChildren",
	} {
		if !strings.Contains(updateTable, required) {
			t.Errorf("updateTable must contain %q", required)
		}
	}
}
