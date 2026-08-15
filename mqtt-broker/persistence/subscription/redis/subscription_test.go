// 文件用途：维护 persistence\subscription\redis\subscription_test.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package redis

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/DrmagicE/gmqtt"
)

func TestEncodeDecodeSubscription(t *testing.T) {
	a := assert.New(t)
	tt := []*gmqtt.Subscription{
		{
			ShareName:         "shareName",
			TopicFilter:       "filter",
			ID:                1,
			QoS:               1,
			NoLocal:           false,
			RetainAsPublished: false,
			RetainHandling:    0,
		}, {
			ShareName:         "",
			TopicFilter:       "abc",
			ID:                0,
			QoS:               2,
			NoLocal:           false,
			RetainAsPublished: true,
			RetainHandling:    1,
		},
	}

	for _, v := range tt {
		b := EncodeSubscription(v)
		sub, err := DecodeSubscription(b)
		a.Nil(err)
		a.Equal(v, sub)
	}
}
