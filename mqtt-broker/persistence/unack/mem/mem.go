// 文件用途：维护 persistence\unack\mem\mem.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package mem

import (
	"github.com/DrmagicE/gmqtt/persistence/unack"
	"github.com/DrmagicE/gmqtt/pkg/packets"
)

var _ unack.Store = (*Store)(nil)

type Store struct {
	clientID     string
	unackpublish map[packets.PacketID]struct{}
}

type Options struct {
	ClientID string
}

func New(opts Options) *Store {
	return &Store{
		clientID:     opts.ClientID,
		unackpublish: make(map[packets.PacketID]struct{}),
	}
}

func (s *Store) Init(cleanStart bool) error {
	if cleanStart {
		s.unackpublish = make(map[packets.PacketID]struct{})
	}
	return nil
}

func (s *Store) Set(id packets.PacketID) (bool, error) {
	if _, ok := s.unackpublish[id]; ok {
		return true, nil
	}
	s.unackpublish[id] = struct{}{}
	return false, nil
}

func (s *Store) Remove(id packets.PacketID) error {
	delete(s.unackpublish, id)
	return nil
}
