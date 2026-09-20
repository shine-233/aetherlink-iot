// 文件用途：SCADA 实时控制命令的权限、确认与审计（ROADMAP P1.3）。
// 核心逻辑：命令执行前依次过「参数 → 文档可写 → Widget/命令已注册 → 权限 → 二次确认」，
// 过闸后**先落 pending 审计再执行**，执行结果推进为 success/failed。
//
// 关键注意事项（本层全部围绕"不许出现没记录的控制动作"）：
//  1. **被拒绝的命令也必须落审计**。只审计成功命令，等于把"谁在反复尝试越权控制"
//     从记录里抹掉，审计就只剩装饰作用。
//  2. 审计先于执行落库（pending），写不进去就拒绝执行。若改成执行后补记，
//     那么审计写失败的那一刻，控制动作已经发生且无痕——比不做更危险。
//  3. 二次确认令牌由 HMAC 绑定 (租户, 文档, Widget, 命令, 操作人, 过期时间)，
//     换个 Widget 或换个人都不能复用，过期即失效；密钥未配置时既不签发也不放行。
//  4. 命令是否需要确认由 Widget 注册声明决定，不在调用点各自判断，
//     否则同一个危险命令在不同画布上确认要求会不一致。
//  5. 执行器未接线时记 failed 并报错，绝不报 success——没有执行器却成功是本类项目最忌讳的假成功。
package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrConfirmationRequired  = errors.New("confirmation token is required")
	ErrConfirmationInvalid   = errors.New("confirmation token is invalid")
	ErrConfirmationExpired   = errors.New("confirmation token has expired")
	ErrConfirmationNoIssuer  = errors.New("confirmation issuer is not configured")
	ErrControlActorMissing   = errors.New("control actor is required")
	ErrControlNoExecutor     = errors.New("control command executor is not wired")
	ErrControlWidgetUnknown  = errors.New("widget is not registered")
	ErrControlCommandUnknown = errors.New("widget does not declare this command")
	ErrControlAuditLost      = errors.New("control audit could not be recorded; command refused")
)

// ---------------------------------------------------------------------------
// 二次确认令牌
// ---------------------------------------------------------------------------

// ConfirmationIssuer 签发与校验控制命令的二次确认令牌。
type ConfirmationIssuer struct {
	secret []byte
	ttl    time.Duration
}

// NewConfirmationIssuer 创建签发器。
// secret 为空直接报错：没有密钥就无法保证令牌不可伪造，
// 此时"放行"等于取消了二次确认，故 fail closed。
func NewConfirmationIssuer(secret string, ttl time.Duration) (*ConfirmationIssuer, error) {
	if strings.TrimSpace(secret) == "" {
		return nil, ErrConfirmationNoIssuer
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &ConfirmationIssuer{secret: []byte(secret), ttl: ttl}, nil
}

// Issue 签发绑定具体命令动作的令牌。
func (i *ConfirmationIssuer) Issue(tenantID, documentID, widgetID, command, actorUserID string, now time.Time) (string, error) {
	if i == nil || len(i.secret) == 0 {
		return "", ErrConfirmationNoIssuer
	}
	expiresAt := now.Add(i.ttl).Unix()
	return strconv.FormatInt(expiresAt, 10) + "." + i.sign(tenantID, documentID, widgetID, command, actorUserID, expiresAt), nil
}

// Verify 校验令牌：任一维度不匹配、过期或格式非法都返回错误。
func (i *ConfirmationIssuer) Verify(token string, tenantID, documentID, widgetID, command, actorUserID string, now time.Time) error {
	if i == nil || len(i.secret) == 0 {
		return ErrConfirmationNoIssuer
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return ErrConfirmationRequired
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return ErrConfirmationInvalid
	}
	expiresAt, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return ErrConfirmationInvalid
	}
	if now.Unix() > expiresAt {
		return ErrConfirmationExpired
	}
	if !hmac.Equal([]byte(parts[1]), []byte(i.sign(tenantID, documentID, widgetID, command, actorUserID, expiresAt))) {
		return ErrConfirmationInvalid
	}
	return nil
}

