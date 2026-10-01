// report_schedule_domain.go 定义定时报表域的接口，是 Wave7-C（GroupApp → 构造注入）
// 的第一个域样板：api 层只依赖本接口，不再直接引用 service.GroupApp。
// 核心逻辑：ReportScheduleService 的全部公开方法在此收敛为 ReportScheduleDomain；
// 组装点（api/enter.go 的 Controllers 初始化）用 GroupApp.ReportSchedule 完成唯一一次赋值。
// 关键注意事项：
//  1. 接口方法签名必须与实现逐字一致（值接收者即可满足，无需指针）；
//  2. Controller 持有本接口时字段名必须用 ReportScheduleSvc——Controller 同时嵌入
//     ReportScheduleApi，其 handler 方法与接口方法同名（CreateReportSchedule 等），
//     同名嵌入会直接编译冲突；
//  3. 后续域按同一模式推进：接口文件 + Controller XxxSvc 字段 + 组装点一行赋值。
package service

import (
	"context"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/utils"
)

// ReportScheduleDomain 是定时报表域对 api 层暴露的能力集合。
type ReportScheduleDomain interface {
	CreateReportSchedule(ctx context.Context, req *model.CreateReportScheduleRequest, claims *utils.UserClaims) (*model.ReportSchedule, error)
	UpdateReportSchedule(ctx context.Context, id string, req *model.UpdateReportScheduleRequest, claims *utils.UserClaims) (*model.ReportSchedule, error)
	DeleteReportSchedule(ctx context.Context, id string, revision int64, claims *utils.UserClaims) error
	ListReportSchedules(ctx context.Context, req model.ReportScheduleListRequest, claims *utils.UserClaims) (*model.ReportScheduleListResponse, error)
	GetReportSchedule(ctx context.Context, id string, claims *utils.UserClaims) (*model.ReportSchedule, error)
	SubmitManualRun(ctx context.Context, id, key string, claims *utils.UserClaims) (*model.ReportRunActionResponse, error)
	SubmitRetry(ctx context.Context, id, runID, key string, claims *utils.UserClaims) (*model.ReportRunActionResponse, error)
	ListRuns(ctx context.Context, id string, req model.ReportRunListRequest, claims *utils.UserClaims) (*model.ReportRunListResponse, error)
	GetRun(ctx context.Context, id, runID string, claims *utils.UserClaims) (*model.ReportRunResponse, error)
}

// 编译期断言：ReportScheduleService 必须持续满足域接口。
var _ ReportScheduleDomain = ReportScheduleService{}
