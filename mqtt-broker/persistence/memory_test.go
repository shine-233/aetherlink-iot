// 文件用途：维护 persistence\memory_test.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package persistence

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/DrmagicE/gmqtt/config"
	queue_test "github.com/DrmagicE/gmqtt/persistence/queue/test"
	sess_test "github.com/DrmagicE/gmqtt/persistence/session/test"
	"github.com/DrmagicE/gmqtt/persistence/subscription"
	sub_test "github.com/DrmagicE/gmqtt/persistence/subscription/test"
	unack_test "github.com/DrmagicE/gmqtt/persistence/unack/test"
	"github.com/DrmagicE/gmqtt/server"
)

type MemorySuite struct {
	suite.Suite
	new server.NewPersistence
	p   server.Persistence
}

func (s *MemorySuite) TestQueue() {
	a := assert.New(s.T())
	qs, err := s.p.NewQueueStore(queue_test.TestServerConfig, queue_test.TestNotifier, queue_test.TestClientID)
	a.Nil(err)
	queue_test.TestQueue(s.T(), qs)
}
func (s *MemorySuite) TestSubscription() {
	newFn := func() subscription.Store {
		st, err := s.p.NewSubscriptionStore(queue_test.TestServerConfig)
		if err != nil {
			panic(err)
		}
		return st
	}
	sub_test.TestSuite(s.T(), newFn)
}

func (s *MemorySuite) TestSession() {
	a := assert.New(s.T())
	st, err := s.p.NewSessionStore(queue_test.TestServerConfig)
	a.Nil(err)
	sess_test.TestSuite(s.T(), st)
}

func (s *MemorySuite) TestUnack() {
	a := assert.New(s.T())
	st, err := s.p.NewUnackStore(unack_test.TestServerConfig, unack_test.TestClientID)
	a.Nil(err)
	unack_test.TestSuite(s.T(), st)
}

func TestMemory(t *testing.T) {
	p, err := NewMemory(config.Config{})
	if err != nil {
		t.Fatal(err.Error())
	}
	suite.Run(t, &MemorySuite{
		p: p,
	})
}
