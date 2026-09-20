// 文件用途：模板市场包的签名、依赖自洽检查与导入冲突预览（ROADMAP P1.6）。
//
// 关键注意事项：
//   - 签名密钥未配置或非法一律 **fail closed**：导出拒绝出包、导入拒绝入包。
//     宁可不可用，也不能让一个无法验真的包在租户间流转。
//   - 摘要覆盖"除签名三字段外的规范 JSON"，验签时先比对摘要再比对 HMAC，
//     两者都用常量时间比较，避免通过响应时间侧信道推断。
//   - 冲突预览只读：**不落库、不建模板**，只回答"导入会发生什么"。
//     包内重名与依赖问题一律列为阻断项，由调用方决定是否拒绝。
package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"aetherlink-iot/backend/internal/model"

	"github.com/spf13/viper"
)

// 签名配置键（与 P0.7 的加密主密钥分开，签名与加密不共用同一把钥匙）。
const (
	marketBundleSigningKeysKey = "market.bundle_signing_keys"
	marketActiveSigningKeyKey  = "market.active_bundle_signing_key_id"
)

// 签名相关错误的判定哨兵，便于调用方区分"配置缺失"与"包被篡改"。
var (
	ErrMarketBundleSigningUnconfigured = errors.New("market bundle signing key is not configured")
	ErrMarketBundleUnsigned            = errors.New("market bundle is not signed")
)

// marketBundleSigningKey 读取当前生效的签名密钥；未配置或非法返回错误。
func marketBundleSigningKey() (keyID string, key []byte, err error) {
	keyID = strings.TrimSpace(viper.GetString(marketActiveSigningKeyKey))
	if keyID == "" {
		return "", nil, ErrMarketBundleSigningUnconfigured
	}
	encoded := strings.TrimSpace(viper.GetString(marketBundleSigningKeysKey + "." + keyID))
	if encoded == "" {
		return "", nil, fmt.Errorf("%w: key %q is missing", ErrMarketBundleSigningUnconfigured, keyID)
	}
	raw, decErr := base64.StdEncoding.DecodeString(encoded)
	if decErr != nil {
		return "", nil, fmt.Errorf("market bundle signing key %q is not valid base64: %w", keyID, decErr)
	}
	if len(raw) < 32 {
		return "", nil, fmt.Errorf("market bundle signing key %q must be at least 32 bytes", keyID)
	}
	return keyID, raw, nil
}

// marketBundleCanonical 序列化"待签内容"：排除签名三字段，保证签的是业务内容本身。
func marketBundleCanonical(bundle *model.MarketBundle) ([]byte, error) {
	if bundle == nil {
		return nil, errors.New("market bundle is nil")
	}
	shadow := struct {
		TypeKey    string                        `json:"type_key"`
		ExportedAt int64                         `json:"exported_at"`
		Count      int                           `json:"count"`
		Templates  []*model.DeviceTemplateExport `json:"templates"`
		Boards     []*model.BoardTemplateExport  `json:"boards,omitempty"`
	}{
		TypeKey:    bundle.TypeKey,
		ExportedAt: bundle.ExportedAt,
		Count:      bundle.Count,
		Templates:  bundle.Templates,
		Boards:     bundle.Boards,
	}
	return json.Marshal(shadow)
}

// ComputeMarketBundleDigest 计算内容摘要（hex SHA-256）。
func ComputeMarketBundleDigest(bundle *model.MarketBundle) (string, error) {
	canonical, err := marketBundleCanonical(bundle)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}

// SignMarketBundle 为包计算摘要并签名；密钥未配置或非法一律返回错误。
func SignMarketBundle(bundle *model.MarketBundle) error {
	if bundle == nil {
		return errors.New("market bundle is nil")
	}
	keyID, key, err := marketBundleSigningKey()
	if err != nil {
		return err
	}
	digest, err := ComputeMarketBundleDigest(bundle)
	if err != nil {
		return err
	}
	canonical, err := marketBundleCanonical(bundle)
	if err != nil {
		return err
	}
	mac := hmac.New(sha256.New, key)
	if _, err := mac.Write(canonical); err != nil {
		return fmt.Errorf("sign market bundle failed: %w", err)
	}
	bundle.Digest = digest
	bundle.Signature = hex.EncodeToString(mac.Sum(nil))
	bundle.SignedKeyID = keyID
	return nil
}

