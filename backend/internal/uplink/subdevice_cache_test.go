package uplink

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
)

func resolvedIDs(t *testing.T, r *subDeviceResolver, parent string, addrs ...string) map[string]string {
	t.Helper()
	got, err := r.resolve(parent, addrs)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	out := map[string]string{}
	for a, d := range got {
		out[a] = d.ID
	}
	return out
}

func TestSubDeviceResolverCachesHitsAndMisses(t *testing.T) {
	topo := standardTopology()
	r := topo.resolver()

	want := map[string]string{"a": "dev-a", "b": "dev-b"}
	for i := 0; i < 3; i++ {
		if got := resolvedIDs(t, r, "gw", "a", "b", "zz"); !reflect.DeepEqual(got, want) {
			t.Fatalf("round %d: got %v, want %v", i, got, want)
		}
	}
	if n := topo.queryCount(); n != 1 {
		t.Fatalf("db queries = %d, want 1 (hits and negative hits must be served from the index)", n)
	}
}

func TestSubDeviceResolverRevalidatesAgainstDeviceCache(t *testing.T) {
	topo := standardTopology()
	r := topo.resolver()

	// Re-address dev-a: the device write path invalidates its device cache
	// entry, so the fresh row no longer matches and the DB is consulted.
	resolvedIDs(t, r, "gw", "a")
	newAddr := "a2"
	topo.devices["dev-a"].SubDeviceAddr = &newAddr
	if got := resolvedIDs(t, r, "gw", "a"); len(got) != 0 {
		t.Fatalf("stale entry served after re-address: %v", got)
	}
	if got := resolvedIDs(t, r, "gw", "a2"); got["a2"] != "dev-a" {
		t.Fatalf("new address not resolved: %v", got)
	}

	// Delete dev-b.
	resolvedIDs(t, r, "gw", "b")
	delete(topo.devices, "dev-b")
	if got := resolvedIDs(t, r, "gw", "b"); len(got) != 0 {
		t.Fatalf("deleted device served: %v", got)
	}

	// Re-parent dev-c from dev-g1 to gw.
	resolvedIDs(t, r, "dev-g1", "c")
	gw := "gw"
	topo.devices["dev-c"].ParentID = &gw
	if got := resolvedIDs(t, r, "dev-g1", "c"); len(got) != 0 {
		t.Fatalf("re-parented device served under old parent: %v", got)
	}
	if got := resolvedIDs(t, r, "gw", "c"); got["c"] != "dev-c" {
		t.Fatalf("re-parented device not found under new parent: %v", got)
	}
}

func TestSubDeviceResolverNegativeEntryExpires(t *testing.T) {
	topo := standardTopology()
	r := topo.resolver()
	now := time.Unix(1_000_000, 0)
	r.now = func() time.Time { return now }

	if got := resolvedIDs(t, r, "gw", "new"); len(got) != 0 {
		t.Fatalf("unexpected: %v", got)
	}
	topo.add("dev-new", "gw", "new")
	if got := resolvedIDs(t, r, "gw", "new"); len(got) != 0 {
		t.Fatalf("negative entry should still apply: %v", got)
	}
	now = now.Add(subDeviceNegativeTTL + time.Second)
	if got := resolvedIDs(t, r, "gw", "new"); got["new"] != "dev-new" {
		t.Fatalf("new sub-device not found after negative TTL: %v", got)
	}
}

func TestSubDeviceResolverInvalidate(t *testing.T) {
	topo := standardTopology()
	r := topo.resolver()
	resolvedIDs(t, r, "gw", "a", "missing")
	resolvedIDs(t, r, "dev-g1", "c")

	// Invalidating a parent drops all its entries, negative ones included.
	r.invalidate("gw")
	topo.add("dev-missing", "gw", "missing")
	before := topo.queryCount()
	if got := resolvedIDs(t, r, "gw", "missing"); got["missing"] != "dev-missing" {
		t.Fatalf("negative entry survived parent invalidation: %v", got)
	}
	// Invalidating a device id drops entries pointing at it.
	r.invalidate("dev-c")
	resolvedIDs(t, r, "dev-g1", "c")
	if n := topo.queryCount() - before; n != 2 {
		t.Fatalf("queries after invalidation = %d, want 2", n)
	}
}

func TestSubDeviceResolverPropagatesDBError(t *testing.T) {
	r := standardTopology().resolver()
	r.queryDB = func([]string, string) (map[string]*model.Device, error) { return nil, errors.New("boom") }
	if _, err := r.resolve("gw", []string{"a"}); err == nil {
		t.Fatal("expected error")
	}
	if len(r.entries) != 0 {
		t.Fatal("errors must not populate negative entries")
	}
}
