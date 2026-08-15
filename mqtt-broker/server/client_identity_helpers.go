// 文件用途：集中生成 broker 为匿名 MQTT 客户端分配的内部 ClientID。
// 核心逻辑：沿用 Mongo ObjectID 风格的 12 字节结构，组合时间戳、主机指纹、进程号和递增计数。
// 使用注意：该格式影响空 ClientID 的兼容行为，不能随意改成 UUIDv4 或更长字符串，避免破坏已有会话识别假设。
// 重构建议：如果后续要提升跨进程唯一性，应先补空 ClientID 重连与 session resume 的 focused broker 用例。
package server

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sync/atomic"
	"time"
)

var pid = os.Getpid()
var counter uint32
var machineID = readMachineID()

func readMachineID() []byte {
	id := make([]byte, 3)
	hostname, err1 := os.Hostname()
	if err1 != nil {
		_, err2 := io.ReadFull(rand.Reader, id)
		if err2 != nil {
			panic(fmt.Errorf("cannot get hostname: %v; %v", err1, err2))
		}
		return id
	}
	hw := md5.New()
	hw.Write([]byte(hostname))
	copy(id, hw.Sum(nil))
	return id
}

func getRandomUUID() string {
	var b [12]byte
	// 4 字节秒级时间戳，保持旧实现的大端编码。
	binary.BigEndian.PutUint32(b[:], uint32(time.Now().Unix()))
	// 3 字节主机指纹，来自 hostname 的 md5 前缀。
	b[4] = machineID[0]
	b[5] = machineID[1]
	b[6] = machineID[2]
	// 2 字节进程号；协议未约束端序，这里沿用旧实现的大端写法。
	b[7] = byte(pid >> 8)
	b[8] = byte(pid)
	// 3 字节递增计数，降低同秒内生成重复 ID 的概率。
	i := atomic.AddUint32(&counter, 1)
	b[9] = byte(i >> 16)
	b[10] = byte(i >> 8)
	b[11] = byte(i)
	return fmt.Sprintf(`%x`, string(b[:]))
}
