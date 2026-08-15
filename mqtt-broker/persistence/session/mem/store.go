// 文件用途：维护 persistence\session\mem\store.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package mem

import (
	"sync"

	"github.com/DrmagicE/gmqtt"
	"github.com/DrmagicE/gmqtt/persistence/session"
)

var _ session.Store = (*Store)(nil)

func New() *Store {
	return &Store{
		mu:   sync.Mutex{},
		sess: make(map[string]*gmqtt.Session),
	}
}

type Store struct {
	mu   sync.Mutex
	sess map[string]*gmqtt.Session
}

func (s *Store) Set(session *gmqtt.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sess[session.ClientID] = session
	return nil
}

func (s *Store) Remove(clientID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sess, clientID)
	return nil
}

func (s *Store) Get(clientID string) (*gmqtt.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sess[clientID], nil
}

func (s *Store) GetAll() ([]*gmqtt.Session, error) {
	return nil, nil
}

func (s *Store) SetSessionExpiry(clientID string, expiry uint32) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s, ok := s.sess[clientID]; ok {
		s.ExpiryInterval = expiry

	}
	return nil
}

func (s *Store) Iterate(fn session.IterateFn) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range s.sess {
		cont := fn(v)
		if !cont {
			break
		}
	}
	return nil
}
