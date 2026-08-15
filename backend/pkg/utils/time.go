// 文件用途：提供 time 相关的后端通用工具能力。
// 核心逻辑：封装可复用的格式处理、校验、加密、脚本或命令构造逻辑，供业务层按需调用，主要围绕 func GetUTCTime、func GetSecondTimestamp、func IsToday、func DaysAgo 等声明展开。
// 关键注意事项：工具函数常被多个模块共享，修改需保持入参约束、返回值和错误语义兼容。
// 重构建议：后续可按职责继续拆分工具包，减少无关工具之间的隐式耦合。

package utils

import "time"

// 时间相关
func GetUTCTime() time.Time {
	return time.Now().UTC()
}

func GetSecondTimestamp() int64 {
	return time.Now().Unix()
}

func IsToday(t time.Time) bool {
	now := time.Now()
	return t.Year() == now.Year() &&
		t.Month() == now.Month() &&
		t.Day() == now.Day()
}

func DaysAgo(n int) time.Time {
	now := time.Now()
	past := now.AddDate(0, 0, -n)
	return past
}

func MillisecondsTimestampDaysAgo(n int) int64 {
	// 获取当前时间
	now := time.Now()
	// 计算n天前的时间
	past := now.AddDate(0, 0, -n)
	// 转换为毫秒时间戳
	milliseconds := past.UnixNano() / int64(time.Millisecond)
	return milliseconds
}
