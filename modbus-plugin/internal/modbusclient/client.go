// 文件用途：Modbus TCP 客户端封装（ROADMAP B1）。
// 核心逻辑：基于 grid-x/modbus 的按目标懒连接客户端，提供点表读值与可写点写入。
// 关键注意事项：单目标串行化（Modbus 事务不能并发）；f32/u32/i32 按大端字序解码；
//   读值缩放 = raw*Multiplier+Offset，写值逆变换；input/discrete 只读。
package modbusclient

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"sync"
	"time"

	"github.com/grid-x/modbus"

	"github.com/shine-233/aetherlink-iot/modbus-plugin/internal/config"
)

// Client 单个 Modbus TCP 从站连接封装（内部互斥，保证事务串行）。
type Client struct {
	mu     sync.Mutex
	target config.TargetConfig
	client modbus.Client
}

// NewClient 创建客户端（不立即建连）。
func NewClient(target config.TargetConfig) *Client {
	return &Client{target: target}
}

func (c *Client) acquire(ctx context.Context) (modbus.Client, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.client != nil {
		return c.client, nil
	}
	addr := net.JoinHostPort(c.target.Host, fmt.Sprint(c.target.Port))
	handler := modbus.NewTCPClientHandler(addr)
	handler.Timeout = time.Duration(c.target.TimeoutMs) * time.Millisecond
	handler.SlaveID = c.target.UnitID
	if err := handler.Connect(ctx); err != nil {
		return nil, fmt.Errorf("modbus connect %s: %w", addr, err)
	}
	c.client = modbus.NewClient(handler)
	return c.client, nil
}

func (c *Client) release(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err != nil && c.client != nil {
		// 连接级错误后丢弃底层连接，下次重连。
		if handler, ok := c.client.(interface{ Close() error }); ok {
			_ = handler.Close()
		} else if closer, ok := any(c.client).(interface{ Close() }); ok {
			closer.Close()
		}
		c.client = nil
	}
}

// ReadPoint 读取单个点位并按缩放返回数值/布尔。
func (c *Client) ReadPoint(ctx context.Context, r *config.RegisterPoint) (any, error) {
	values, err := c.readRaw(ctx, r)
	if err != nil {
		return nil, err
	}
	value := decodeValue(r, values)
	if isBoolType(r.Type) {
		return value, nil
	}
	rawFloat, _ := value.(float64)
	return rawFloat*r.Multiplier + r.Offset, nil
}

// WritePoint 向可写点位写值（逆缩放后下发）。
func (c *Client) WritePoint(ctx context.Context, r *config.RegisterPoint, value float64) error {
	if !r.Writable {
		return fmt.Errorf("register %q is not writable", r.Key)
	}
	switch r.Type {
	case "coil":
		raw := value != 0
		return c.writeCoil(ctx, r.Address, raw)
	case "holding":
		return c.writeHolding(ctx, r, value)
	default:
		return fmt.Errorf("register type %q is read-only", r.Type)
	}
}

func (c *Client) writeHolding(ctx context.Context, r *config.RegisterPoint, value float64) error {
	scaled := (value - r.Offset) / r.Multiplier
	words, err := encodeWords(r.DataType, scaled)
	if err != nil {
		return err
	}
	cli, err := c.acquire(ctx)
	if err != nil {
		return err
	}
	if len(words) == 1 {
		_, err = cli.WriteSingleRegister(ctx, r.Address, words[0])
	} else {
		_, err = cli.WriteMultipleRegisters(ctx, r.Address, uint16(len(words)), wordsToBytes(words))
	}
	c.release(err)
	return err
}

func (c *Client) writeCoil(ctx context.Context, address uint16, on bool) error {
	cli, err := c.acquire(ctx)
	if err != nil {
		return err
	}
	value := uint16(0)
	if on {
		value = 1
	}
	_, err = cli.WriteSingleCoil(ctx, address, value)
	c.release(err)
	return err
}

func (c *Client) readRaw(ctx context.Context, r *config.RegisterPoint) ([]byte, error) {
	count := r.RegisterCount()
	cli, err := c.acquire(ctx)
	if err != nil {
		return nil, err
	}
	var (
		result []byte
		readEr error
	)
	switch r.Type {
	case "holding":
		result, readEr = cli.ReadHoldingRegisters(ctx, r.Address, count)
	case "input":
		result, readEr = cli.ReadInputRegisters(ctx, r.Address, count)
	case "coil":
		result, readEr = cli.ReadCoils(ctx, r.Address, 1)
	case "discrete":
		result, readEr = cli.ReadDiscreteInputs(ctx, r.Address, 1)
	default:
		c.release(fmt.Errorf("unsupported type"))
		return nil, fmt.Errorf("unsupported register type %q", r.Type)
	}
	c.release(readEr)
	if readEr != nil {
		return nil, readEr
	}
	return result, nil
}

func decodeValue(r *config.RegisterPoint, raw []byte) any {
	switch r.Type {
	case "coil", "discrete":
		return len(raw) > 0 && raw[0] != 0
	}
	switch r.DataType {
	case "u16":
		if len(raw) >= 2 {
			return float64(binary.BigEndian.Uint16(raw[:2]))
		}
	case "i16":
		if len(raw) >= 2 {
			return float64(int16(binary.BigEndian.Uint16(raw[:2])))
		}
	case "u32":
		if len(raw) >= 4 {
			return float64(binary.BigEndian.Uint32(raw[:4]))
		}
	case "i32":
		if len(raw) >= 4 {
			return float64(int32(binary.BigEndian.Uint32(raw[:4])))
		}
	case "f32":
		if len(raw) >= 4 {
			return float64(math.Float32frombits(binary.BigEndian.Uint32(raw[:4])))
		}
	}
	return float64(0)
}

func encodeWords(dataType string, value float64) ([]uint16, error) {
	switch dataType {
	case "u16":
		if value < 0 || value > math.MaxUint16 {
			return nil, fmt.Errorf("value %v out of u16 range", value)
		}
		return []uint16{uint16(value)}, nil
	case "i16":
		if value < math.MinInt16 || value > math.MaxInt16 {
			return nil, fmt.Errorf("value %v out of i16 range", value)
		}
		return []uint16{uint16(int16(value))}, nil
	case "u32":
		if value < 0 || value > math.MaxUint32 {
			return nil, fmt.Errorf("value %v out of u32 range", value)
		}
		raw := make([]byte, 4)
		binary.BigEndian.PutUint32(raw, uint32(value))
		return []uint16{binary.BigEndian.Uint16(raw[:2]), binary.BigEndian.Uint16(raw[2:])}, nil
	case "i32":
		if value < math.MinInt32 || value > math.MaxInt32 {
			return nil, fmt.Errorf("value %v out of i32 range", value)
		}
		raw := make([]byte, 4)
		binary.BigEndian.PutUint32(raw, uint32(int32(value)))
		return []uint16{binary.BigEndian.Uint16(raw[:2]), binary.BigEndian.Uint16(raw[2:])}, nil
	case "f32":
		raw := make([]byte, 4)
		binary.BigEndian.PutUint32(raw, math.Float32bits(float32(value)))
		return []uint16{binary.BigEndian.Uint16(raw[:2]), binary.BigEndian.Uint16(raw[2:])}, nil
	default:
		return nil, fmt.Errorf("unsupported data_type %q", dataType)
	}
}

func wordsToBytes(words []uint16) []byte {
	out := make([]byte, 0, len(words)*2)
	for _, w := range words {
		out = append(out, byte(w>>8), byte(w))
	}
	return out
}

func isBoolType(registerType string) bool {
	return registerType == "coil" || registerType == "discrete"
}
