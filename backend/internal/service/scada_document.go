// 文件用途：SCADA 项目与画布文档服务层（ROADMAP P1.3）。
// 核心逻辑：项目 CRUD、画布保存/加载往返、乐观并发、发布与回滚、归档终态。
//
// 关键注意事项（本轮刻意守住的几条，违反任一条都会造出假成功）：
//  1. 保存必须带 expectedVersion 且走数据库条件更新。RowsAffected=0 时**必须再查一次**
//     来区分"版本冲突"与"文档不存在"——直接统一报冲突，会让"文档早被删了"伪装成
//     "你保存太慢了"，反过来也会让并发冲突伪装成资源消失。
//  2. 跨租户一律表现为 CodeNotFound，不用 403：403 会泄漏"该 ID 存在于别处"。
//  3. 回滚**不改历史**：把目标版本的内容写成新的草稿版本，不由调用方直接把
//     published_version 指回旧值。后者会让"当前草稿"与"已发布内容"脱节，
//     并且丢掉"做过一次回滚"这件事本身的可追溯性（与 fleet 批次回滚保持同一语义）。
//  4. 归档是终态。已归档文档拒绝保存/发布/控制——否则"归档"只是个装饰性标签。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ScadaDocumentService SCADA 项目与画布文档服务。
type ScadaDocumentService struct{}

// 画布空载荷归一化结果：空画布存 {} 而不是 NULL/空串，
// 这样"加载出来没有 widget"与"加载失败"在前端是两种可区分的事实。
const scadaEmptyCanvas = "{}"


// ScadaProjectCreate 创建项目入参。
type ScadaProjectCreate struct {
	TenantID    string
	Name        string
	Description *string
	CreatedBy   *string
}

// ScadaDocumentCreate 创建文档入参。
type ScadaDocumentCreate struct {
	TenantID  string
	ProjectID string
	Name      string
	Canvas    string
	CreatedBy *string
}

// scadaNotFound 统一的不存在错误。
func scadaNotFound(message string) error {
	return errcode.NewWithMessage(errcode.CodeNotFound, message)
}

// scadaDenied 统一的操作被拒绝错误。
func scadaDenied(message string) error {
	return errcode.NewWithMessage(errcode.CodeOpDenied, message)
}

// scadaParam 统一的参数错误。
func scadaParam(message string) error {
	return errcode.NewWithMessage(errcode.CodeParamError, message)
}

// normalizeCanvas 归一化画布载荷：空白一律折叠为 {}。
func normalizeCanvas(canvas string) string {
	if strings.TrimSpace(canvas) == "" {
		return scadaEmptyCanvas
	}
	return canvas
}

// scadaRequireEditable 校验文档可写：归档为终态，拒绝一切写入。
func scadaRequireEditable(doc *model.ScadaDocument) error {
	if model.IsScadaTerminalStatus(doc.Status) {
		return scadaDenied("archived scada document cannot be modified")
	}
	return nil
}

// ---------------------------------------------------------------------------
// 项目 CRUD（门禁：项目 CRUD 不再返回 unsupported）
// ---------------------------------------------------------------------------

// CreateProject 创建项目。同名冲突返回参数错误，不泄漏其他租户信息。
func (s *ScadaDocumentService) CreateProject(ctx context.Context, req ScadaProjectCreate) (*model.ScadaProject, error) {
	// ID 由服务层显式生成：数据库默认值（gen_random_uuid()）只覆盖 PostgreSQL，
	// 一旦换库或走内存库就会拿到空主键，而空主键会让后续所有引用静默指向错误行。
	project := &model.ScadaProject{
		ID:          uuid.New().String(),
		TenantID:    strings.TrimSpace(req.TenantID),
		Name:        strings.TrimSpace(req.Name),
		Description: req.Description,
		CreatedBy:   req.CreatedBy,
	}
	if err := model.ValidateScadaProject(project); err != nil {
		return nil, scadaParam(err.Error())
	}
	if err := dal.CreateScadaProject(project); err != nil {
		return nil, scadaParam("scada project already exists or is invalid")
	}
	return project, nil
}

// GetProject 租户内读取项目；跨租户或不存在均表现为 not found。
func (s *ScadaDocumentService) GetProject(ctx context.Context, id, tenantID string) (*model.ScadaProject, error) {
	project, err := dal.GetScadaProjectInTenant(id, tenantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, scadaNotFound("scada project not found")
		}
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return project, nil
}

