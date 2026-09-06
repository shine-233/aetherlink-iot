package grpcgateway

import (
	"bytes"
	"encoding/json"
	"fmt"

	"google.golang.org/grpc/encoding"
)

// PHASE-D-D9 BEGIN JSON 编解码（无 protoc 环境的契约载体）
//
// Codec 名 aetherjson：服务端与客户端双侧显式指定；
// 编码禁 HTML 转义，保证遥测载荷可读性（与平台 webhook 契约一致）。

// CodecName 编解码注册名。
const CodecName = "aetherjson"

func init() {
	encoding.RegisterCodec(jsonCodec{})
}

type jsonCodec struct{}

func (jsonCodec) Name() string { return CodecName }

func (jsonCodec) Marshal(v any) ([]byte, error) {
	if v == nil {
		return []byte("{}"), nil
	}
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSpace(buffer.Bytes()), nil
}

func (jsonCodec) Unmarshal(data []byte, v any) error {
	if len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("aetherjson unmarshal: %w", err)
	}
	return nil
}

// PHASE-D-D9 END
