// 文件用途：集中定义文件上传链路的路径常量（./files 存储根与 OTA 下载对外前缀）。
// 核心逻辑：api/upload.go 的落盘/对外路径语义与 service/media_library.go 的删除路径
//
//	解析必须共用同一份常量，避免"磁盘路径"与"对外路径"两套口径漂移。
//
// 关键注意事项：调整任一常量都会影响已落盘文件的可达性与媒体删除映射，需同步迁移与前端。
//
// 重构建议：若后续支持对象存储（S3/OSS），把路径语义抽成接口而不是继续加常量分支。
package common

const (
	// BaseUploadDir 上传文件的真实存储根目录（相对后端工作目录）。
	BaseUploadDir = "./files/"
	// OtaPath OTA 升级包对外暴露的下载路由前缀（磁盘真实位置仍在 BaseUploadDir 下）。
	OtaPath = "./api/v1/ota/download/files/"
	// DefaultBinaryMime 未知扩展名媒体登记使用的兜底 MIME。
	DefaultBinaryMime = "application/octet-stream"
)
