// 文件用途：LwM2M 对象实例模型（ROADMAP C6 续）——设备资源树 /{对象}/{实例}/{资源}。
// 核心逻辑：ObjectStore 以 objID→instID→resID 存资源值（int/string/bytes），提供
//   Get/Set/Delete/Clear，并把多段 Uri-Path 映射到 LwM2M 语义；经 coap.Registry 前缀通配
//   挂载形如 "/19*"（二进制 App 数据容器 19/0）等对象读取/写入。
// 关键注意事项：
//   - 路径段必须为正整数，非数字或资源缺失返回 4.04/4.05；长度与值类型做边界钳制；
//   - 多客户端隔离：一个 ObjectStore 对应一个已注册客户端（由上层按 ClientID 分发）。
package lwm2m

import (
	"fmt"
	"strconv"
	"strings"

	"aetherlink-iot/backend/internal/coap"
)

// maxResourceValueSize 单资源值上限（防放大）。
const maxResourceValueSize = 64 * 1024

// ObjectStore 单客户端的对象实例存储（objID→instID→resID→value）。
type ObjectStore struct {
	values   map[uint16]map[uint16]map[uint16]string
	revision uint64 // 单调变更号：每次写/删 +1，供 observe/上报增量推送
}

// NewObjectStore 新建对象存储。
func NewObjectStore() *ObjectStore {
	return &ObjectStore{values: map[uint16]map[uint16]map[uint16]string{}}
}

// Revision 返回当前变更号（0 = 尚无写入）。
func (s *ObjectStore) Revision() uint64 { return s.revision }

// Set 写资源值（非数值以外一律按文本存；超长拒绝）。
func (s *ObjectStore) Set(obj, inst, res uint16, value string) error {
	if len(value) > maxResourceValueSize {
		return fmt.Errorf("lwm2m: 资源值超限 %d", len(value))
	}
	if _, ok := s.values[obj]; !ok {
		s.values[obj] = map[uint16]map[uint16]string{}
	}
	if _, ok := s.values[obj][inst]; !ok {
		s.values[obj][inst] = map[uint16]string{}
	}
	s.values[obj][inst][res] = value
	s.revision++
	return nil
}

// Snapshot 返回 (obj/inst/res → value) 扁平快照，供全量上报。
func (s *ObjectStore) Snapshot() map[string]string {
	out := map[string]string{}
	for o, insts := range s.values {
		for i, ress := range insts {
			for r, v := range ress {
				out[pathKey(o, i, r)] = v
			}
		}
	}
	return out
}

func pathKey(o, i, r uint16) string {
	return fmt.Sprintf("%d/%d/%d", o, i, r)
}

// Get 读资源值；不存在返回 ok=false。
func (s *ObjectStore) Get(obj, inst, res uint16) (string, bool) {
	if insts, ok := s.values[obj]; ok {
		if ress, ok2 := insts[inst]; ok2 {
			v, ok3 := ress[res]
			return v, ok3
		}
	}
	return "", false
}

// Delete 删除资源（或整实例/整对象当 res/inst 用通配路径时由上层调用）。
func (s *ObjectStore) Delete(obj, inst, res uint16) bool {
	if insts, ok := s.values[obj]; ok {
		if ress, ok2 := insts[inst]; ok2 {
			if _, ok3 := ress[res]; ok3 {
				delete(ress, res)
				if len(ress) == 0 {
					delete(insts, inst)
				}
				s.revision++
				return true
			}
		}
	}
	return false
}

// SnapshotCount 返回已存储的资源条目数（统计/健康检查用）。
func (s *ObjectStore) SnapshotCount() int {
	n := 0
	for _, insts := range s.values {
		for _, ress := range insts {
			n += len(ress)
		}
	}
	return n
}

// parseResourcePath 把 "/19/0/1" 拆成 (obj,inst,res)；不合法返回 false。
func parseResourcePath(path string) (uint16, uint16, uint16, bool) {
	segs := strings.Split(strings.Trim(path, "/"), "/")
	if len(segs) != 3 {
		return 0, 0, 0, false
	}
	nums := make([]uint64, 3)
	for i, s := range segs {
		v, err := strconv.ParseUint(strings.TrimSpace(s), 10, 16)
		if err != nil {
			return 0, 0, 0, false
		}
		nums[i] = v
	}
	return uint16(nums[0]), uint16(nums[1]), uint16(nums[2]), true
}

// ObjectHandler 构造挂在 coap.Registry 前缀上的对象读写处理器。
func (s *ObjectStore) ObjectHandler() coap.Handler {
	return func(req *coap.Message) (coap.Code, []byte, int, error) {
		obj, inst, res, ok := parseResourcePath(req.UriPath())
		if !ok {
			return coap.CodeBadRequest, []byte("resource path must be /{obj}/{inst}/{res}"), 0, nil
		}
		switch req.Code {
		case coap.CodeGet:
			if v, exists := s.Get(obj, inst, res); exists {
				return coap.CodeContent, []byte(v), 0, nil
			}
			return coap.CodeNotFound, []byte("resource not found"), 0, nil
		case coap.CodePut:
			if err := s.Set(obj, inst, res, string(req.Payload)); err != nil {
				return coap.CodeBadRequest, []byte(err.Error()), 0, nil
			}
			return coap.CodeChanged, nil, 0, nil
		case coap.CodePost:
			if err := s.Set(obj, inst, res, string(req.Payload)); err != nil {
				return coap.CodeBadRequest, []byte(err.Error()), 0, nil
			}
			return coap.CodeChanged, nil, 0, nil
		case coap.CodeDelete:
			if s.Delete(obj, inst, res) {
				return coap.CodeDeleted, nil, 0, nil
			}
			return coap.CodeNotFound, []byte("resource not found"), 0, nil
		default:
			return coap.CodeMethodNotAllowed, nil, 0, nil
		}
	}
}

// BindObjects 把对象存储挂到给定 coap.Registry 的 /{objPrefix}* 前缀上。
func BindObjects(cr *coap.Registry, objID uint16, store *ObjectStore) {
	cr.Register(fmt.Sprintf("/%d*", objID), store.ObjectHandler())
}