// sign 计算规范串的 HMAC-SHA256。
func (i *ConfirmationIssuer) sign(tenantID, documentID, widgetID, command, actorUserID string, expiresAt int64) string {
	canonical := strings.Join([]string{
		strings.TrimSpace(tenantID),
		strings.TrimSpace(documentID),
		strings.TrimSpace(widgetID),
		strings.TrimSpace(command),
		strings.TrimSpace(actorUserID),
		strconv.FormatInt(expiresAt, 10),
	}, "|")
	mac := hmac.New(sha256.New, i.secret)
	mac.Write([]byte(canonical))
	return hex.EncodeToString(mac.Sum(nil))
}

// ---------------------------------------------------------------------------
// 控制执行
// ---------------------------------------------------------------------------

// ControlActor 控制命令发起者。
type ControlActor struct {
	UserID    string
	TenantID  string
	Authority string
	// Claims 真实请求者凭证。
	// 必须有：命令下发通道（CommandData.CommandPutMessageWithTracking）在**不传**
	// claims 时会直接跳过设备写权限校验（见 command_data.go ensureCommandWriteAccess）。
	// 只带 UserID 往下转发，等于把"这台设备归不归你管"这道闸整个绕过去。
	Claims *utils.UserClaims
}

// ControlExecution 交给执行器的执行上下文。
// DeviceID 为命令的目标设备：SCADA 画布上点击的控件最终总是落到某台设备，
// 移动端下发同样必须指定设备。缺少目标设备的命令不可能被执行，
// 因此它是必填字段而非可选。
type ControlExecution struct {
	TenantID    string
	DeviceID    string
	DocumentID  string
	WidgetID    string
	WidgetType  string
	Version     string
	Command     string
	Params      string
	ActorUserID string
	// ActorClaims 真实请求者凭证。执行器必须原样带到下发通道，
	// 缺失时执行器必须拒绝执行（fail closed）——见 ControlActor.Claims 的说明。
	ActorClaims *utils.UserClaims
}

// ControlCommandExecutor 真正的下发执行器。未接线即不得报成功。
type ControlCommandExecutor interface {
	Execute(ctx context.Context, exec ControlExecution) error
}

// ControlRequest 控制命令请求。
type ControlRequest struct {
	TenantID          string
	// DeviceID 命令目标设备（必填：没有目标的命令无从执行）。
	DeviceID          string
	DocumentID        string
	WidgetID          string
	WidgetType        string
	Version           string
	Command           string
	Params            interface{}
	Actor             ControlActor
	ConfirmationToken string
}

// ScadaControlService 控制命令服务。
type ScadaControlService struct {
	registry           *WidgetRegistry
	issuer             *ConfirmationIssuer
	executor           ControlCommandExecutor
	allowedAuthorities map[string]bool
}

// NewScadaControlService 创建控制服务。
// executor 为 nil 时所有命令都以 failed 结束（不假成功）。
func NewScadaControlService(registry *WidgetRegistry, issuer *ConfirmationIssuer, executor ControlCommandExecutor) *ScadaControlService {
	return &ScadaControlService{
		registry: registry,
		issuer:   issuer,
		executor: executor,
		allowedAuthorities: map[string]bool{
			constant.SYS_ADMIN:    true,
			constant.TENANT_ADMIN: true,
		},
	}
}

// SetAllowedAuthorities 覆盖允许下发控制的角色集合（默认系统管理员与租户管理员）。
func (s *ScadaControlService) SetAllowedAuthorities(authorities ...string) {
	if s == nil {
		return
	}
	next := make(map[string]bool, len(authorities))
	for _, a := range authorities {
		next[strings.TrimSpace(a)] = true
	}
	s.allowedAuthorities = next
}

