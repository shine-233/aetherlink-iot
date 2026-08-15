package main

import (
	"os"
	"strings"
	"testing"
)

// TestDockerRuntimeBoundary keeps development sources and tools out of the
// runtime image while preserving resources that the backend reads at startup.
func TestDockerRuntimeBoundary(t *testing.T) {
	content, err := os.ReadFile("Dockerfile")
	if err != nil {
		t.Fatalf("read Dockerfile: %v", err)
	}

	dockerfile := string(content)
	for _, required := range []string{
		"COPY --from=builder --chown=aetherlink:aetherlink /go/src/app/AetherLink-Go ./AetherLink-Go",
		"COPY --from=builder --chown=aetherlink:aetherlink /go/src/app/configs ./configs",
		"COPY --from=builder --chown=aetherlink:aetherlink /go/src/app/sql ./sql",
		`USER 10001:10001`,
		`ENTRYPOINT [ "./AetherLink-Go" ]`,
	} {
		if !strings.Contains(dockerfile, required) {
			t.Errorf("Dockerfile must preserve runtime resource contract: %s", required)
		}
	}

	for _, forbidden := range []string{
		"COPY --from=builder /go/src/app .",
		"COPY --from=builder /go/src/app/ ./",
	} {
		if strings.Contains(dockerfile, forbidden) {
			t.Errorf("runtime image must not copy the complete backend source tree: %s", forbidden)
		}
	}
}
