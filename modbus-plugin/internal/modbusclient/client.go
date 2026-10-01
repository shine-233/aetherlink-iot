// 文件用途：Modbus TCP 客户端封装（ROADMAP B1）。
// 核心逻辑：基于 grid-x/modbus 的按目标懒连接客户端，提供点表读值与可写点写入，
//
//	以及按地址分组的批量读入口（PlanBatches/ReadBatches），减少整轮采集的 TCP 事务数。
//
// 关键注意事项：单目标串行化（Modbus 事务不能并发）；f32/u32/i32 按大端字序解码；
//
//	读值缩放 = raw*Multiplier+Offset，写值逆变换；input/discrete 只读；
//	批量读遵循 Modbus 协议上限（寄存器类 125 个/次，位类 2000 位/次）。
package modbusclient

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"sort"
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
		c.closeLocked()
	}
}

// closeLocked 关闭并丢弃缓存的底层连接（调用方必须已持有 c.mu）。
func (c *Client) closeLocked() {
	if c.client == nil {
		return
	}
	if handler, ok := c.client.(interface{ Close() error }); ok {
		_ = handler.Close()
	} else if closer, ok := any(c.client).(interface{ Close() }); ok {
		closer.Close()
	}
	c.client = nil
}

// Close 主动关闭底层 Modbus TCP 连接（优雅停机时调用）。
// 说明：连接复用期间库会在空闲 60s 后自动断开并在下次事务透明重连，
// 但停机路径不应依赖 OS 回收——显式关闭同时停掉库内的 idle close 定时器。
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closeLocked()
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

// Modbus 协议上限：FC3/4 单次最多 125 个寄存器；FC1/2 单次最多 2000 位。
// 批量分组按这两个上限切分，避免拼出协议不允许的超长请求。
const (
	maxRegistersPerRequest = 125
	maxBitsPerRequest      = 2000
	// defaultGapThreshold 是合并相邻点位时允许跨越的"空洞"寄存器数上限；
	// 超过该间隔仍合并会把大量无用寄存器塞进一次事务，得不偿失。
	defaultGapThreshold = 4
)

// Batch 是一组可以用单次 Modbus 事务读取的连续（或近邻）点位。
// Start/Count 是该事务请求的起始地址与寄存器（或位）数量；Points 保留原始点位，
// 解码时按各自 Address 相对 Start 的偏移从批量结果里切回字节/位段。
type Batch struct {
	Type   string // holding | input | coil | discrete
	Start  uint16
	Count  uint16
	Points []*config.RegisterPoint
}

// PlanBatches 把点表按类型分组，再按地址排序贪心合并为尽量少的批量请求。
// 同类型下地址区间 gap（当前点结束地址到下一点起始地址的距离）不超过 gapThreshold
// 的点位会被合并进同一个 Batch；超过阈值或超过协议单次请求上限则另起一组。
// gapThreshold<0 时使用 defaultGapThreshold。调用方（poller）应在设备点表确定后
// 预计算一次分组表并复用，避免每轮采集重算。
func PlanBatches(points []config.RegisterPoint, gapThreshold int) []Batch {
	if gapThreshold < 0 {
		gapThreshold = defaultGapThreshold
	}
	byType := map[string][]*config.RegisterPoint{}
	for i := range points {
		p := &points[i]
		byType[p.Type] = append(byType[p.Type], p)
	}
	var batches []Batch
	for _, typ := range []string{"holding", "input", "coil", "discrete"} {
		pts := byType[typ]
		if len(pts) == 0 {
			continue
		}
		sort.Slice(pts, func(i, j int) bool { return pts[i].Address < pts[j].Address })
		maxSpan := uint16(maxRegistersPerRequest)
		if typ == "coil" || typ == "discrete" {
			maxSpan = uint16(maxBitsPerRequest)
		}
		batches = append(batches, groupContiguous(typ, pts, uint16(gapThreshold), maxSpan)...)
	}
	return batches
}

