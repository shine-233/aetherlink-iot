// 文件用途：提供 API 层的公共入口能力，包括控制器聚合、请求绑定、结构体校验和 WebSocket 升级基础设施。
// 核心流程：路由命中具体 Handler 后，会优先复用本文件中的 BindAndValidate 完成 query/body 绑定与 validator 校验，再把参数错误转换成统一 errcode。
// 兼容边界：这里只处理“入参适配”和“公共基础设施”，不接管鉴权、租户边界、业务错误码细分或最终响应封装，避免和中间件、service 层职责重叠。
// 静态审查建议：若后续继续拆分控制器，优先把绑定分发、校验消息格式化和错误上报抽成更细粒度的 helper，
// 同时评估 WebSocket CheckOrigin 常开策略、Delete 请求仅走 JSON 绑定的兼容性，以及 validator 中文错误提示是否需要统一字段别名。
package api

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"aetherlink-iot/backend/pkg/errcode"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/gorilla/websocket"
)

type Controller struct {
	UserApi
	DictApi
	OTAApi
	UpLoadApi
	ProtocolPluginApi
	DeviceApi
	DeviceDebugApi
	DeviceModelApi
	UiElementsApi
	BoardApi
	TelemetryDataApi
	AttributeDataApi
	EventDataApi
	CommandSetLogApi
	OperationLogsApi
	LogoApi
	DataPolicyApi
	DeviceConfigApi
	DataScriptApi
	RoleApi
	CasbinApi
	NotificationGroupApi
	NotificationHistoryApi
	NotificationServicesConfigApi
	AlarmApi
	SceneAutomationsApi
	SceneApi
	SystemApi
	SysFunctionApi
	ServicePluginApi
	ServiceAccessApi
	ExpectedDataApi
	OpenAPIKeyApi
	MessagePushApi
	SystemMonitorApi
	DeviceAuthApi
	DeviceTopicMappingApi
	FleetSavedFilterApi
	DashboardMenuApi
	DeviceTwinApi
	DeviceShadowApi
	DeviceModbusProfileApi
	AiQueryApi
	RuleChainApi
	RDIApi
	PayloadSchemaApi
	CalculatedFieldApi
	ProductApi
	EntityVersionApi
}

var (
	// Controllers 作为 API 分组聚合根，供路由注册阶段统一引用具体 Handler。
	Controllers = new(Controller)
	// Validate 是全局复用的 validator 实例，负责结构体标签校验。
	Validate *validator.Validate
)

func init() {
	Validate = validator.New()
}

// ValidateStruct 对请求结构体执行字段校验，并返回第一条可读错误。
func ValidateStruct(i interface{}) error {
	return ValidateStructLang(i, "")
}

// ValidateStructLang 按 Accept-Language 生成可读校验错误。空 lang 时默认中文。
func ValidateStructLang(i interface{}, lang string) error {
	err := Validate.Struct(i)
	if err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			return err
		}

		var messages []string
		for _, validationErr := range err.(validator.ValidationErrors) {
			messages = append(messages, validationErrorToText(validationErr, lang))
		}

		return fmt.Errorf("%s", messages[0])
	}
	return nil
}

// isEnglishLang 判断 Accept-Language 是否优先英文（如 "en", "en-US", "en-US;q=0.9,..."）。
func isEnglishLang(lang string) bool {
	if lang == "" {
		return false
	}
	normalized := strings.ToLower(errcode.NormalizeLanguage(strings.SplitN(lang, ",", 2)[0]))
	return strings.HasPrefix(normalized, "en")
}

// validationErrorToText 将 validator 的字段错误转换为面向调用方的说明，按 lang 决定中英文。
func validationErrorToText(e validator.FieldError, lang string) string {
	if isEnglishLang(lang) {
		switch e.Tag() {
		case "required":
			return fmt.Sprintf("Field '%s' is required", e.Field())
		case "email":
			return fmt.Sprintf("Field '%s' must be a valid email address", e.Field())
		case "gte":
			return fmt.Sprintf("The value of field '%s' must be at least %s", e.Field(), e.Param())
		case "lte":
			return fmt.Sprintf("The value of field '%s' must be at most %s", e.Field(), e.Param())
		default:
			return fmt.Sprintf("Field '%s' failed validation (%s)", e.Field(), validationErrorHint(e, lang))
		}
	}
	switch e.Tag() {
	case "required":
		return fmt.Sprintf("字段 %q 为必填项", e.Field())
	case "email":
		return fmt.Sprintf("字段 %q 必须是有效的邮箱地址", e.Field())
	case "gte":
		return fmt.Sprintf("字段 %q 的值不能小于 %s", e.Field(), e.Param())
	case "lte":
		return fmt.Sprintf("字段 %q 的值不能大于 %s", e.Field(), e.Param())
	default:
		return fmt.Sprintf("字段 %q 未通过校验（%s）", e.Field(), validationErrorHint(e, lang))
	}
}