// IssueConfirmation 签发二次确认令牌。
// 签发器未配置时直接报错：没有签发器就没有真正的二次确认，
// 此时返回空令牌等于把危险操作变成一键触发。
func (s *ScadaControlService) IssueConfirmation(ctx context.Context, tenantID, documentID, widgetID, command, actorUserID string) (string, error) {
	if s == nil || s.issuer == nil {
		return "", ErrConfirmationNoIssuer
	}
	return s.issuer.Issue(tenantID, documentID, widgetID, command, actorUserID, time.Now())
}

// ExecuteControl 执行控制命令，返回最终审计结果（success/failed）或错误。
// 被拒绝时返回 denied 且错误非 nil；审计在这两种情况下都已落库。
func (s *ScadaControlService) ExecuteControl(ctx context.Context, req ControlRequest) (string, error) {
	if strings.TrimSpace(req.Actor.UserID) == "" {
		return model.ControlOutcomeDenied, errcode.NewWithMessage(errcode.CodeParamError, ErrControlActorMissing.Error())
	}
	if strings.TrimSpace(req.TenantID) == "" ||
		strings.TrimSpace(req.DeviceID) == "" ||
		strings.TrimSpace(req.DocumentID) == "" ||
		strings.TrimSpace(req.WidgetID) == "" ||
		strings.TrimSpace(req.Command) == "" ||
		strings.TrimSpace(req.WidgetType) == "" {
		return model.ControlOutcomeDenied, errcode.NewWithMessage(errcode.CodeParamError, "control request is incomplete")
	}

	// 闸 1：文档存在且未归档。
	doc, err := dal.GetScadaDocumentInTenant(req.DocumentID, req.TenantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return s.deny(ctx, req, "scada document not found"),
				errcode.NewWithMessage(errcode.CodeNotFound, "scada document not found")
		}
		return model.ControlOutcomeDenied, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	if model.IsScadaTerminalStatus(doc.Status) {
		return s.deny(ctx, req, "scada document is archived"),
			errcode.NewWithMessage(errcode.CodeOpDenied, "archived scada document cannot be controlled")
	}

	// 闸 2：Widget 已注册，且该 Widget 确实声明了这条命令。
	def := s.registry.Get(req.WidgetType, req.Version)
	if def == nil {
		return s.deny(ctx, req, "widget is not registered: "+req.WidgetType+"@"+req.Version),
			errcode.NewWithMessage(errcode.CodeOpDenied, ErrControlWidgetUnknown.Error())
	}
	cmd := def.FindCommand(req.Command)
	if cmd == nil {
		return s.deny(ctx, req, "widget does not declare command: "+req.Command),
			errcode.NewWithMessage(errcode.CodeOpDenied, ErrControlCommandUnknown.Error())
	}

	// 闸 3：权限。跨租户一律拒绝（actor 租户必须与文档租户一致）。
	if !s.hasPermission(req.Actor, doc.TenantID) {
		return s.deny(ctx, req, "actor lacks control permission"),
			errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to send control commands")
	}

	// 闸 4：二次确认。令牌绑定本次动作的全部维度。
	if cmd.RequiresConfirmation {
		if err := s.issuer.Verify(req.ConfirmationToken, req.TenantID, req.DocumentID, req.WidgetID, req.Command, req.Actor.UserID, time.Now()); err != nil {
			return s.deny(ctx, req, "confirmation failed: "+err.Error()),
				errcode.NewWithMessage(errcode.CodeOpDenied, err.Error())
		}
	}

	// 闸 5：审计先于执行落库。写不进去就拒绝执行（见文件头注意事项 2）。
	auditID, err := s.writePending(ctx, req)
	if err != nil || auditID == "" {
		return model.ControlOutcomeFailed, errcode.NewWithMessage(errcode.CodeDBError, ErrControlAuditLost.Error())
	}

	if s.executor == nil {
		s.settle(ctx, req, auditID, model.ControlOutcomeFailed, ErrControlNoExecutor.Error())
		return model.ControlOutcomeFailed, errcode.NewWithMessage(errcode.CodeOpDenied, ErrControlNoExecutor.Error())
	}

	execErr := s.executor.Execute(ctx, ControlExecution{
		TenantID:    req.TenantID,
		DeviceID:    strings.TrimSpace(req.DeviceID),
		DocumentID:  req.DocumentID,
		WidgetID:    req.WidgetID,
		WidgetType:  req.WidgetType,
		Version:     req.Version,
		Command:     req.Command,
		Params:      marshalParams(req.Params),
		ActorUserID: req.Actor.UserID,
		ActorClaims: req.Actor.Claims,
	})
	if execErr != nil {
		s.settle(ctx, req, auditID, model.ControlOutcomeFailed, execErr.Error())
		return model.ControlOutcomeFailed, errcode.WithData(errcode.CodeOpDenied, map[string]interface{}{"error": execErr.Error()})
	}

	s.settle(ctx, req, auditID, model.ControlOutcomeSuccess, "")
	return model.ControlOutcomeSuccess, nil
}

