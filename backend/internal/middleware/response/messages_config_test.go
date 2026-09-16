// 文件用途：用**仓库真实**的 configs/messages.yaml 锁定 CSV 导入错误文案契约（ROADMAP P0.5 剩余任务 1）。
// 核心逻辑：100006 必须能把 csv_row 与 message 一起渲染出来；100005 的存量行为必须一字不变。
// 关键注意事项：本测试刻意读真实配置文件而非内联 YAML——本次缺陷恰恰是"模板吞掉子原因"，
// 只测内联模板会漏掉真实模板写错的情况（例如有人把 100005 模板改回只插值 ${field}）。
// 静态审查建议：若后续把错误码迁到独立文件，请同步更新 configPath 的相对路径。
package response

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"aetherlink-iot/backend/pkg/errcode"

	"github.com/stretchr/testify/require"
)

func realConfigManager(t *testing.T) *errcode.ErrorManager {
	t.Helper()
	codePath := filepath.Join("..", "..", "..", "configs", "messages.yaml")
	strPath := filepath.Join("..", "..", "..", "configs", "messages_str.yaml")
	manager := errcode.NewErrorManager(codePath, strPath)
	require.NoError(t, manager.LoadMessages(), "真实错误码配置必须可加载")
	return manager
}

// render 复刻 Handler.resolveMessage 的渲染链：取模板 → 变量替换。
func render(manager *errcode.ErrorManager, code int, lang string, vars map[string]interface{}) string {
	msg := manager.GetMessage(code, lang)
	if len(vars) > 0 {
		msg = replaceVariables(msg, vars)
	}
	return msg
}

// 坏行反馈必须同时带出行号与真实原因。
// 行号断言与浏览器 E2E（e2e/28_p05_preregister_csv.spec.js）的 /\b2\b/ 对齐，
// 保证"后端渲染出 2"这一环在单测层就被钉住，而不是等浏览器跑完整条链路才暴露。
func TestRealConfigRendersCsvRowAndReason(t *testing.T) {
	manager := realConfigManager(t)
	vars := map[string]interface{}{
		"csv_row": 2,
		"message": "device_number and name are required",
	}

	for _, lang := range []string{"zh-CN", "en-US"} {
		msg := render(manager, 100006, lang, vars)
		require.Contains(t, msg, "device_number and name are required", "lang=%s 必须透出真实原因", lang)
		require.Regexp(t, regexp.MustCompile(`\b2\b`), msg, "lang=%s 必须带出行号 2", lang)
		require.NotContains(t, msg, "不能为空", "lang=%s 不得回退成字段为空文案", lang)
		require.NotContains(t, msg, "${", "lang=%s 不得残留未替换的占位符", lang)
	}
}

func TestRealConfigRendersCsvFileLevelReason(t *testing.T) {
	manager := realConfigManager(t)

	msg := render(manager, 100007, "zh-CN", map[string]interface{}{"message": "empty csv"})
	require.Contains(t, msg, "empty csv")
	require.NotContains(t, msg, "不能为空", "文件级错误不得渲染成字段为空")
	require.NotContains(t, msg, "${")

	header := render(manager, 100007, "zh-CN", map[string]interface{}{
		"message": "csv header must be device_number,name (actual: sn,name)",
	})
	require.Contains(t, header, "sn,name", "表头错误必须回显实际表头")
}

// 回归护栏（一）：100005 是 17 处共享的「字段不能为空」，行为必须一字不变。
func TestRealConfigKeepsGenericEmptyFieldTemplate(t *testing.T) {
	manager := realConfigManager(t)

	require.Equal(t, "batch_file不能为空", render(manager, 100005, "zh-CN", map[string]interface{}{"field": "batch_file"}))
	require.Equal(t, "device_count cannot be empty", render(manager, 100005, "en-US", map[string]interface{}{"field": "device_count"}))
}

// 回归护栏（二）：**刻意不做**"模板优先使用调用方 message"的全局覆盖。
// 100002 被 board.go / device_group.go 等处以 message 形式传入具体原因，
// 而 tests/17_api_boundary_smoke.test.js 与 tests/helpers/casbin_fixtures.js
// 硬断言了 100002 的通用文案「请求参数验证失败」。若在此处引入全局覆盖，这两处会立刻变红。
// 因此 100005/100002 这类共享码的模板保持不变，逐行/文件级原因走独立的 100006/100007。
func TestRealConfigKeepsSharedCodesUnchangedEvenWithCallerMessage(t *testing.T) {
	manager := realConfigManager(t)
	vars := map[string]interface{}{
		"field":   "batch_file",
		"message": "device_number and name are required",
	}

	require.Equal(t, "batch_file不能为空", render(manager, 100005, "zh-CN", vars))
	require.Equal(t, "请求参数验证失败", render(manager, 100002, "zh-CN", map[string]interface{}{
		"message": "start_time must be less than or equal to end_time",
	}))
}

// 两个新码必须都是合法错误码（validateCode 在加载期已校验，此处做显式回归锚点）。
func TestRealConfigRegistersCsvErrorCodes(t *testing.T) {
	manager := realConfigManager(t)

	for _, code := range []int{100006, 100007} {
		msg := manager.GetMessage(code, "zh-CN")
		require.NotEmpty(t, msg)
		require.NotEqual(t, "未知错误", msg, "错误码 %d 必须已在 messages.yaml 登记", code)
	}
	require.True(t, strings.Contains(manager.GetMessage(100006, "en-US"), "Row"), "100006 英文模板应含 Row")
}