// VerifyMarketBundle 验签：未签名、密钥缺失、摘要不符、签名不符一律拒绝。
func VerifyMarketBundle(bundle *model.MarketBundle) error {
	if bundle == nil {
		return errors.New("market bundle is nil")
	}
	if strings.TrimSpace(bundle.Signature) == "" || strings.TrimSpace(bundle.Digest) == "" {
		return ErrMarketBundleUnsigned
	}
	_, key, err := marketBundleSigningKey()
	if err != nil {
		return err
	}
	wantDigest, err := ComputeMarketBundleDigest(bundle)
	if err != nil {
		return err
	}
	// 摘要不符说明内容被改动过：即使签名碰巧对上也不能信。
	if !hmac.Equal([]byte(strings.ToLower(wantDigest)), []byte(strings.ToLower(bundle.Digest))) {
		return fmt.Errorf("market bundle digest mismatch: content was modified after signing")
	}
	canonical, err := marketBundleCanonical(bundle)
	if err != nil {
		return err
	}
	mac := hmac.New(sha256.New, key)
	if _, err := mac.Write(canonical); err != nil {
		return fmt.Errorf("verify market bundle failed: %w", err)
	}
	wantSig := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(strings.ToLower(wantSig)), []byte(strings.ToLower(bundle.Signature))) {
		return fmt.Errorf("market bundle signature is invalid")
	}
	return nil
}

// MarketBundleImportPreview 导入预览。结构与判定方法在 model 层定义（供 API 响应直接引用），
// 这里以别名保持既有调用点与测试的编译兼容。
type MarketBundleImportPreview = model.MarketBundleImportPreview

// CheckMarketBundleDependencies 包内自洽与依赖检查，返回问题列表（空表示通过）。
// 判定口径：
//   - 模板或看板名缺失：导入后无法定位，也无法幂等重放；
//   - 包内模板或看板重名：导入后互相覆盖，最终状态取决于顺序，属不确定行为；
//   - 包声明了行业类型，而模板 type_key 与之不符：包不自洽。
func CheckMarketBundleDependencies(bundle *model.MarketBundle) []string {
	if bundle == nil {
		return []string{"bundle is nil"}
	}
	issues := make([]string, 0, 4)
	seenTemplates := make(map[string]bool, len(bundle.Templates))
	for i, template := range bundle.Templates {
		if template == nil {
			issues = append(issues, fmt.Sprintf("templates[%d] is nil", i))
			continue
		}
		name := strings.TrimSpace(template.Name)
		if name == "" {
			issues = append(issues, fmt.Sprintf("templates[%d] has empty name", i))
			continue
		}
		if seenTemplates[name] {
			issues = append(issues, fmt.Sprintf("duplicate template name %q within bundle", name))
		}
		seenTemplates[name] = true
		if bundle.TypeKey != "" {
			templateTypeKey := ""
			if template.TypeKey != nil {
				templateTypeKey = strings.TrimSpace(*template.TypeKey)
			}
			if templateTypeKey != "" && templateTypeKey != bundle.TypeKey {
				issues = append(issues, fmt.Sprintf("template %q type_key %q does not match bundle type_key %q",
					name, templateTypeKey, bundle.TypeKey))
			}
		}
	}

	seenBoards := make(map[string]bool, len(bundle.Boards))
	for i, board := range bundle.Boards {
		if board == nil {
			issues = append(issues, fmt.Sprintf("boards[%d] is nil", i))
			continue
		}
		name := strings.TrimSpace(board.Name)
		if name == "" {
			issues = append(issues, fmt.Sprintf("boards[%d] has empty name", i))
			continue
		}
		if seenBoards[name] {
			issues = append(issues, fmt.Sprintf("duplicate board name %q within bundle", name))
		}
		seenBoards[name] = true
		if bundle.TypeKey != "" {
			boardTypeKey := ""
			if board.TypeKey != nil {
				boardTypeKey = strings.TrimSpace(*board.TypeKey)
			}
			if boardTypeKey != "" && boardTypeKey != bundle.TypeKey {
				issues = append(issues, fmt.Sprintf("board %q type_key %q does not match bundle type_key %q",
					name, boardTypeKey, bundle.TypeKey))
			}
		}
	}

	totalItems := len(bundle.Templates) + len(bundle.Boards)
	if bundle.Count != totalItems {
		if len(bundle.Boards) == 0 {
			issues = append(issues, fmt.Sprintf("bundle count %d does not match templates length %d",
				bundle.Count, len(bundle.Templates)))
		} else {
			issues = append(issues, fmt.Sprintf("bundle count %d does not match items length %d",
				bundle.Count, totalItems))
		}
	}
	return issues
}

