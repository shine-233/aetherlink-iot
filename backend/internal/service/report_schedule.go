// 文件用途：定时报表（ROADMAP D3）服务层。
// 核心逻辑：维护报表任务 CRUD（租户隔离），并提供由 cron 调度调用的到期扫描与 CSV 生成 + 邮件投递。
// 关键注意事项：
//   - 邮件投递复用 D2 渠道 sendEmailMessageForDevices，依赖 SMTP 配置；无 SMTP 时仅记录 last_status。
//   - 租户隔离在查询层强制（GetTelemetryDataForReport 带 tenant_id 过滤），定时任务后台运行时按任务记录的 tenant_id 执行。
//   - 到期判定采用“补跑”语义：next = sched.Next(lastRunAt)，若 next 已过且 lastRunAt 早于 next 则执行，避免漏跑与重复跑。
package service

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
	"github.com/google/uuid"
	"github.com/robfig/cron"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const (
	// reportScheduleMaxRowsPerSeries 单 (设备,测点) 序列导出行数上限，防止超大数据集撑爆内存。
	reportScheduleMaxRowsPerSeries = 100000
)

// ReportScheduleService 定时报表业务服务。
type ReportScheduleService struct{}

// CreateReportSchedule 创建定时报表任务，默认回看 24h、格式 csv、启用。
func (ReportScheduleService) CreateReportSchedule(req *model.CreateReportScheduleReq, claims *utils.UserClaims) (*model.ReportSchedule, error) {
	if err := validateCronExpr(req.CronExpr); err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "invalid cron_expr: "+err.Error())
	}
	now := time.Now()
	s := &model.ReportSchedule{
		ID:            uuid.New().String(),
		TenantID:      claims.TenantID,
		Name:          req.Name,
		CronExpr:      req.CronExpr,
		Recipients:    req.Recipients,
		DeviceIDs:     req.DeviceIDs,
		Keys:          req.Keys,
		LookbackHours: req.LookbackHours,
		Format:        req.Format,
		Enabled:       req.Enabled,
		LastRunAt:     &now, // 播种为创建时间，避免创建后立刻被 cron 补跑
		LastStatus:    "pending",
	}
	if s.LookbackHours == 0 {
		s.LookbackHours = 24
	}
	if s.Format == "" {
		s.Format = "csv"
	}
	if err := dal.CreateReportSchedule(s); err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return s, nil
}

// UpdateReportSchedule 部分更新报表任务。
func (ReportScheduleService) UpdateReportSchedule(req *model.UpdateReportScheduleReq, claims *utils.UserClaims) (*model.ReportSchedule, error) {
	existing, err := dal.GetReportScheduleInTenant(req.ID, claims.TenantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "report schedule not found")
		}
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.CronExpr != "" {
		if err := validateCronExpr(req.CronExpr); err != nil {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "invalid cron_expr: "+err.Error())
		}
		existing.CronExpr = req.CronExpr
	}
	if req.Recipients != "" {
		existing.Recipients = req.Recipients
	}
	if req.DeviceIDs != nil {
		existing.DeviceIDs = req.DeviceIDs
	}
	if req.Keys != nil {
		existing.Keys = req.Keys
	}
	if req.LookbackHours != 0 {
		existing.LookbackHours = req.LookbackHours
	}
	if req.Format != "" {
		existing.Format = req.Format
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if err := dal.UpdateReportSchedule(existing); err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return existing, nil
}

// DeleteReportSchedule 删除本租户的报表任务。
func (ReportScheduleService) DeleteReportSchedule(id string, claims *utils.UserClaims) error {
	if err := dal.DeleteReportSchedule(id, claims.TenantID); err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return nil
}

// ListReportSchedules 列出本租户全部报表任务。
func (ReportScheduleService) ListReportSchedules(claims *utils.UserClaims) ([]*model.ReportSchedule, error) {
	list, err := dal.ListReportSchedules(claims.TenantID, 200)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return list, nil
}

// GetReportSchedule 获取单条报表任务。
func (ReportScheduleService) GetReportSchedule(id string, claims *utils.UserClaims) (*model.ReportSchedule, error) {
	s, err := dal.GetReportScheduleInTenant(id, claims.TenantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "report schedule not found")
		}
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return s, nil
}

// RunNow 手动立即触发一次报表生成与投递（供调试/验证，不等待 cron）。
func (ReportScheduleService) RunNow(id string, claims *utils.UserClaims) error {
	s, err := dal.GetReportScheduleInTenant(id, claims.TenantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.NewWithMessage(errcode.CodeParamError, "report schedule not found")
		}
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	ReportScheduleService{}.ExecuteSchedule(s)
	return nil
}

