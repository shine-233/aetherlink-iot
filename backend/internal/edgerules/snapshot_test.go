// 文件用途：edgerules 快照信封与修订号单测（TB-21 scoped v1）。
// 核心逻辑：ParseSnapshot 的类型/修订号/图校验正负例；ClassifyRevision 四态
//
//	（newer/unchanged/regression/invalid）与云端 edge_sync_revision.go 语义对齐。
//
// 关键注意事项：修订号 < 1 一律拒收（fail-closed）；本文件不依赖 DB/broker。
package edgerules

import (
	"fmt"
	"strings"
	"testing"
)

// snapshotEnvelope 构造 rule_chain 快照信封文本。
func snapshotEnvelope(revision int64, graph string) []byte {
	return []byte(fmt.Sprintf(`{
		"type": "rule_chain",
		"resource_id": "chain-1",
		"name": "高温链",
		"content": %s,
		"generated_at": "2026-09-25T00:00:00Z",
		"version": 1,
		"revision": %d
	}`, graph, revision))
}

func TestParseSnapshotValid(t *testing.T) {
	snap, graph, _, err := ParseSnapshot(snapshotEnvelope(3, validGraph))
	if err != nil {
		t.Fatalf("ParseSnapshot error = %v", err)
	}
	if snap.Type != EdgeResourceRuleChain || snap.ResourceID != "chain-1" {
		t.Fatalf("snapshot = %+v", snap)
	}
	if snap.Revision != 3 {
		t.Fatalf("revision = %d, want 3", snap.Revision)
	}
	if graph == nil || len(graph.Nodes) != 3 {
		t.Fatalf("graph not parsed: %+v", graph)
	}
}

func TestParseSnapshotRejections(t *testing.T) {
	cases := []struct {
		name  string
		raw   string
		quiet string // 期望错误包含的文本；空串只要求失败
	}{
		{"empty payload", ``, "empty"},
		{"bad json", `{bad`, "not valid json"},
		{"dashboard type", `{"type":"dashboard","resource_id":"b1","revision":1}`, "unsupported snapshot type"},
		{"ota type", `{"type":"ota","resource_id":"p1","revision":1}`, "unsupported snapshot type"},
		{"missing resource id", `{"type":"rule_chain","revision":1}`, "resource_id is required"},
		{"missing revision", `{"type":"rule_chain","resource_id":"c1"}`, "not a valid version"},
		{"zero revision", `{"type":"rule_chain","resource_id":"c1","revision":0}`, "not a valid version"},
		{"negative revision", `{"type":"rule_chain","resource_id":"c1","revision":-2}`, "not a valid version"},
		{"empty graph", `{"type":"rule_chain","resource_id":"c1","revision":1,"content":{"nodes":[],"edges":[]}}`, "graph invalid"},
		{"graph bad json", `{"type":"rule_chain","resource_id":"c1","revision":1,"content":"nope"}`, "graph invalid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, err := ParseSnapshot([]byte(tc.raw))
			if err == nil {
				t.Fatalf("ParseSnapshot(%s) should fail", tc.name)
			}
			if tc.quiet != "" && !strings.Contains(err.Error(), tc.quiet) {
				t.Fatalf("error = %v, want contains %q", err, tc.quiet)
			}
		})
	}
}

func TestClassifyRevision(t *testing.T) {
	cases := []struct {
		name     string
		current  int64
		incoming int64
		want     RevisionState
	}{
		{"first load from empty", 0, 1, RevisionStateNewer},
		{"first load higher", 0, 7, RevisionStateNewer},
		{"upgrade", 3, 4, RevisionStateNewer},
		{"unchanged replays dedup", 4, 4, RevisionStateUnchanged},
		{"regression exposed", 4, 3, RevisionStateRegression},
		{"invalid zero incoming", 4, 0, RevisionStateInvalid},
		{"invalid negative incoming", 0, -1, RevisionStateInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifyRevision(tc.current, tc.incoming); got != tc.want {
				t.Fatalf("ClassifyRevision(%d,%d) = %q, want %q", tc.current, tc.incoming, got, tc.want)
			}
		})
	}
}

func TestIsValidRevision(t *testing.T) {
	if IsValidRevision(0) || IsValidRevision(-1) {
		t.Fatal("0/负数不是合法修订号")
	}
	if !IsValidRevision(1) || !IsValidRevision(1<<20) {
		t.Fatal(">=1 应为合法修订号")
	}
}