// validationErrorHint 提供更短的规则说明，避免把内部 validator 标签直接暴露给调用方。
func validationErrorHint(e validator.FieldError, lang string) string {
	unit := ""
	switch e.Kind() {
	case reflect.String:
		unit = "characters"
	case reflect.Array, reflect.Slice, reflect.Map:
		unit = "items"
	}

	if isEnglishLang(lang) {
		switch e.Tag() {
		case "min":
			if unit != "" {
				return fmt.Sprintf("At least %s %s", e.Param(), unit)
			}
			return fmt.Sprintf("Must be at least %s", e.Param())
		case "max":
			if unit != "" {
				return fmt.Sprintf("At most %s %s", e.Param(), unit)
			}
			return fmt.Sprintf("Must be at most %s", e.Param())
		case "len":
			if unit != "" {
				return fmt.Sprintf("Must contain exactly %s %s", e.Param(), unit)
			}
			return fmt.Sprintf("Must equal %s", e.Param())
		case "oneof":
			return fmt.Sprintf("Must be one of: %s", strings.ReplaceAll(e.Param(), " ", ", "))
		default:
			return "Does not meet validation rules"
		}
	}

	chineseUnit := ""
	switch unit {
	case "characters":
		chineseUnit = "个字符"
	case "items":
		chineseUnit = "项"
	}
	switch e.Tag() {
	case "min":
		if chineseUnit != "" {
			return fmt.Sprintf("至少需要 %s %s", e.Param(), chineseUnit)
		}
		return fmt.Sprintf("必须至少为 %s", e.Param())
	case "max":
		if chineseUnit != "" {
			return fmt.Sprintf("最多 %s %s", e.Param(), chineseUnit)
		}
		return fmt.Sprintf("必须至多为 %s", e.Param())
	case "len":
		if chineseUnit != "" {
			return fmt.Sprintf("必须正好包含 %s %s", e.Param(), chineseUnit)
		}
		return fmt.Sprintf("必须等于 %s", e.Param())
	case "oneof":
		return fmt.Sprintf("必须是以下值之一：%s", strings.ReplaceAll(e.Param(), " ", "、"))
	default:
		return "不符合校验规则"
	}
}

// ApiResponse 定义统一的 HTTP 返回结构。
type ApiResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// reportParamError 将绑定或校验错误转换成统一的参数错误码并挂到 Gin 上下文。
func reportParamError(c *gin.Context, err error) {
	c.Error(errcode.NewWithMessage(errcode.CodeParamError, err.Error()))
}

// bindRequest 根据 HTTP 方法选择 query 或 JSON 绑定策略。
// 使用注意：当前 DELETE 请求也按 JSON body 处理，若后续出现 query/path 组合删除接口，需要额外评估这里的分发规则。
func bindRequest(c *gin.Context, obj interface{}) error {
	switch c.Request.Method {
	case http.MethodGet:
		return c.ShouldBindQuery(obj)
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return c.ShouldBindJSON(obj)
	default:
		return nil
	}
}

// BindAndValidate 根据请求方法完成绑定和校验，失败时把参数错误写入 Gin 上下文。
func BindAndValidate(c *gin.Context, obj interface{}) bool {
	if err := bindRequest(c, obj); err != nil {
		reportParamError(c, err)
		return false
	}

	if err := ValidateStructLang(obj, c.GetHeader("Accept-Language")); err != nil {
		reportParamError(c, err)
		return false
	}

	return true
}

var Wsupgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(_ *http.Request) bool {
		// 兼容现有跨域握手行为：当前不额外限制来源。
		// 静态审查建议：若后续 WebSocket 承载更敏感数据，建议把来源校验下沉为可配置策略。
		return true
	},
}
