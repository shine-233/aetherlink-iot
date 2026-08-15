// 文件用途: 覆盖 MQTT AUTH 控制包的序列化和解析行为。
// 核心逻辑: 构造字节样例、解码控制包，并验证编码/解码往返一致性。
// 关键注意事项: 测试样例需要与 MQTT v5 属性和原因码规则保持一致。
// 重构建议: 增加表驱动的畸形包用例，补齐边界覆盖。
package packets

import (
	"bytes"
	"testing"

	"github.com/DrmagicE/gmqtt/pkg/codes"
	"github.com/stretchr/testify/assert"
)

func TestReadWriteAuthPacket(t *testing.T) {
	tt := []struct {
		testname   string
		code       codes.Code
		properties *Properties
		want       []byte
	}{
		{
			testname:   "omit properties when code = 0",
			code:       codes.Success,
			properties: nil,
			want:       []byte{0xF0, 0},
		},
		{
			testname: "code = 0 with properties",
			code:     codes.Success,
			properties: &Properties{
				ReasonString: []byte("a"),
			},
			want: []byte{0xF0, 6, 0, 4, 0x1F, 0, 1, 'a'},
		}, {
			testname:   "code != 0 with properties",
			code:       codes.NotAuthorized,
			properties: &Properties{},
			want:       []byte{0xF0, 2, codes.NotAuthorized, 0},
		},
	}

	for _, v := range tt {
		t.Run(v.testname, func(t *testing.T) {
			a := assert.New(t)
			b := make([]byte, 0, 2048)
			buf := bytes.NewBuffer(b)
			au := &Auth{
				Properties: v.properties,
				Code:       v.code,
			}
			err := NewWriter(buf).WriteAndFlush(au)
			a.Nil(err)
			a.Equal(v.want, buf.Bytes())

			bufr := bytes.NewBuffer(buf.Bytes())
			p, err := NewReader(bufr).ReadPacket()
			a.Nil(err)
			rp := p.(*Auth)

			a.Equal(v.code, rp.Code)
			a.Equal(v.properties, rp.Properties)

		})
	}

}
