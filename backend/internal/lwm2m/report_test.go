// 文件用途：对象存储变更号与快照单测（observe/上报增量推送的地基）。
package lwm2m

import "testing"

func TestRevisionMonotonicAndSnapshot(t *testing.T) {
	s := NewObjectStore()
	if s.Revision() != 0 {
		t.Fatalf("初始 revision=%d", s.Revision())
	}
	if err := s.Set(19, 0, 0, "a"); err != nil {
		t.Fatal(err)
	}
	if s.Revision() != 1 {
		t.Fatalf("Set 后 revision=%d", s.Revision())
	}
	if err := s.Set(19, 0, 1, "b"); err != nil {
		t.Fatal(err)
	}
	if err := s.Set(3303, 0, 5700, "23.5"); err != nil {
		t.Fatal(err)
	}
	if s.Revision() != 3 {
		t.Fatalf("三次写入 revision=%d", s.Revision())
	}
	snap := s.Snapshot()
	if len(snap) != 3 || snap["19/0/0"] != "a" || snap["3303/0/5700"] != "23.5" {
		t.Fatalf("快照不符: %v", snap)
	}
	if !s.Delete(19, 0, 1) {
		t.Fatal("删除失败")
	}
	if s.Revision() != 4 {
		t.Fatalf("删除后 revision=%d", s.Revision())
	}
	if len(s.Snapshot()) != 2 {
		t.Fatalf("删除后快照数=%d", len(s.Snapshot()))
	}
}

func TestSnapshotIsCopy(t *testing.T) {
	s := NewObjectStore()
	_ = s.Set(1, 0, 0, "x")
	snap := s.Snapshot()
	_ = s.Set(2, 0, 0, "y")
	if _, ok := snap["2/0/0"]; ok {
		t.Fatal("快照应为写时拷贝，不含后续写入")
	}
}
