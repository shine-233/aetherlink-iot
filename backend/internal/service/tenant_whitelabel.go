// 文件用途：白标服务（TB-47）——租户翻译覆盖与自定义 CSS 的业务编排层。
// 核心逻辑：入参校验（lang 白名单 / key 形态 / value 与 css 内容约束）纯函数化、
//
//	claims→租户作用域解析（SYS_ADMIN 全局空串、TENANT_ADMIN 本租户、TENANT_USER
//	只读）、CRUD 分发到 dal，以及"登录后可读覆盖获取"的响应装配。
//
// 关键注意事项：
//  1. 校验与合并纯函数（NormalizeTenantTranslationLang / ValidateTenantTranslationKey /
//     ValidateTenantTranslationValue / SanitizeTenantCustomCSS /
//     ApplyTranslationOverrides / BuildTenantWhitelabelOverrides）是唯一判定点，
//     前端契约测试与后续服务端渲染（如邮件模板）应复用同一口径，不得另写一份。
//  2. 租户作用域 fail-closed：TENANT_USER 无写权限；TENANT_ADMIN 租户上下文为空
//     直接拒绝；SYS_ADMIN 作用域为空串=系统全局行（语义同 logo 表）。
//  3. 自定义 CSS 的最终 XSS 防线是前端 textContent 注入（禁止 innerHTML）；
//     SanitizeTenantCustomCSS 的 '</style' 拒绝是纵深防御，不是替代。
//
// 重构建议：若需要"全局行向租户级联合并"，在 GetWhitelabelOverrides 装配处叠加
//
//	全局行查询 + ApplyTranslationOverrides 两段合并，不要在 SQL 层做 UNION。
package service

import (
	"context"
	"strings"
	"time"
	"unicode"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// WhitelabelService 白标（翻译覆盖 + 自定义 CSS）服务。
type WhitelabelService struct{}

// SupportedTenantTranslationLangs 翻译覆盖允许的语言白名单：
// 与前端静态四语言目录（locales/langs/*）一一对应，覆盖项只能落在这些语言上。
var SupportedTenantTranslationLangs = []string{"zh-cn", "en-us", "es-es", "fr-fr"}

// MaxTenantCustomCSSLength 自定义 CSS 长度上限（与列约束/请求校验一致的 64KiB）。
const MaxTenantCustomCSSLength = 65536

// MaxTenantTranslationsPerRequest 单次批量写入/删除条数上限（fail-closed，防批量滥用）。
const MaxTenantTranslationsPerRequest = 500

// IsSupportedTenantTranslationLang 判断（已规范化的）语言是否在白名单内。
func IsSupportedTenantTranslationLang(lang string) bool {
	for _, supported := range SupportedTenantTranslationLangs {
		if lang == supported {
			return true
		}
	}
	return false
}

// NormalizeTenantTranslationLang 规范化语言标签：去空白 + 转小写（与前端 locale
// 目录的小写连字符形态对齐），不在白名单时返回 ("", false)（fail-closed）。
func NormalizeTenantTranslationLang(lang string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(lang))
	if !IsSupportedTenantTranslationLang(normalized) {
		return "", false
	}
	return normalized, true
}

// ValidateTenantTranslationKey 校验词条键形态：仅允许字母/数字/./-/_，1..200 字符
// （与列宽一致）；拒绝空白、首尾点与连续点；拒绝斜杠等路径字符，
// 从源头封死按 key 拼接路径/选择器的可能。
func ValidateTenantTranslationKey(key string) error {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" || len(trimmed) > 200 {
		return errcode.NewWithMessage(errcode.CodeTenantTranslationInvalid, "translation key must be 1-200 characters")
	}
	if trimmed != key {
		return errcode.NewWithMessage(errcode.CodeTenantTranslationInvalid, "translation key must not contain leading/trailing whitespace")
	}
	if strings.HasPrefix(trimmed, ".") || strings.HasSuffix(trimmed, ".") || strings.Contains(trimmed, "..") {
		return errcode.NewWithMessage(errcode.CodeTenantTranslationInvalid, "translation key must not start/end with '.' or contain '..'")
	}
	for _, r := range trimmed {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r == '.' || r == '-' || r == '_':
		default:
			return errcode.NewWithMessage(errcode.CodeTenantTranslationInvalid, "translation key only allows letters, digits and '.-_'")
		}
	}
	return nil
}

