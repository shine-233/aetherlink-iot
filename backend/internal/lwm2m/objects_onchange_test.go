// 文件用途：ObjectStore.SetOnChange 写入回调测试（P1-C 遥测汇入挂钩点）。
package lwm2m

import "testing"

func TestObjectStoreOnChangeFiresOnSet(t *testing.T) {
	s := NewObjectStore()
	type call struct {
		obj, inst, res uint16
		value          string
	}
	calls := make([]call, 0, 2)
	s.SetOnChange(func(obj, inst, res uint16, value string) {
		calls = append(calls, call{obj, inst, res, value})
	})

	if err := s.Set(3303, 0, 5700, "23.5"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := s.Set(3303, 0, 5700, "24.0"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("onChange 触发次数 = %d, 期望 2", len(calls))
	}
	if calls[0] != (call{3303, 0, 5700, "23.5"}) || calls[1] != (call{3303, 0, 5700, "24.0"}) {
		t.Fatalf("onChange 参数不符: %+v", calls)
	}
}

func TestObjectStoreOnChangeNotFiredOnFailedSet(t *testing.T) {
	s := NewObjectStore()
	fired := false
	s.SetOnChange(func(obj, inst, res uint16, value string) { fired = true })

	oversize := make([]byte, maxResourceValueSize+1)
	if err := s.Set(3303, 0, 5700, string(oversize)); err == nil {
		t.Fatal("超长值必须被拒绝")
	}
	if fired {
		t.Fatal("失败的 Set 不得触发 onChange")
	}
}

func TestObjectStoreOnChangeNilSafeAndClearable(t *testing.T) {
	s := NewObjectStore()
	// 未注册回调直接写入必须安全。
	if err := s.Set(1, 0, 0, "x"); err != nil {
		t.Fatalf("set: %v", err)
	}
	// 注册后再清除（nil）恢复无回调状态。
	s.SetOnChange(func(obj, inst, res uint16, value string) {})
	s.SetOnChange(nil)
	if err := s.Set(1, 0, 1, "y"); err != nil {
		t.Fatalf("set after clear: %v", err)
	}
}
