// 文件用途：提供后端通用字符串、JSON、错误包装、响应 payload 和随机码辅助函数。
// 核心逻辑：用标准库完成序列化、随机数生成和格式拼装，并复用常量包保持基础判断一致。
// 关键注意事项：这里不应承载租户、权限或设备业务规则；随机值用于业务标识时需确认长度和碰撞风险。
// 重构建议：后续可按 JSON、随机数、响应体和错误包装拆分文件，降低公共包职责密度。
package common

import (
	constant "aetherlink-iot/backend/pkg/constant"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"time"

	"github.com/pkg/errors"
)

// CheckEmpty 判断字符串是否为空
func CheckEmpty(str string) bool {
	return str == constant.EMPTY
}

// GetMessageID 基于当前 Unix 时间戳后七位生成消息ID
func GetMessageID() string {
	// 获取当前Unix时间戳
	timestamp := time.Now().Unix()
	// 将时间戳转换为字符串
	timestampStr := strconv.FormatInt(timestamp, 10)
	// 截取后七位
	messageID := timestampStr[len(timestampStr)-7:]

	return messageID
}

// JsonToString 将任意值序列化为 JSON 字符串
func JsonToString(any any) (string, error) {
	data, err := json.Marshal(any)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// GetErrors 为已有错误附加描述信息
func GetErrors(err error, message string) error {
	return errors.WithMessage(err, message)
}

// GetResponsePayload 构造 MQTT 主题响应内容
// 成功示例：{"result":0,"message":"success","ts":1609143039}
// 失败示例：{"result":1,"errcode":"000","message":"xxxxxx","ts":1609143039}
func GetResponsePayload(method string, err error) []byte {
	if err != nil {
		data := map[string]interface{}{
			"result":  1,
			"errcode": "000",
			"message": err.Error(),
			"ts":      time.Now().Unix(),
		}
		res, _ := json.Marshal(data)
		return res
	}
	data := map[string]interface{}{
		"result":  0,
		"message": "success",
		"ts":      time.Now().Unix(),
	}
	if method != "" {
		data["method"] = method
	}
	res, _ := json.Marshal(data)
	return res
}

// StringSpt 返回字符串的指针
func StringSpt(str string) *string {
	return &str
}

// IsStringEmpty 判断字符串指针是否为空或指向空字符串
func IsStringEmpty(str *string) bool {
	return str == nil || *str == ""
}

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// GenerateRandomString 生成指定长度的随机字母数字字符串
func GenerateRandomString(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	for i := range b {
		b[i] = charset[b[i]%byte(len(charset))]
	}
	return string(b), nil
}

var ErrNoRows = errors.New("record not found")

// GetRandomNineDigits 生成 [100000000, 999999999] 范围内的随机九位数字字符串
func GetRandomNineDigits() (string, error) {
	// 生成 [100000000, 999999999] 范围内的随机数
	min := big.NewInt(100000000)
	max := big.NewInt(999999999)

	// 计算范围大小
	diff := new(big.Int).Sub(max, min)
	diff = diff.Add(diff, big.NewInt(1))

	// 生成随机数
	n, err := rand.Int(rand.Reader, diff)
	if err != nil {
		return "", fmt.Errorf("生成随机数失败: %w", err)
	}

	// 加上最小值以确保在正确范围内
	n = n.Add(n, min)

	// 转换为字符串
	return n.String(), nil
}

// GenerateNumericCode 生成指定长度的随机数字验证码
func GenerateNumericCode(length int) (string, error) {
	if length <= 0 {
		return "", fmt.Errorf("长度必须大于0")
	}

	// 构建验证码
	code := make([]byte, length)

	for i := 0; i < length; i++ {
		// 生成 0-9 的随机数
		num, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", fmt.Errorf("生成随机数字失败: %w", err)
		}

		// 转换为字符并添加到验证码中
		code[i] = byte(num.Int64() + '0')
	}

	return string(code), nil
}