// NormalizeDeviceTemplateVersion 归一化模板版本号：空值一律落到 "1.0.0"。
// 预览与导入必须共用这一份实现——两处各写一份会让"同版本重导"的判定与实际
// 幂等键 (租户, 名称, 版本) 分叉，闸门就会挡住本该无需确认的幂等路径。
func NormalizeDeviceTemplateVersion(version *string) string {
	if version == nil {
		return "1.0.0"
	}
	if v := strings.TrimSpace(*version); v != "" {
		return v
	}
	return "1.0.0"
}

// PreviewMarketBundleImport 预览导入结果（兼容单物模型包调用）；existing 为租户内已有模板的 名称→版本。
func PreviewMarketBundleImport(bundle *model.MarketBundle, existing map[string]string) MarketBundleImportPreview {
	return PreviewResourceBundleImport(bundle, existing, nil)
}

// PreviewResourceBundleImport 资源中心综合预览：支持物模型与大屏看板双重冲突分析。
func PreviewResourceBundleImport(bundle *model.MarketBundle, existingTemplates map[string]string, existingBoards map[string]string) MarketBundleImportPreview {
	preview := MarketBundleImportPreview{
		Create:            make([]string, 0, 4),
		Overwrite:         make([]string, 0, 4),
		Blocking:          CheckMarketBundleDependencies(bundle),
		TemplateCreate:    make([]string, 0, 4),
		TemplateOverwrite: make([]string, 0, 4),
		BoardCreate:       make([]string, 0, 4),
		BoardOverwrite:    make([]string, 0, 4),
	}
	if bundle == nil {
		return preview
	}

	// 1. 判定物模型模板
	for _, template := range bundle.Templates {
		if template == nil {
			continue
		}
		name := strings.TrimSpace(template.Name)
		if name == "" {
			continue
		}
		if existingVersion, ok := existingTemplates[name]; ok {
			if existingVersion == NormalizeDeviceTemplateVersion(template.Version) {
				continue
			}
			preview.Overwrite = append(preview.Overwrite, name)
			preview.TemplateOverwrite = append(preview.TemplateOverwrite, name)
			continue
		}
		preview.Create = append(preview.Create, name)
		preview.TemplateCreate = append(preview.TemplateCreate, name)
	}

	// 2. 判定大屏看板
	for _, board := range bundle.Boards {
		if board == nil {
			continue
		}
		name := strings.TrimSpace(board.Name)
		if name == "" {
			continue
		}
		if existingVersion, ok := existingBoards[name]; ok {
			if existingVersion == NormalizeDeviceTemplateVersion(board.Version) {
				continue
			}
			preview.Overwrite = append(preview.Overwrite, name)
			preview.BoardOverwrite = append(preview.BoardOverwrite, name)
			continue
		}
		preview.Create = append(preview.Create, name)
		preview.BoardCreate = append(preview.BoardCreate, name)
	}

	preview.Total = len(preview.Create) + len(preview.Overwrite)
	return preview
}
