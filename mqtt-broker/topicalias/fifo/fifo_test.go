// 文件用途：验证 FIFO topic alias 管理器的分配、命中和淘汰复用行为。
// 核心逻辑：构造固定 alias 上限，先填满队列，再检查已有 topic 命中和新 topic 复用最早 alias。
// 关键注意事项：测试当前覆盖正常上限场景，尚未覆盖 maxAlias 为 0 或异常配置。
// 重构建议：后续可改用表驱动测试补齐边界值，并去除未实际使用的 mock controller 依赖。
package fifo

import (
	"strconv"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/DrmagicE/gmqtt/config"
	"github.com/DrmagicE/gmqtt/pkg/packets"
)

func TestQueue(t *testing.T) {
	a := assert.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cid := "clientID"
	max := uint16(10)
	q := New(config.DefaultConfig(), max, cid).(*Queue)
	for i := uint16(1); i <= max; i++ {
		alias, ok := q.Check(&packets.Publish{
			TopicName: []byte(strconv.Itoa(int(i))),
		})
		a.Equal(i, alias)
		a.False(ok)
	}
	alias := uint16(1)
	for e := q.topicAlias.alias.Front(); e != nil; e = e.Next() {
		elem := e.Value.(*aliasElem)
		a.Equal(alias, elem.alias)
		a.Equal(strconv.Itoa(int(alias)), elem.topic)
		alias++
	}
	a.Equal(10, q.topicAlias.alias.Len())

	// alias exist
	alias, ok := q.Check(&packets.Publish{TopicName: []byte("1")})
	a.True(ok)
	a.EqualValues(1, alias)

	alias, ok = q.Check(&packets.Publish{TopicName: []byte("not exist")})
	a.False(ok)
	a.EqualValues(1, alias)

}
