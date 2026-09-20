// 文件用途：从设备物模型链路两跳解析遥测键定义的物理源单位（ROADMAP TB-9）。
// 链路：devices -> device_configs -> device_template_id -> device_model_telemetry.unit。
package dal

import (
	"context"
	"strings"

	"aetherlink-iot/backend/pkg/global"
)

// ResolveDeviceTelemetryUnit 尝试从设备物模型中解析指定遥测标识符的单位符号。
// 若未找到或数据库未初始化，返回 ("", nil)，由上层决定是否降级或说明原因。
func ResolveDeviceTelemetryUnit(ctx context.Context, deviceID, key string) (string, error) {
	if global.DB == nil {
		return "", nil
	}
	deviceID = strings.TrimSpace(deviceID)
	key = strings.TrimSpace(key)
	if deviceID == "" || key == "" {
		return "", nil
	}

	var unit *string
	query := `
		SELECT dmt.unit
		FROM devices d
		JOIN device_configs dc ON d.device_config_id = dc.id
		JOIN device_model_telemetry dmt ON dc.device_template_id = dmt.device_template_id
		WHERE d.id = ? AND dmt.data_identifier = ?
		LIMIT 1
	`
	err := global.DB.WithContext(ctx).Raw(query, deviceID, key).Scan(&unit).Error
	if err != nil {
		return "", err
	}
	if unit == nil {
		return "", nil
	}
	return strings.TrimSpace(*unit), nil
}