// hasPermission 判断发起者是否有权下发控制。
// 角色白名单 + 租户一致；系统管理员可跨租户，租户管理员仅限本租户。
func (s *ScadaControlService) hasPermission(actor ControlActor, documentTenantID string) bool {
	if !s.allowedAuthorities[strings.TrimSpace(actor.Authority)] {
		return false
	}
	if actor.Authority == constant.SYS_ADMIN {
		return true
	}
	return strings.TrimSpace(actor.TenantID) == strings.TrimSpace(documentTenantID)
}

// deny 落一条拒绝审计并返回 outcome（调用方按其原样返回）。
func (s *ScadaControlService) deny(ctx context.Context, req ControlRequest, detail string) string {
	s.writeAudit(ctx, req, model.ControlOutcomeDenied, detail)
	return model.ControlOutcomeDenied
}

// writePending 落一条 pending 审计，返回记录 ID；失败返回空串。
func (s *ScadaControlService) writePending(ctx context.Context, req ControlRequest) (string, error) {
	record := s.buildAudit(req, model.ControlOutcomePending, "passed all gates; dispatching")
	if err := model.ValidateScadaControlAudit(record); err != nil {
		return "", err
	}
	if err := dal.CreateScadaControlAudit(record); err != nil {
		return "", err
	}
	return record.ID, nil
}

// settle 将 pending 审计推进到终态。
func (s *ScadaControlService) settle(ctx context.Context, req ControlRequest, auditID, outcome, detail string) {
	_, _ = dal.UpdateScadaControlAuditOutcome(auditID, req.TenantID, model.ControlOutcomePending, outcome, detail)
}

// writeAudit 直接落一条审计（用于拒绝路径）。
func (s *ScadaControlService) writeAudit(ctx context.Context, req ControlRequest, outcome, detail string) {
	record := s.buildAudit(req, outcome, detail)
	if err := model.ValidateScadaControlAudit(record); err != nil {
		return
	}
	_ = dal.CreateScadaControlAudit(record)
}

// buildAudit 构造审计记录。确认令牌原样留存：空令牌本身即"未走确认流程"的证据。
func (s *ScadaControlService) buildAudit(req ControlRequest, outcome, detail string) *model.ScadaControlAudit {
	params := marshalParams(req.Params)
	return &model.ScadaControlAudit{
		// ID 必须显式生成：字符串主键留空时，第二条记录就会撞上空主键而写入失败，
		// 结果是"只有第一次拒绝被审计"——恰好把最需要留痕的重复越权尝试丢掉。
		ID:                uuid.New().String(),
		TenantID:          strings.TrimSpace(req.TenantID),
		DocumentID:        strings.TrimSpace(req.DocumentID),
		WidgetID:          strings.TrimSpace(req.WidgetID),
		Command:           strings.TrimSpace(req.Command),
		Params:            &params,
		ActorUserID:       strings.TrimSpace(req.Actor.UserID),
		ConfirmationToken: strings.TrimSpace(req.ConfirmationToken),
		Outcome:           outcome,
		Detail:            &detail,
	}
}
