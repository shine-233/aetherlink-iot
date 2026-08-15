// 文件用途：维护 persistence\session\test\test_suite.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package test

import (
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/DrmagicE/gmqtt"
	"github.com/DrmagicE/gmqtt/persistence/session"
)

func TestSuite(t *testing.T, store session.Store) {
	a := assert.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	var tt = []*gmqtt.Session{
		{
			ClientID: "client",
			Will: &gmqtt.Message{
				Topic:   "topicA",
				Payload: []byte("abc"),
			},
			WillDelayInterval: 1,
			ConnectedAt:       time.Unix(1, 0),
			ExpiryInterval:    2,
		}, {
			ClientID:          "client2",
			Will:              nil,
			WillDelayInterval: 0,
			ConnectedAt:       time.Unix(2, 0),
			ExpiryInterval:    0,
		},
	}
	for _, v := range tt {
		a.Nil(store.Set(v))
	}
	for _, v := range tt {
		sess, err := store.Get(v.ClientID)
		a.Nil(err)
		a.EqualValues(v, sess)
	}
	var sess []*gmqtt.Session
	err := store.Iterate(func(session *gmqtt.Session) bool {
		sess = append(sess, session)
		return true
	})
	a.Nil(err)
	a.ElementsMatch(sess, tt)
}