// ListProjects 列出租户内项目。
func (s *ScadaDocumentService) ListProjects(ctx context.Context, tenantID string, limit int) ([]model.ScadaProject, error) {
	if strings.TrimSpace(tenantID) == "" {
		return nil, scadaParam("tenant id is required")
	}
	rows, err := dal.ListScadaProjects(tenantID, limit)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return rows, nil
}

// DeleteProject 删除项目。文档由数据库外键级联删除；无匹配时表现为 not found。
func (s *ScadaDocumentService) DeleteProject(ctx context.Context, id, tenantID string) error {
	affected, err := dal.DeleteScadaProjectInTenant(id, tenantID)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	if affected == 0 {
		return scadaNotFound("scada project not found")
	}
	return nil
}

// ---------------------------------------------------------------------------
// 画布文档
// ---------------------------------------------------------------------------

// CreateDocument 在指定项目下创建文档。项目必须属于同一租户。
func (s *ScadaDocumentService) CreateDocument(ctx context.Context, req ScadaDocumentCreate) (*model.ScadaDocument, error) {
	if _, err := s.GetProject(ctx, req.ProjectID, req.TenantID); err != nil {
		return nil, err
	}
	canvas := normalizeCanvas(req.Canvas)
	doc := &model.ScadaDocument{
		ID:             uuid.New().String(),
		TenantID:       strings.TrimSpace(req.TenantID),
		ProjectID:      strings.TrimSpace(req.ProjectID),
		Name:           strings.TrimSpace(req.Name),
		Status:         model.ScadaStatusDraft,
		CurrentVersion: 1,
		JSONData:       &canvas,
		CreatedBy:      req.CreatedBy,
	}
	if err := model.ValidateScadaDocument(doc); err != nil {
		return nil, scadaParam(err.Error())
	}
	if err := dal.CreateScadaDocument(doc); err != nil {
		return nil, scadaParam("scada document already exists or is invalid")
	}
	return doc, nil
}

// ListDocuments 列出项目下的文档。项目必须属于同一租户，
// 否则"列别人的文档"会退化成返回空列表，把越权伪装成"这个项目没有文档"。
func (s *ScadaDocumentService) ListDocuments(ctx context.Context, projectID, tenantID string, limit int) ([]model.ScadaDocument, error) {
	if _, err := s.GetProject(ctx, projectID, tenantID); err != nil {
		return nil, err
	}
	rows, err := dal.ListScadaDocumentsByProject(tenantID, projectID, limit)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return rows, nil
}

// ListControlAudits 列出文档的控制审计（含被拒绝的尝试）。
func (s *ScadaDocumentService) ListControlAudits(ctx context.Context, documentID, tenantID string, limit int) ([]model.ScadaControlAudit, error) {
	if _, err := s.LoadDocument(ctx, documentID, tenantID); err != nil {
		return nil, err
	}
	rows, err := dal.ListScadaControlAudits(tenantID, documentID, limit)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return rows, nil
}

// LoadDocument 加载画布。门禁"画布保存/加载可往返"的一半。
func (s *ScadaDocumentService) LoadDocument(ctx context.Context, id, tenantID string) (*model.ScadaDocument, error) {
	doc, err := dal.GetScadaDocumentInTenant(id, tenantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, scadaNotFound("scada document not found")
		}
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return doc, nil
}

