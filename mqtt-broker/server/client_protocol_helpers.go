// 文件用途：集中存放 MQTT 客户端协议层的小型转换工具，避免 client.go 继续累积零散 helper。
// 核心逻辑：处理布尔属性、可空数值属性、错误属性回写和普通 error 到 MQTT Reason Code 的转换。
// 使用注意：这些函数被 CONNECT、SUBSCRIBE、PUBLISH、UNSUBSCRIBE、控制包和 session 生命周期复用，修改时要保持 MQTT v5 兼容语义。
// 重构建议：后续若继续扩展 MQTT v5 属性协商，可把 CONNECT 专属协商 helper 与通用协议 helper 再拆成两个更深模块。
package server

import (
	"github.com/DrmagicE/gmqtt/pkg/codes"
	"github.com/DrmagicE/gmqtt/pkg/packets"
)

func bool2Byte(bo bool) *byte {
	var b byte
	if bo {
		b = 1
	} else {
		b = 0
	}
	return &b
}

func convertUint16(u *uint16, defaultValue uint16) uint16 {
	if u == nil {
		return defaultValue
	}
	return *u
}

func convertUint32(u *uint32, defaultValue uint32) uint32 {
	if u == nil {
		return defaultValue
	}
	return *u
}

func getErrorProperties(client *client, errDetails *codes.ErrorDetails) *packets.Properties {
	if client.version == packets.Version5 && client.opts.RequestProblemInfo && errDetails != nil {
		return &packets.Properties{
			ReasonString: errDetails.ReasonString,
			User:         kvsToProperties(errDetails.UserProperties),
		}
	}
	return nil
}

func converError(err error) *codes.Error {
	if err == nil {
		return nil
	}
	if e, ok := err.(*codes.Error); ok {
		return e
	}
	return &codes.Error{
		Code: codes.UnspecifiedError,
		ErrorDetails: codes.ErrorDetails{
			ReasonString: []byte(err.Error()),
		},
	}
}
