// 文件用途：SCADA / Widget 基础层的 HTTP 接口（ROADMAP P1.3）。
// 核心逻辑：项目与画布文档的 CRUD、保存、发布、回滚、归档，以及控制命令下发。
// 关键注意事项：
//  1. 租户一律由 claims 推导，不接受请求体自带租户来"指定"操作范围。
//     系统管理员可显式带 tenant_id，租户管理员强制绑定本租户，
//     且传入其他租户一律拒绝（不是静默忽略，否则越权会表现为"查不到"）。
//  2. 控制命令走独立入口，且服务未接线时返回错误而非成功——
//     接口存在不等于能力可用，报成功而不下发是最严重的假成功。
package api

import (
	"strings"

	service "aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type ScadaApi struct{}

// 请求/响应 DTO。
type (
	CreateScadaProjectReq struct {
		TenantID    string  `json:"tenant_id"`
		Name        string  `json:"name" binding:"required"`
		Description *string `json:"description"`
	}
	CreateScadaDocumentReq struct {
		Name     string `json:"name" binding:"required"`
		JSONData string `json:"json_data"`
	}
	SaveScadaDocumentReq struct {
		// ExpectedVersion 乐观并发凭证。缺失即拒绝：
		// 允许"不带版本直接保存"会让并发覆盖无声发生，画好的画布被别人冲掉。
		ExpectedVersion int32  `json:"expected_version" binding:"required"`
		JSONData        string `json:"json_data"`
	}
	RollbackScadaDocumentReq struct {
		Version int32 `json:"version" binding:"required"`
	}
	ControlCommandReq struct {
		TenantID          string                 `json:"tenant_id"`
		DeviceID          string                 `json:"device_id" binding:"required"`
		DocumentID        string                 `json:"document_id" binding:"required"`
		WidgetID          string                 `json:"widget_id" binding:"required"`
		WidgetType        string                 `json:"widget_type" binding:"required"`
		Version           string                 `json:"version"`
		Command           string                 `json:"command" binding:"required"`
		Params            map[string]interface{} `json:"params"`
		ConfirmationToken string                 `json:"confirmation_token"`
	}
	IssueConfirmationReq struct {
		TenantID   string `json:"tenant_id"`
		DocumentID string `json:"document_id" binding:"required"`
		WidgetID   string `json:"widget_id" binding:"required"`
		Command    string `json:"command" binding:"required"`
	}
)

// resolveScadaTenant 由 claims 推导操作租户。
func resolveScadaTenant(c *gin.Context, requested string) (string, error) {
	raw := c.MustGet("claims")
	claims, ok := raw.(*utils.UserClaims)
	if !ok || claims == nil {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, "missing user claims")
	}
	requested = strings.TrimSpace(requested)

	if claims.Authority == constant.SYS_ADMIN {
		if requested != "" {
			return requested, nil
		}
		if strings.TrimSpace(claims.TenantID) == "" {
			return "", errcode.NewWithMessage(errcode.CodeParamError, "tenant id is required")
		}
		return claims.TenantID, nil
	}

	if strings.TrimSpace(claims.TenantID) == "" {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, "no tenant context")
	}
	// 非系统管理员指定其他租户：明确拒绝，而不是静默改成自己的租户。
	if requested != "" && requested != claims.TenantID {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, "cross-tenant operation is not allowed")
	}
	return claims.TenantID, nil
}

func scadaActor(c *gin.Context) (service.ControlActor, error) {
	claims, ok := c.MustGet("claims").(*utils.UserClaims)
	if !ok || claims == nil {
		return service.ControlActor{}, errcode.NewWithMessage(errcode.CodeNoPermission, "missing user claims")
	}
	return service.ControlActor{
		UserID:    claims.ID,
		TenantID:  claims.TenantID,
		Authority: claims.Authority,
		// 带上原始凭证：下发通道靠它做"这台设备归不归你管"的校验，
		// 只传 UserID 会让这道闸静默失效。
		Claims: claims,
	}, nil
}

