// 文件用途：提供 lang 相关的后端通用工具能力。
// 核心逻辑：封装可复用的格式处理、校验、加密、脚本或命令构造逻辑，供业务层按需调用，主要围绕 func FormatLangCode 等声明展开。
// 关键注意事项：工具函数常被多个模块共享，修改需保持入参约束、返回值和错误语义兼容。
// 重构建议：后续可按职责继续拆分工具包，减少无关工具之间的隐式耦合。

package utils

import "strings"

func FormatLangCode(acceptLanguage string) string {
	// 如果为空则返回默认值 en_US
	if acceptLanguage == "" {
		return "en_US"
	}

	// 分割 accept-language，取第一个
	langs := strings.Split(acceptLanguage, ",")
	primaryLang := strings.TrimSpace(langs[0])

	// 处理可能的权重值 如 zh-CN;q=0.9
	primaryLang = strings.Split(primaryLang, ";")[0]

	// 替换 - 为 _
	primaryLang = strings.Replace(primaryLang, "-", "_", 1)

	// 处理特殊情况
	switch primaryLang {
	case "zh":
		return "zh_CN"
	case "en":
		return "en_US"
	}

	// 如果已经是正确格式则直接返回
	if len(primaryLang) == 5 && primaryLang[2] == '_' {
		return primaryLang
	}

	// 其他情况返回默认值
	return "zh_CN"
}