// groupContiguous 对同一类型、按地址升序排好的点位做贪心区间合并。
func groupContiguous(typ string, pts []*config.RegisterPoint, gapThreshold, maxSpan uint16) []Batch {
	var batches []Batch
	i := 0
	for i < len(pts) {
		start := pts[i].Address
		end := start + pointSpan(typ, pts[i])
		group := []*config.RegisterPoint{pts[i]}
		j := i + 1
		for j < len(pts) {
			next := pts[j]
			nextEnd := next.Address + pointSpan(typ, next)
			// gap：下一点起始地址与当前已覆盖末端地址的距离；地址重叠（同一地址被多个
			// 点引用，罕见但合法）视为 gap=0，一律可合并，超过阈值则另起一组。
			if next.Address > end && next.Address-end > gapThreshold {
				break
			}
			if span := nextEnd - start; span > maxSpan {
				break
			}
			group = append(group, next)
			if nextEnd > end {
				end = nextEnd
			}
			j++
		}
		batches = append(batches, Batch{
			Type:   typ,
			Start:  start,
			Count:  end - start,
			Points: group,
		})
		i = j
	}
	return batches
}

// pointSpan 返回该点位占用的地址宽度：寄存器类按 RegisterCount()（1 或 2），
// 位类（coil/discrete）固定为 1 位。
func pointSpan(typ string, r *config.RegisterPoint) uint16 {
	switch typ {
	case "coil", "discrete":
		return 1
	default:
		return r.RegisterCount()
	}
}

// ReadBatches 依次执行每个批量请求（单目标仍需串行，但每个 Batch 只用一次 TCP 事务
// 覆盖多个点位），并把结果解码、缩放后写入 out[point.Key]。单个 Batch 失败不中断
// 其余 Batch，调用方可根据日志决定是否降级为逐点读。返回遇到的（可能是多个）错误，
// 便于上层记录失败详情，但不会因此丢弃已成功批次的数据。
func (c *Client) ReadBatches(ctx context.Context, batches []Batch, out map[string]any) []error {
	var errs []error
	for _, b := range batches {
		if err := ctx.Err(); err != nil {
			return append(errs, err)
		}
		raw, err := c.readBatchRaw(ctx, b)
		if err != nil {
			errs = append(errs, fmt.Errorf("batch %s@%d x%d: %w", b.Type, b.Start, b.Count, err))
			continue
		}
		for _, p := range b.Points {
			segment := sliceForPoint(b, p, raw)
			value := decodeValue(p, segment)
			if isBoolType(p.Type) {
				out[p.Key] = value
				continue
			}
			rawFloat, _ := value.(float64)
			out[p.Key] = rawFloat*p.Multiplier + p.Offset
		}
	}
	return errs
}

// readBatchRaw 为一个 Batch 发出单次 Modbus 事务，寄存器类返回原始字节，
// 位类返回打包后的位字节（grid-x/modbus 的 ReadCoils/ReadDiscreteInputs 格式：
// 每字节 8 位，位 n 对应 bit(n%8) of byte(n/8)，与 sliceForPoint 的解包方式对应）。
func (c *Client) readBatchRaw(ctx context.Context, b Batch) ([]byte, error) {
	cli, err := c.acquire(ctx)
	if err != nil {
		return nil, err
	}
	var (
		result []byte
		readEr error
	)
	switch b.Type {
	case "holding":
		result, readEr = cli.ReadHoldingRegisters(ctx, b.Start, b.Count)
	case "input":
		result, readEr = cli.ReadInputRegisters(ctx, b.Start, b.Count)
	case "coil":
		result, readEr = cli.ReadCoils(ctx, b.Start, b.Count)
	case "discrete":
		result, readEr = cli.ReadDiscreteInputs(ctx, b.Start, b.Count)
	default:
		c.release(fmt.Errorf("unsupported type"))
		return nil, fmt.Errorf("unsupported register type %q", b.Type)
	}
	c.release(readEr)
	if readEr != nil {
		return nil, readEr
	}
	return result, nil
}

// sliceForPoint 从批量结果里切出单个点位对应的字节段。
// 寄存器类（holding/input）每寄存器 2 字节，按 (Address-Start)*2 定位。
// 位类（coil/discrete）从打包位里抽取单个比特，还原成 decodeValue 期望的
// "首字节非零即真" 的单字节表示，与原逐点读的 ReadCoils(addr,1) 返回格式一致。
func sliceForPoint(b Batch, p *config.RegisterPoint, raw []byte) []byte {
	offset := p.Address - b.Start
	if b.Type == "coil" || b.Type == "discrete" {
		byteIdx := int(offset) / 8
		bitIdx := uint(offset) % 8
		if byteIdx >= len(raw) {
			return []byte{0}
		}
		if raw[byteIdx]&(1<<bitIdx) != 0 {
			return []byte{1}
		}
		return []byte{0}
	}
	start := int(offset) * 2
	span := int(pointSpan(b.Type, p)) * 2
	if start+span > len(raw) {
		return nil
	}
	return raw[start : start+span]
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