// ValidateTenantTranslationValue 校验译文：1..4000 字符（按 rune 计），
// 禁止换行/回车/制表之外的控制字符（防不可见字符注入 UI）。
func ValidateTenantTranslationValue(value string) error {
	runes := []rune(value)
	if len(runes) == 0 || len(runes) > 4000 {
		return errcode.NewWithMessage(errcode.CodeTenantTranslationInvalid, "translation value must be 1-4000 characters")
	}
	for _, r := range runes {
		if r == '\n' || r == '\r' || r == '\t' {
			continue
		}
		if unicode.IsControl(r) {
			return errcode.NewWithMessage(errcode.CodeTenantTranslationInvalid, "translation value must not contain control characters")
		}
	}
	return nil
}

// SanitizeTenantCustomCSS 清洗自定义 CSS：去首尾空白、限长 64KiB、
// 拒绝 '</style' 序列（大小写不敏感，纵深防御：即使前端误用 innerHTML 也无法
// 提前闭合 style 标签注入标记）、拒绝换行/回车/制表之外的控制字符。
// 空串合法（语义=清除自定义样式）。
func SanitizeTenantCustomCSS(css string) (string, error) {
	trimmed := strings.TrimSpace(css)
	if trimmed == "" {
		return "", nil
	}
	if len(trimmed) > MaxTenantCustomCSSLength {
		return "", errcode.NewWithMessage(errcode.CodeTenantCustomCSSInvalid, "custom css must not exceed 65536 characters")
	}
	if strings.Contains(strings.ToLower(trimmed), "</style") {
		return "", errcode.NewWithMessage(errcode.CodeTenantCustomCSSInvalid, "custom css must not contain '</style' sequences")
	}
	for _, r := range trimmed {
		if r == '\n' || r == '\r' || r == '\t' {
			continue
		}
		if unicode.IsControl(r) {
			return "", errcode.NewWithMessage(errcode.CodeTenantCustomCSSInvalid, "custom css must not contain control characters")
		}
	}
	return trimmed, nil
}

// ApplyTranslationOverrides 把租户覆盖合并到基础词条表：覆盖项优先，返回新 map，
// 不改写入参（合并语义的唯一判定点；base/overrides 同键时 overrides 胜出）。
func ApplyTranslationOverrides(base, overrides map[string]string) map[string]string {
	merged := make(map[string]string, len(base)+len(overrides))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range overrides {
		merged[k] = v
	}
	return merged
}

// BuildTenantWhitelabelOverrides 把翻译行按 lang 分组装配为覆盖获取端点出参。
// 纯函数：rows 为空时 Translations 为空 map（保证 JSON 输出为 {} 而非 null）。
func BuildTenantWhitelabelOverrides(tenantID string, rows []*model.TenantTranslation, css string) *model.TenantWhitelabelOverrides {
	translations := make(map[string]map[string]string, len(rows))
	for _, row := range rows {
		langMap, ok := translations[row.Lang]
		if !ok {
			langMap = make(map[string]string, 8)
			translations[row.Lang] = langMap
		}
		langMap[row.Key] = row.Value
	}
	return &model.TenantWhitelabelOverrides{
		TenantID:     tenantID,
		Translations: translations,
		CSS:          css,
	}
}

// whitelabelWriteScope 解析写操作的租户作用域：
// SYS_ADMIN → claims.TenantID（空串=系统全局行）；TENANT_ADMIN → 本租户（必须非空）；
// 其余角色（含 TENANT_USER）与缺凭证一律拒绝（fail-closed）。
func whitelabelWriteScope(claims *utils.UserClaims) (string, error) {
	if claims == nil {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, "missing user claims")
	}
	switch claims.Authority {
	case constant.SYS_ADMIN:
		return claims.TenantID, nil
	case constant.TENANT_ADMIN:
		if strings.TrimSpace(claims.TenantID) == "" {
			return "", errcode.NewWithMessage(errcode.CodeNoPermission, "complete tenant initialization before updating white-label settings")
		}
		return claims.TenantID, nil
	default:
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to update white-label settings")
	}
}