func stringPtrOrNil(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

// ---------------------------------------------------------------------------
// 项目
// ---------------------------------------------------------------------------

func (*ScadaApi) CreateScadaProject(c *gin.Context) {
	var req CreateScadaProjectReq
	if !BindAndValidate(c, &req) {
		return
	}
	tenantID, err := resolveScadaTenant(c, req.TenantID)
	if err != nil {
		c.Error(err)
		return
	}
	actor, _ := scadaActor(c)
	data, err := service.GroupApp.ScadaDocument.CreateProject(c, service.ScadaProjectCreate{
		TenantID:    tenantID,
		Name:        req.Name,
		Description: req.Description,
		CreatedBy:   stringPtrOrNil(actor.UserID),
	})
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

func (*ScadaApi) ListScadaProjects(c *gin.Context) {
	tenantID, err := resolveScadaTenant(c, c.Query("tenant_id"))
	if err != nil {
		c.Error(err)
		return
	}
	data, err := service.GroupApp.ScadaDocument.ListProjects(c, tenantID, 0)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

func (*ScadaApi) GetScadaProject(c *gin.Context) {
	tenantID, err := resolveScadaTenant(c, c.Query("tenant_id"))
	if err != nil {
		c.Error(err)
		return
	}
	data, err := service.GroupApp.ScadaDocument.GetProject(c, c.Param("id"), tenantID)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

func (*ScadaApi) DeleteScadaProject(c *gin.Context) {
	tenantID, err := resolveScadaTenant(c, c.Query("tenant_id"))
	if err != nil {
		c.Error(err)
		return
	}
	if err := service.GroupApp.ScadaDocument.DeleteProject(c, c.Param("id"), tenantID); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// ---------------------------------------------------------------------------
// 文档
// ---------------------------------------------------------------------------

func (*ScadaApi) CreateScadaDocument(c *gin.Context) {
	var req CreateScadaDocumentReq
	if !BindAndValidate(c, &req) {
		return
	}
	tenantID, err := resolveScadaTenant(c, c.Query("tenant_id"))
	if err != nil {
		c.Error(err)
		return
	}
	actor, _ := scadaActor(c)
	data, err := service.GroupApp.ScadaDocument.CreateDocument(c, service.ScadaDocumentCreate{
		TenantID:  tenantID,
		ProjectID: c.Param("id"),
		Name:      req.Name,
		Canvas:    req.JSONData,
		CreatedBy: stringPtrOrNil(actor.UserID),
	})
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

func (*ScadaApi) ListScadaDocuments(c *gin.Context) {
	tenantID, err := resolveScadaTenant(c, c.Query("tenant_id"))
	if err != nil {
		c.Error(err)
		return
	}
	docs, err := service.GroupApp.ScadaDocument.ListDocuments(c, c.Param("id"), tenantID, 0)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", docs)
}

func (*ScadaApi) GetScadaDocument(c *gin.Context) {
	tenantID, err := resolveScadaTenant(c, c.Query("tenant_id"))
	if err != nil {
		c.Error(err)
		return
	}
	data, err := service.GroupApp.ScadaDocument.LoadDocument(c, c.Param("id"), tenantID)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

func (*ScadaApi) SaveScadaDocument(c *gin.Context) {
	var req SaveScadaDocumentReq
	if !BindAndValidate(c, &req) {
		return
	}
	tenantID, err := resolveScadaTenant(c, c.Query("tenant_id"))
	if err != nil {
		c.Error(err)
		return
	}
	actor, _ := scadaActor(c)
	data, err := service.GroupApp.ScadaDocument.SaveDocument(
		c, c.Param("id"), tenantID, req.ExpectedVersion, req.JSONData, stringPtrOrNil(actor.UserID))
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

func (*ScadaApi) PublishScadaDocument(c *gin.Context) {
	tenantID, err := resolveScadaTenant(c, c.Query("tenant_id"))
	if err != nil {
		c.Error(err)
		return
	}
	actor, _ := scadaActor(c)
	data, err := service.GroupApp.ScadaDocument.PublishDocument(c, c.Param("id"), tenantID, stringPtrOrNil(actor.UserID))
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

func (*ScadaApi) RollbackScadaDocument(c *gin.Context) {
	var req RollbackScadaDocumentReq
	if !BindAndValidate(c, &req) {
		return
	}
	tenantID, err := resolveScadaTenant(c, c.Query("tenant_id"))
	if err != nil {
		c.Error(err)
		return
	}
	actor, _ := scadaActor(c)
	data, err := service.GroupApp.ScadaDocument.RollbackDocument(
		c, c.Param("id"), tenantID, req.Version, stringPtrOrNil(actor.UserID))
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

func (*ScadaApi) ArchiveScadaDocument(c *gin.Context) {
	tenantID, err := resolveScadaTenant(c, c.Query("tenant_id"))
	if err != nil {
		c.Error(err)
		return
	}
	actor, _ := scadaActor(c)
	data, err := service.GroupApp.ScadaDocument.ArchiveDocument(c, c.Param("id"), tenantID, stringPtrOrNil(actor.UserID))
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

func (*ScadaApi) ListScadaDocumentVersions(c *gin.Context) {
	tenantID, err := resolveScadaTenant(c, c.Query("tenant_id"))
	if err != nil {
		c.Error(err)
		return
	}
	data, err := service.GroupApp.ScadaDocument.ListDocumentVersions(c, c.Param("id"), tenantID, 0)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// ---------------------------------------------------------------------------
// 控制
// ---------------------------------------------------------------------------

// scadaControlService 取控制服务；未接线返回 nil，由调用方 fail closed。
func scadaControlService() *service.ScadaControlService {
	return service.GroupApp.ScadaControl
}

// IssueControlConfirmation 签发二次确认令牌。
// 控制服务未接线时直接失败：没有签发器就没有真正的二次确认，
// 返回空令牌等于把危险操作变成一键触发。
func (*ScadaApi) IssueControlConfirmation(c *gin.Context) {
	var req IssueConfirmationReq
	if !BindAndValidate(c, &req) {
		return
	}
	svc := scadaControlService()
	if svc == nil {
		c.Error(errcode.NewWithMessage(errcode.CodeOpDenied, "scada control is not wired"))
		return
	}
	tenantID, err := resolveScadaTenant(c, req.TenantID)
	if err != nil {
		c.Error(err)
		return
	}
	actor, err := scadaActor(c)
	if err != nil {
		c.Error(err)
		return
	}
	token, err := svc.IssueConfirmation(c, tenantID, req.DocumentID, req.WidgetID, req.Command, actor.UserID)
	if err != nil {
		c.Error(errcode.NewWithMessage(errcode.CodeOpDenied, err.Error()))
		return
	}
	c.Set("data", gin.H{"confirmation_token": token})
}

func (*ScadaApi) ExecuteControl(c *gin.Context) {
	var req ControlCommandReq
	if !BindAndValidate(c, &req) {
		return
	}
	svc := scadaControlService()
	if svc == nil {
		c.Error(errcode.NewWithMessage(errcode.CodeOpDenied, "scada control is not wired"))
		return
	}
	tenantID, err := resolveScadaTenant(c, req.TenantID)
	if err != nil {
		c.Error(err)
		return
	}
	actor, err := scadaActor(c)
	if err != nil {
		c.Error(err)
		return
	}
	outcome, execErr := svc.ExecuteControl(c, service.ControlRequest{
		TenantID:          tenantID,
		DeviceID:          req.DeviceID,
		DocumentID:        req.DocumentID,
		WidgetID:          req.WidgetID,
		WidgetType:        req.WidgetType,
		Version:           req.Version,
		Command:           req.Command,
		Params:            req.Params,
		Actor:             actor,
		ConfirmationToken: req.ConfirmationToken,
	})
	if execErr != nil {
		c.Error(execErr)
		return
	}
	c.Set("data", gin.H{"outcome": outcome})
}

// ListScadaControlAudits 列出文档的控制审计（含被拒绝的尝试）。
func (*ScadaApi) ListScadaControlAudits(c *gin.Context) {
	tenantID, err := resolveScadaTenant(c, c.Query("tenant_id"))
	if err != nil {
		c.Error(err)
		return
	}
	rows, err := service.GroupApp.ScadaDocument.ListControlAudits(c, c.Param("id"), tenantID, 0)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", rows)
}