// SaveDocument 乐观并发保存画布，成功返回更新后的文档。
// expectedVersion 必须等于当前版本，否则返回版本冲突。
func (s *ScadaDocumentService) SaveDocument(ctx context.Context, id, tenantID string, expectedVersion int32, canvas string, actorUserID *string) (*model.ScadaDocument, error) {
	if expectedVersion <= 0 {
		return nil, scadaParam("expected version must be positive")
	}
	payload := normalizeCanvas(canvas)
	doc := &model.ScadaDocument{JSONData: &payload}
	if err := model.ValidateScadaCanvas(doc.JSONData); err != nil {
		return nil, scadaParam(err.Error())
	}

	affected, err := dal.SaveScadaDocumentInTenant(id, tenantID, expectedVersion, payload, actorUserID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	if affected == 0 {
		// 必须区分"版本冲突"与"不存在"：见文件头注意事项 1。
		existing, getErr := dal.GetScadaDocumentInTenant(id, tenantID)
		if getErr != nil {
			if errors.Is(getErr, gorm.ErrRecordNotFound) {
				return nil, scadaNotFound("scada document not found")
			}
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": getErr.Error()})
		}
		if model.IsScadaTerminalStatus(existing.Status) {
			return nil, scadaDenied("archived scada document cannot be modified")
		}
		return nil, scadaDenied("scada document version conflict; reload before saving")
	}
	return s.LoadDocument(ctx, id, tenantID)
}

// PublishDocument 发布当前草稿版本：写入不可变快照并把文档标记为已发布。
// 同一版本重复发布被拒绝——重复发布同一内容只会让版本历史出现噪声，
// 让人误以为发生过两次变更。
func (s *ScadaDocumentService) PublishDocument(ctx context.Context, id, tenantID string, actorUserID *string) (*model.ScadaDocument, error) {
	doc, err := s.LoadDocument(ctx, id, tenantID)
	if err != nil {
		return nil, err
	}
	if err := scadaRequireEditable(doc); err != nil {
		return nil, err
	}
	if doc.PublishedVersion != nil && *doc.PublishedVersion == doc.CurrentVersion {
		return nil, scadaDenied("current draft version is already published; save a change before publishing again")
	}

	snapshot := &model.ScadaDocumentVersion{
		ID:          uuid.New().String(),
		TenantID:    doc.TenantID,
		DocumentID:  doc.ID,
		Version:     doc.CurrentVersion,
		JSONData:    doc.JSONData,
		PublishedBy: actorUserID,
	}
	if err := model.ValidateScadaVersionSnapshot(snapshot); err != nil {
		return nil, scadaParam(err.Error())
	}
	if err := dal.InsertScadaDocumentVersion(snapshot); err != nil {
		return nil, scadaDenied("this version was already published")
	}

	affected, err := dal.MarkScadaDocumentPublished(doc.ID, doc.TenantID, doc.CurrentVersion, doc.CurrentVersion, actorUserID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	if affected == 0 {
		// 快照已写入但文档状态没跟上：并发保存把版本推走了。
		// 此时必须明确失败——留着一条没人引用的快照，回滚列表里会出现幽灵版本。
		return nil, scadaDenied("scada document changed while publishing; retry")
	}
	return s.LoadDocument(ctx, id, tenantID)
}

// RollbackDocument 回滚到某个已发布版本。
// 语义（见文件头注意事项 3）：不改历史，把目标版本内容写成新的草稿版本，
// 且不自动发布——回滚同样需要人确认后再发布，否则一次误操作会直接上线。
func (s *ScadaDocumentService) RollbackDocument(ctx context.Context, id, tenantID string, targetVersion int32, actorUserID *string) (*model.ScadaDocument, error) {
	if targetVersion <= 0 {
		return nil, scadaParam("target version must be positive")
	}
	doc, err := s.LoadDocument(ctx, id, tenantID)
	if err != nil {
		return nil, err
	}
	if err := scadaRequireEditable(doc); err != nil {
		return nil, err
	}
	snapshot, err := dal.GetScadaDocumentVersion(doc.TenantID, doc.ID, targetVersion)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, scadaDenied("target version was never published")
		}
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}

	canvas := normalizeCanvas(stringPtrValue(snapshot.JSONData))
	affected, err := dal.SaveScadaDocumentInTenant(doc.ID, doc.TenantID, doc.CurrentVersion, canvas, actorUserID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	if affected == 0 {
		return nil, scadaDenied("scada document changed during rollback; retry")
	}
	return s.LoadDocument(ctx, id, tenantID)
}

// ArchiveDocument 归档文档（终态）。
func (s *ScadaDocumentService) ArchiveDocument(ctx context.Context, id, tenantID string, actorUserID *string) (*model.ScadaDocument, error) {
	doc, err := s.LoadDocument(ctx, id, tenantID)
	if err != nil {
		return nil, err
	}
	affected, err := dal.MarkScadaDocumentArchived(doc.ID, doc.TenantID, actorUserID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	if affected == 0 {
		return nil, scadaNotFound("scada document not found")
	}
	return s.LoadDocument(ctx, id, tenantID)
}

// ListDocumentVersions 列出已发布版本，供回滚选择。
func (s *ScadaDocumentService) ListDocumentVersions(ctx context.Context, id, tenantID string, limit int) ([]model.ScadaDocumentVersion, error) {
	if _, err := s.LoadDocument(ctx, id, tenantID); err != nil {
		return nil, err
	}
	rows, err := dal.ListScadaDocumentVersions(tenantID, id, limit)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return rows, nil
}

// stringPtrValue 取指针字符串值，nil 时返回空串。
func stringPtrValue(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// marshalParams 将任意结构序列化为 JSON 字符串，失败返回 "{}"。
// 控制审计的参数序列化失败不应阻断审计写入本身，但也不能假装成功，
// 故退化为空对象并在调用方留下可识别的痕迹。
func marshalParams(v interface{}) string {
	if v == nil {
		return "{}"
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(raw)
}
