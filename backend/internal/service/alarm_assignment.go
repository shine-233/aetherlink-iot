// 文件用途：告警指派的 service（ROADMAP TB-1 第二片）。
// 核心逻辑：指派 / 取消指派（各写一条流水）、按告警倒序列出流水。
// 关键注意事项：
//  1. 租户归属一律取**告警自身的 tenant_id**，不取 claims.TenantID ——
//     SYS_ADMIN 的 claims.TenantID 为空，取 claims 会写出 tenant_id='' 的行，
//     直接撞表上的 CHECK 约束（这条约束是刻意留着的，别去放宽它）。
//  2. 读写前都必须先过 ensureAlarmHistoryReadAccess：它同时校验告警存在性与租户边界，
//     漏掉就会出现"能把别的租户的告警指派出去"。
//  3. 被指派人必须存在**且属于该告警所在租户**。跨租户指派不是"多给一个权限"，
//     而是把告警内容暴露给了本租户之外的人，一律拒绝（返回 404 而不是 403：
//     对调用方来说"这个人不在这个租户里"等于"查无此人"，不该泄露存在性）。
//  4. 取消指派 = 新写一行 assignee_user_id = NULL，**不改不删**历史行。
//     重复指派同一个人是合法操作（语义是再次确认），不报错、不合并。
//
// 重构建议：若后续要支持"指派给团队/角色"，把 assignee 拆成 (kind, id) 两列并另加 CHECK，
// 不要用特殊前缀 ID 来表达类型。
package service

import (
	"strings"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/google/uuid"
)

const alarmAssignmentNotFoundMessage = "assignee user not found"

// CreateAlarmAssignment 指派或取消指派一条告警历史，返回新写的流水行。
func (*Alarm) CreateAlarmAssignment(req *model.CreateAlarmAssignmentReq, claims *utils.UserClaims) (interface{}, error) {
	if req == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "request cannot be empty")
	}

	// 先确认调用者能看到这条告警，再动指派 —— 否则可以靠猜 alarm_history_id
	// 去探测/改写别的租户的告警归属。
	history, err := ensureAlarmHistoryReadAccess(req.AlarmHistoryID, claims)
	if err != nil {
		return nil, err
	}

	// 租户取告警自身，避免 SYS_ADMIN（TenantID 为空）写出空租户行。
	tenantID := history.TenantID

	var assignee *string
	if req.AssigneeUserID != nil {
		userID := strings.TrimSpace(*req.AssigneeUserID)
		// 空串冒充"取消指派"是被 CHECK 与服务层双重拒绝的：NULL 与空串必须可分，
		// 否则 `IS NULL` 判定当前处理人会失效。
		if userID == "" {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "assignee_user_id must be null or a non-empty user id")
		}
		if err := ensureAssigneeInTenant(userID, tenantID); err != nil {
			return nil, err
		}
		assignee = &userID
	}

	row := &model.AlarmAssignment{
		ID:             uuid.New().String(),
		TenantID:       tenantID,
		AlarmHistoryID: history.ID,
		AssigneeUserID: assignee,
		OperatorUserID: claims.ID,
		Remark:         trimAlarmAssignmentRemark(req.Remark),
		CreatedAt:      time.Now().UTC(),
	}
	if err := dal.CreateAlarmAssignment(row); err != nil {
		return nil, wrapAlarmDBError(err)
	}
	return row, nil
}

// ListAlarmAssignments 列出某条告警的指派流水，时间倒序（首行即当前处理人）。
func (*Alarm) ListAlarmAssignments(req *model.ListAlarmAssignmentsReq, claims *utils.UserClaims) (interface{}, error) {
	if req == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "request cannot be empty")
	}
	history, err := ensureAlarmHistoryReadAccess(req.AlarmHistoryID, claims)
	if err != nil {
		return nil, err
	}
	rows, err := dal.ListAlarmAssignments(history.TenantID, history.ID)
	if err != nil {
		return nil, wrapAlarmDBError(err)
	}
	if rows == nil {
		rows = []*model.AlarmAssignment{}
	}
	return map[string]interface{}{"list": rows}, nil
}

// ensureAssigneeInTenant 校验被指派人存在且属于告警所在租户。
func ensureAssigneeInTenant(userID, tenantID string) error {
	userTenantID, found, err := dal.GetUserTenantIDByID(userID)
	if err != nil {
		return wrapAlarmDBError(err)
	}
	if !found || userTenantID != tenantID {
		// 404 而非 403：跨租户的用户对本租户来说就是"查无此人"，
		// 返回 403 等于确认"这个 ID 存在，只是不在你租户里"。
		return errcode.NewWithMessage(errcode.CodeNotFound, alarmAssignmentNotFoundMessage)
	}
	return nil
}

// trimAlarmAssignmentRemark 归一化备注：空白串当没填（存 NULL），其余去首尾空白。
func trimAlarmAssignmentRemark(remark *string) *string {
	if remark == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*remark)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