// whitelabelReadScope 解析读操作的租户作用域：任何登录用户可读本作用域（自身租户
// 或 SYS_ADMIN 的全局行）的覆盖；缺凭证拒绝（"登录后可读"语义）。
func whitelabelReadScope(claims *utils.UserClaims) (string, error) {
	if claims == nil {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, "missing user claims")
	}
	return claims.TenantID, nil
}

// UpsertTranslations 批量写入翻译覆盖：逐条校验 lang/key/value，同请求内键重复
// 以后者为准，落库为单条 UPSERT 语句（同键覆盖 value）。返回实际写入条数。
func (*WhitelabelService) UpsertTranslations(ctx context.Context, req *model.UpsertTenantTranslationsReq, claims *utils.UserClaims) (int, error) {
	tenantID, err := whitelabelWriteScope(claims)
	if err != nil {
		return 0, err
	}
	if req == nil || len(req.Items) == 0 {
		return 0, errcode.NewWithMessage(errcode.CodeParamError, "items must not be empty")
	}
	if len(req.Items) > MaxTenantTranslationsPerRequest {
		return 0, errcode.NewWithMessage(errcode.CodeParamError, "items must not exceed 500 per request")
	}

	now := time.Now()
	rows := make([]*model.TenantTranslation, 0, len(req.Items))
	// seen 记录 (lang,key)→rows 下标，同请求内重复键按"后者为准"原地覆盖对应行。
	seen := make(map[string]int, len(req.Items))
	for i := range req.Items {
		item := req.Items[i]
		lang, ok := NormalizeTenantTranslationLang(item.Lang)
		if !ok {
			return 0, errcode.WithVars(errcode.CodeTenantTranslationInvalid, map[string]interface{}{
				"index": i, "reason": "unsupported language (allowed: zh-cn/en-us/es-es/fr-fr)",
			})
		}
		if err := ValidateTenantTranslationKey(item.Key); err != nil {
			return 0, errcode.WithVars(errcode.CodeTenantTranslationInvalid, map[string]interface{}{
				"index": i, "reason": "invalid key",
			})
		}
		if err := ValidateTenantTranslationValue(item.Value); err != nil {
			return 0, errcode.WithVars(errcode.CodeTenantTranslationInvalid, map[string]interface{}{
				"index": i, "reason": "invalid value",
			})
		}
		row := &model.TenantTranslation{
			ID: uuid.NewString(), TenantID: tenantID, Lang: lang, Key: item.Key, Value: item.Value, UpdatedAt: &now,
		}
		dedupKey := lang + "\x00" + item.Key
		if idx, dup := seen[dedupKey]; dup {
			rows[idx] = row
			continue
		}
		seen[dedupKey] = len(rows)
		rows = append(rows, row)
	}

	if err := dal.UpsertTenantTranslations(ctx, rows); err != nil {
		logrus.Error(err)
		return 0, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"err": err.Error()})
	}
	return len(rows), nil
}

// ListTranslations 管理面列表：返回本作用域全部（或按 lang 过滤）翻译覆盖行。
// lang 携带时先按白名单规范化，非法语言直接报参数错误（不静默返回空列表）。
func (*WhitelabelService) ListTranslations(ctx context.Context, lang string, claims *utils.UserClaims) (map[string]interface{}, error) {
	tenantID, err := whitelabelWriteScope(claims)
	if err != nil {
		return nil, err
	}
	normalizedLang := ""
	if trimmed := strings.TrimSpace(lang); trimmed != "" {
		normalized, ok := NormalizeTenantTranslationLang(trimmed)
		if !ok {
			return nil, errcode.NewWithMessage(errcode.CodeTenantTranslationInvalid, "unsupported language filter (allowed: zh-cn/en-us/es-es/fr-fr)")
		}
		normalizedLang = normalized
	}
	list, err := dal.ListTenantTranslations(ctx, tenantID, normalizedLang)
	if err != nil {
		logrus.Error(err)
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"err": err.Error()})
	}
	return map[string]interface{}{"total": len(list), "list": list}, nil
}