// ScanAndExecuteDueSchedules 由 cron 每分钟调用：扫描启用任务，执行到期者（补跑语义）。
func (ReportScheduleService) ScanAndExecuteDueSchedules() {
	schedules, err := dal.ListEnabledReportSchedules()
	if err != nil {
		logrus.WithError(err).Warn("【定时报表】扫描启用任务失败")
		return
	}
	now := time.Now()
	for _, s := range schedules {
		sched, perr := parseCronExpr(s.CronExpr)
		if perr != nil {
			logrus.Warnf("【定时报表】任务 %s cron_expr %q 非法: %v", s.ID, s.CronExpr, perr)
			dal.UpdateReportScheduleRunResult(s.ID, now, "invalid_cron")
			continue
		}
		last := s.LastRunAt
		if last == nil {
			t := s.CreatedAt
			last = &t
		}
		next := sched.Next(*last)
		if next.After(now) {
			continue // 尚未到触发时间
		}
		// 去重：本周期已执行过（lastRunAt 不早于 next）则跳过
		if s.LastRunAt != nil && !s.LastRunAt.Before(next) {
			continue
		}
		ReportScheduleService{}.ExecuteSchedule(s)
	}
}

// ExecuteSchedule 生成指定作用域的遥测 CSV 并经邮件投递，落库执行结果。
func (ReportScheduleService) ExecuteSchedule(s *model.ReportSchedule) {
	now := time.Now()
	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF") // UTF-8 BOM，便于 Excel 正确识别中文表头
	w := csv.NewWriter(&buf)
	if err := w.Write([]string{"时间", "设备ID", "测点", "数值"}); err != nil {
		dal.UpdateReportScheduleRunResult(s.ID, now, "csv_error")
		return
	}

	startMs := now.Add(-time.Duration(s.LookbackHours) * time.Hour).Unix() * 1000
	endMs := now.Unix() * 1000
	total := 0
	for _, dev := range s.DeviceIDs {
		for _, key := range s.Keys {
			rows, qerr := dal.GetTelemetryDataForReport(s.TenantID, dev, key, startMs, endMs, reportScheduleMaxRowsPerSeries)
			if qerr != nil {
				logrus.WithError(qerr).Warnf("【定时报表】任务 %s 设备 %s 测点 %s 查询失败", s.ID, dev, key)
				continue
			}
			for _, r := range rows {
				t := time.Unix(0, r.T*int64(time.Millisecond))
				if werr := w.Write([]string{t.Format("2006-01-02 15:04:05.000"), dev, key, historyTelemetryValueToString(r)}); werr != nil {
					dal.UpdateReportScheduleRunResult(s.ID, now, "csv_error")
					return
				}
				total++
			}
		}
	}
	w.Flush()
	if w.Error() != nil {
		dal.UpdateReportScheduleRunResult(s.ID, now, "csv_error")
		return
	}

	recipients := splitRecipients(s.Recipients)
	if len(recipients) == 0 {
		dal.UpdateReportScheduleRunResult(s.ID, now, "no_recipient")
		return
	}

	subject := fmt.Sprintf("[AetherLink 定时报表] %s (%s)", s.Name, now.Format("2006-01-02 15:04"))
	body := fmt.Sprintf("报表「%s」已生成，近 %d 小时共 %d 行遥测数据。\n\n%s", s.Name, s.LookbackHours, total, buf.String())
	if err := sendEmailMessageForDevices(body, subject, s.TenantID, s.DeviceIDs, recipients...); err != nil {
		logrus.WithError(err).Warnf("【定时报表】任务 %s 邮件投递失败", s.ID)
		dal.UpdateReportScheduleRunResult(s.ID, now, "email_failed")
		return
	}
	dal.UpdateReportScheduleRunResult(s.ID, now, fmt.Sprintf("ok rows=%d", total))
}

// parseCronExpr 兼容 5 段（标准）与 6 段（含秒）cron 表达式。
func parseCronExpr(expr string) (cron.Schedule, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, fmt.Errorf("empty expression")
	}
	fields := strings.Fields(expr)
	if len(fields) == 5 {
		return cron.ParseStandard(expr)
	}
	return cron.Parse(expr)
}

func validateCronExpr(expr string) error {
	_, err := parseCronExpr(expr)
	return err
}

// splitRecipients 按逗号/分号/空白拆分收件人，去空。
func splitRecipients(s string) []string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\n' || r == '\t'
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