// DeleteTranslations 批量删除翻译覆盖，返回实际删除行数。
func (*WhitelabelService) DeleteTranslations(ctx context.Context, req *model.DeleteTenantTranslationsReq, claims *utils.UserClaims) (int64, error) {
	tenantID, err := whitelabelWriteScope(claims)
	if err != nil {
		return 0, err
	}
	if req == nil || len(req.Items) == 0 {
		return 0, errcode.NewWithMessage(errcode.CodeParamError, "items must not be empty")
	}
	if len(req.Items) > MaxTenantTranslationsPerRequest {
		return 0, errcode.NewWithMessage(errcode.CodeParamError, "items must not exceed 500 per request")
	}

	keys := make([]model.TenantTranslationKeyItem, 0, len(req.Items))
	for i := range req.Items {
		item := req.Items[i]
		lang, ok := NormalizeTenantTranslationLang(item.Lang)
		if !ok {
			return 0, errcode.WithVars(errcode.CodeTenantTranslationInvalid, map[string]interface{}{
				"index": i, "reason": "unsupported language (allowed: zh-cn/en-us/es-es/fr-fr)",
			})
		}
		if err := ValidateTenantTranslationKey(item.Key); err != nil {
			return 0, errcode.WithVars(errcode.CodeTenantTranslationInvalid, map[string]interface{}{
				"index": i, "reason": "invalid key",
			})
		}
		keys = append(keys, model.TenantTranslationKeyItem{Lang: lang, Key: item.Key})
	}

	deleted, err := dal.DeleteTenantTranslations(ctx, tenantID, keys)
	if err != nil {
		logrus.Error(err)
		return 0, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"err": err.Error()})
	}
	return deleted, nil
}

// GetWhitelabelOverrides 登录后可读的覆盖获取：返回本作用域全部翻译覆盖（按语言
// 分组）与自定义 CSS。任何登录角色可调用（TENANT_USER 读自身租户，SYS_ADMIN 读全局行）。
func (*WhitelabelService) GetWhitelabelOverrides(ctx context.Context, claims *utils.UserClaims) (*model.TenantWhitelabelOverrides, error) {
	tenantID, err := whitelabelReadScope(claims)
	if err != nil {
		return nil, err
	}
	rows, err := dal.ListTenantTranslations(ctx, tenantID, "")
	if err != nil {
		logrus.Error(err)
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"err": err.Error()})
	}
	cssRow, err := dal.GetTenantCustomCSS(ctx, tenantID)
	if err != nil {
		logrus.Error(err)
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"err": err.Error()})
	}
	css := ""
	if cssRow != nil {
		css = cssRow.CSS
	}
	return BuildTenantWhitelabelOverrides(tenantID, rows, css), nil
}

// GetCustomCSS 管理面读取自定义 CSS（回显编辑框；无行返回空串）。
func (*WhitelabelService) GetCustomCSS(ctx context.Context, claims *utils.UserClaims) (map[string]interface{}, error) {
	tenantID, err := whitelabelWriteScope(claims)
	if err != nil {
		return nil, err
	}
	row, err := dal.GetTenantCustomCSS(ctx, tenantID)
	if err != nil {
		logrus.Error(err)
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"err": err.Error()})
	}
	css := ""
	var updatedAt *time.Time
	if row != nil {
		css = row.CSS
		updatedAt = row.UpdatedAt
	}
	return map[string]interface{}{"css": css, "updated_at": updatedAt}, nil
}

// UpsertCustomCSS 写入自定义 CSS（清洗后 UPSERT；空串=清除）。
func (*WhitelabelService) UpsertCustomCSS(ctx context.Context, req *model.UpsertTenantCustomCSSReq, claims *utils.UserClaims) error {
	tenantID, err := whitelabelWriteScope(claims)
	if err != nil {
		return err
	}
	if req == nil {
		return errcode.NewWithMessage(errcode.CodeParamError, "css payload is required")
	}
	css, err := SanitizeTenantCustomCSS(req.CSS)
	if err != nil {
		return err
	}
	if err := dal.UpsertTenantCustomCSS(ctx, tenantID, css); err != nil {
		logrus.Error(err)
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{"err": err.Error()})
	}
	return nil
}
