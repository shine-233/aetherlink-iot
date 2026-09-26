// 文件用途：移动应用中心服务层单测（ROADMAP TB-23）——状态机流转、重复版本拒绝与租户隔离。
// 核心逻辑：sqlite 内存库 + 真实 dal 栈 + 临时工作目录（./files 落盘），覆盖：
//
//	状态机合法矩阵（publish/archive/delete/update 的资格判定）、发布/归档全流程
//	（published_at 落值与保留）、同租户重复版本拒绝（预检 + 唯一约束兜底 + 文件回滚）、
//	跨租户不可见与"版本唯一性是租户内概念"、上传校验矩阵（平台/版本/扩展名/ZIP 签名）、
//	下载路径白名单（files/apps/ 前缀 + 防目录逃逸）。
//
// 关键注意事项：测试用 os.Chdir 切到临时目录使 common.BaseUploadDir（./files/）落临时区，
//
//	用例不得 t.Parallel()；multipart 文件头经真实 multipart.ReadForm 构造，
//	保证 file.Open() 链路真实可走（不伪造 FileHeader 内部字段）。
//
// 重构建议：uniapp 对接阶段加"published 拉取面"时，在本文件补对应租户与状态过滤用例。
package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	global "aetherlink-iot/backend/pkg/global"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupAppBundleServiceTestDB 建立仅含 mobile_app_bundles 的 sqlite 内存库并切换临时工作目录。
func setupAppBundleServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := global.DB
	dbName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open app bundle service sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.MobileAppBundle{}); err != nil {
		t.Fatalf("migrate mobile_app_bundles: %v", err)
	}
	global.DB = db
	t.Cleanup(func() { global.DB = oldDB })

	// ./files 是相对工作目录的路径，切到临时目录让落盘/删除发生在测试沙箱内。
	// 因此本文件内的用例不可 t.Parallel()。
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("change to temp working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
	return db
}

// makeBundleFileHeader 用真实 multipart 编码构造可 Open 的 FileHeader。
func makeBundleFileHeader(t *testing.T, fileName string, content []byte) *multipart.FileHeader {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	form, err := multipart.NewReader(body, writer.Boundary()).ReadForm(int64(body.Len() + 1024))
	if err != nil {
		t.Fatalf("parse multipart form: %v", err)
	}
	headers := form.File["file"]
	if len(headers) == 0 {
		t.Fatal("multipart form has no file field")
	}
	return headers[0]
}

// zipContent 构造带 PK 魔数的最小 ZIP 头内容（apk/ipa/zip 共用的容器签名）。
func zipContent(marker string) []byte {
	return append([]byte("PK\x03\x04"), []byte(marker)...)
}

// createAppBundleForTest 走真实 CreateAppBundle 落一条 draft 登记。
func createAppBundleForTest(t *testing.T, tenantID, platform, version string) *model.MobileAppBundle {
	t.Helper()
	record, err := (&MobileAppBundleService{}).CreateAppBundle(context.Background(),
		&model.AppBundleCreateInput{
			TenantID:     tenantID,
			Platform:     platform,
			Version:      version,
			ReleaseNotes: "notes " + version,
			FileName:     "bundle" + map[string]string{"android": ".apk", "ios": ".ipa", "h5": ".zip"}[platform],
		},
		makeBundleFileHeader(t, "bundle"+map[string]string{"android": ".apk", "ios": ".ipa", "h5": ".zip"}[platform], zipContent(version)),
	)
	if err != nil {
		t.Fatalf("create app bundle %s/%s for %s: %v", platform, version, tenantID, err)
	}
	return record
}

// bizCode 提取业务错误码（非 errcode.Error 直接失败，避免把 -1 误当业务码比对）。
// 注意：不要复用 entity_version_test.go 的 errCodeOf——那里非业务错误返回 -1，
// 这里宁可 fail-fast 也不引入"错误没被包装却断言通过"的假绿。
func bizCode(t *testing.T, err error) int {
	t.Helper()
	var bizErr *errcode.Error
	if !errors.As(err, &bizErr) {
		t.Fatalf("expected errcode.Error, got %T: %v", err, err)
	}
	return bizErr.Code
}

// TestCanTransitionAppBundleMatrix 状态机唯一判定点的合法矩阵。
func TestCanTransitionAppBundleMatrix(t *testing.T) {
	cases := []struct {
		name    string
		current string
		action  string
		want    string
		ok      bool
	}{
		{"draft 可发布", model.AppBundleStatusDraft, AppBundleActionPublish, model.AppBundleStatusPublished, true},
		{"published 可归档", model.AppBundleStatusPublished, AppBundleActionArchive, model.AppBundleStatusArchived, true},
		{"draft 不可直接归档", model.AppBundleStatusDraft, AppBundleActionArchive, "", false},
		{"published 不可重复发布", model.AppBundleStatusPublished, AppBundleActionPublish, "", false},
		{"archived 不可复活", model.AppBundleStatusArchived, AppBundleActionPublish, "", false},
		{"archived 不可重复归档", model.AppBundleStatusArchived, AppBundleActionArchive, "", false},
		{"未知动作拒绝", model.AppBundleStatusDraft, "rollback", "", false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, ok := CanTransitionAppBundle(testCase.current, testCase.action)
			if got != testCase.want || ok != testCase.ok {
				t.Fatalf("CanTransition(%s, %s) = (%q, %v), want (%q, %v)",
					testCase.current, testCase.action, got, ok, testCase.want, testCase.ok)
			}
		})
	}
}

// TestCreateAppBundleSavesFileAndChecksum 上传落盘 + SHA-256 + 默认 draft。
func TestCreateAppBundleSavesFileAndChecksum(t *testing.T) {
	if setupAppBundleServiceTestDB(t) == nil {
		t.Fatal("setup failed")
	}
	content := zipContent("test-apk-payload")
	record, err := (&MobileAppBundleService{}).CreateAppBundle(context.Background(),
		&model.AppBundleCreateInput{
			TenantID:     "tenant-a",
			Platform:     model.AppBundlePlatformAndroid,
			Version:      " 1.0.0 ", // 首尾空白应被归一
			ReleaseNotes: "first release",
			FileName:     "  app-release.apk  ",
		},
		makeBundleFileHeader(t, "app-release.apk", content),
	)
	if err != nil {
		t.Fatalf("create app bundle: %v", err)
	}
	if record.Status != model.AppBundleStatusDraft {
		t.Fatalf("new bundle status = %q, want draft", record.Status)
	}
	if record.Version != "1.0.0" {
		t.Fatalf("version not normalized: %q", record.Version)
	}
	wantChecksum := hex.EncodeToString(func() []byte {
		sum := sha256.Sum256(content)
		return sum[:]
	}())
	if record.Checksum != wantChecksum {
		t.Fatalf("checksum = %q, want sha256 %q", record.Checksum, wantChecksum)
	}
	if record.FileSize != int64(len(content)) {
		t.Fatalf("file size = %d, want %d", record.FileSize, len(content))
	}
	// 文件必须真实落盘在 files/apps/<platform>/ 下。
	diskRel := strings.TrimPrefix(strings.TrimPrefix(record.FilePath, "./"), "files/")
	if _, err := os.Stat(filepath.Join("files", filepath.FromSlash(diskRel))); err != nil {
		t.Fatalf("uploaded file not found on disk: %v (path %s)", err, record.FilePath)
	}
	if !strings.Contains(filepath.ToSlash(record.FilePath), "apps/android/") {
		t.Fatalf("file path should live under apps/android/: %s", record.FilePath)
	}
	if record.PublishedAt != nil {
		t.Fatalf("draft bundle should have nil published_at, got %v", record.PublishedAt)
	}
}

// TestCreateAppBundleValidationMatrix 上传校验矩阵（fail-closed）。
func TestCreateAppBundleValidationMatrix(t *testing.T) {
	setupAppBundleServiceTestDB(t)
	svc := &MobileAppBundleService{}
	ctx := context.Background()

	cases := []struct {
		name    string
		input   model.AppBundleCreateInput
		file    string
		content []byte
		want    int
	}{
		{"非法平台", model.AppBundleCreateInput{TenantID: "t", Platform: "windows", Version: "1.0.0", FileName: "a.zip"}, "a.zip", zipContent("x"), errcode.CodeParamError},
		{"空版本", model.AppBundleCreateInput{TenantID: "t", Platform: "android", Version: "  ", FileName: "a.apk"}, "a.apk", zipContent("x"), errcode.CodeParamError},
		{"版本含路径符号", model.AppBundleCreateInput{TenantID: "t", Platform: "android", Version: "../../x", FileName: "a.apk"}, "a.apk", zipContent("x"), errcode.CodeParamError},
		{"版本含空白", model.AppBundleCreateInput{TenantID: "t", Platform: "android", Version: "1 0", FileName: "a.apk"}, "a.apk", zipContent("x"), errcode.CodeParamError},
		{"版本点开头", model.AppBundleCreateInput{TenantID: "t", Platform: "android", Version: ".1.0", FileName: "a.apk"}, "a.apk", zipContent("x"), errcode.CodeParamError},
		{"平台与扩展名不匹配", model.AppBundleCreateInput{TenantID: "t", Platform: "android", Version: "1.0.0", FileName: "a.zip"}, "a.zip", zipContent("x"), errcode.CodeFileTypeMismatch},
		{"ios 不收 apk", model.AppBundleCreateInput{TenantID: "t", Platform: "ios", Version: "1.0.0", FileName: "a.apk"}, "a.apk", zipContent("x"), errcode.CodeFileTypeMismatch},
		{"改后缀伪装文件", model.AppBundleCreateInput{TenantID: "t", Platform: "android", Version: "1.0.0", FileName: "a.apk"}, "a.apk", []byte("MZ fake executable"), errcode.CodeFileTypeMismatch},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			input := testCase.input
			_, err := svc.CreateAppBundle(ctx, &input, makeBundleFileHeader(t, testCase.file, testCase.content))
			if got := bizCode(t, err); got != testCase.want {
				t.Fatalf("error code = %d, want %d (err=%v)", got, testCase.want, err)
			}
		})
	}

	// 空文件单独覆盖（nil 与 0 字节两个入口）。
	if _, err := svc.CreateAppBundle(ctx, &model.AppBundleCreateInput{TenantID: "t", Platform: "android", Version: "1.0.0", FileName: "a.apk"}, nil); bizCode(t, err) != errcode.CodeFileEmpty {
		t.Fatalf("nil file should be rejected with CodeFileEmpty, got %v", err)
	}
	if _, err := svc.CreateAppBundle(ctx, &model.AppBundleCreateInput{TenantID: "t", Platform: "android", Version: "1.0.0", FileName: "a.apk"}, makeBundleFileHeader(t, "a.apk", nil)); bizCode(t, err) != errcode.CodeFileEmpty {
		t.Fatalf("empty file should be rejected with CodeFileEmpty, got %v", err)
	}
	// 无租户上下文 fail-closed。
	if _, err := svc.CreateAppBundle(ctx, &model.AppBundleCreateInput{Platform: "android", Version: "1.0.0", FileName: "a.apk"}, makeBundleFileHeader(t, "a.apk", zipContent("x"))); bizCode(t, err) != errcode.CodeNoPermission {
		t.Fatalf("missing tenant should be rejected with CodeNoPermission, got %v", err)
	}
}

// TestCreateAppBundleRejectsDuplicateVersion 同租户同平台同版本拒绝（含并发唯一约束兜底），文件回滚。
func TestCreateAppBundleRejectsDuplicateVersion(t *testing.T) {
	setupAppBundleServiceTestDB(t)
	svc := &MobileAppBundleService{}
	ctx := context.Background()

	createAppBundleForTest(t, "tenant-a", model.AppBundlePlatformAndroid, "1.0.0")
	appsDir := filepath.Join("files", "apps", "android", time.Now().Format("2006-01-02"))
	entriesBefore, err := os.ReadDir(appsDir)
	if err != nil {
		t.Fatalf("list apps dir: %v", err)
	}

	// 预检命中：重复版本直接拒绝。
	_, err = svc.CreateAppBundle(ctx, &model.AppBundleCreateInput{
		TenantID: "tenant-a", Platform: model.AppBundlePlatformAndroid, Version: "1.0.0", FileName: "another.apk",
	}, makeBundleFileHeader(t, "another.apk", zipContent("dup")))
	if bizCode(t, err) != errcode.CodeAppBundleVersionExists {
		t.Fatalf("duplicate version should be rejected with %d, got %v", errcode.CodeAppBundleVersionExists, err)
	}

	// 绕过预检直插，验证唯一约束兜底错误映射（sqlite 文案）。
	now := time.Now().UTC()
	direct := &model.MobileAppBundle{
		ID: "dup-row", TenantID: "tenant-a", Platform: model.AppBundlePlatformAndroid, Version: "1.0.0",
		FileName: "d.apk", FilePath: "./files/apps/android/d.apk", Checksum: "x", Status: model.AppBundleStatusDraft,
		CreatedAt: &now, UpdatedAt: &now,
	}
	if insertErr := dal.CreateAppBundle(direct); insertErr == nil {
		t.Fatal("inserting duplicate (tenant, platform, version) should violate unique constraint")
	} else if !dal.IsDuplicateAppBundleError(insertErr) {
		t.Fatalf("error should be detected as duplicate-version conflict: %v", insertErr)
	}

	// 同版本不同平台放行（唯一键含 platform）。
	createAppBundleForTest(t, "tenant-a", model.AppBundlePlatformH5, "1.0.0")

	// 拒绝路径不留孤儿文件：apps/android 下文件数不变。
	entriesAfter, err := os.ReadDir(appsDir)
	if err != nil {
		t.Fatalf("re-list apps dir: %v", err)
	}
	if len(entriesAfter) != len(entriesBefore) {
		t.Fatalf("rejected upload must roll back its file: before=%d after=%d", len(entriesBefore), len(entriesAfter))
	}
}

// TestPublishArchiveRejectsIllegalTransitions 全流程：发布/归档合法流转 + 非法流转拒绝。
func TestPublishArchiveRejectsIllegalTransitions(t *testing.T) {
	setupAppBundleServiceTestDB(t)
	svc := &MobileAppBundleService{}
	ctx := context.Background()

	record := createAppBundleForTest(t, "tenant-a", model.AppBundlePlatformAndroid, "2.0.0")

	// draft 直接归档拒绝。
	if _, err := svc.ArchiveAppBundle(ctx, record.ID, "tenant-a"); bizCode(t, err) != errcode.CodeAppBundleInvalidTransition {
		t.Fatalf("archive on draft should be rejected, got %v", err)
	}

	// draft → published：published_at 落值。
	published, err := svc.PublishAppBundle(ctx, record.ID, "tenant-a")
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if published.Status != model.AppBundleStatusPublished || published.PublishedAt == nil {
		t.Fatalf("after publish: status=%s published_at=%v", published.Status, published.PublishedAt)
	}
	publishedAt := *published.PublishedAt

	// published 重复发布拒绝（202005）。
	if _, err := svc.PublishAppBundle(ctx, record.ID, "tenant-a"); bizCode(t, err) != errcode.CodeAppBundleInvalidTransition {
		t.Fatalf("re-publish should be rejected, got %v", err)
	}

	// published → archived：published_at 保留作发布履历。
	archived, err := svc.ArchiveAppBundle(ctx, record.ID, "tenant-a")
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if archived.Status != model.AppBundleStatusArchived {
		t.Fatalf("after archive status = %s", archived.Status)
	}
	if archived.PublishedAt == nil || !archived.PublishedAt.Equal(publishedAt) {
		t.Fatalf("archived bundle should keep published_at %v, got %v", publishedAt, archived.PublishedAt)
	}

	// archived 重复归档/复活发布均拒绝（202005）。
	if _, err := svc.ArchiveAppBundle(ctx, record.ID, "tenant-a"); bizCode(t, err) != errcode.CodeAppBundleInvalidTransition {
		t.Fatalf("re-archive should be rejected, got %v", err)
	}
	if _, err := svc.PublishAppBundle(ctx, record.ID, "tenant-a"); bizCode(t, err) != errcode.CodeAppBundleInvalidTransition {
		t.Fatalf("publish from archived should be rejected, got %v", err)
	}

	// 不存在的记录按 404 语义（CodeNotFound）。
	if _, err := svc.PublishAppBundle(ctx, "missing-id", "tenant-a"); bizCode(t, err) != errcode.CodeNotFound {
		t.Fatalf("publish missing bundle should be CodeNotFound, got %v", err)
	}
}

// TestAppBundleUpdateDraftOnly 更新资格：仅 draft 可改发布说明。
func TestAppBundleUpdateDraftOnly(t *testing.T) {
	setupAppBundleServiceTestDB(t)
	svc := &MobileAppBundleService{}
	ctx := context.Background()

	record := createAppBundleForTest(t, "tenant-a", model.AppBundlePlatformH5, "1.1.0")
	notes := "updated notes"
	updated, err := svc.UpdateAppBundle(ctx, record.ID, &model.UpdateAppBundleReq{ReleaseNotes: &notes}, "tenant-a")
	if err != nil {
		t.Fatalf("update draft: %v", err)
	}
	if updated.ReleaseNotes != notes {
		t.Fatalf("release notes = %q, want %q", updated.ReleaseNotes, notes)
	}

	if _, err := svc.PublishAppBundle(ctx, record.ID, "tenant-a"); err != nil {
		t.Fatalf("publish: %v", err)
	}
	longNotes := strings.Repeat("x", 2001)
	if _, err := svc.UpdateAppBundle(ctx, record.ID, &model.UpdateAppBundleReq{ReleaseNotes: &longNotes}, "tenant-a"); bizCode(t, err) != errcode.CodeAppBundleInvalidTransition {
		t.Fatalf("update on published should be rejected as invalid transition, got %v", err)
	}
	if _, err := svc.UpdateAppBundle(ctx, record.ID, &model.UpdateAppBundleReq{ReleaseNotes: &notes}, "tenant-a"); bizCode(t, err) != errcode.CodeAppBundleInvalidTransition {
		t.Fatalf("update on published should be rejected, got %v", err)
	}
}

// TestDeleteAppBundleLifecycle 删除资格：published 拒绝；draft/archived 连文件一起删。
func TestDeleteAppBundleLifecycle(t *testing.T) {
	setupAppBundleServiceTestDB(t)
	svc := &MobileAppBundleService{}
	ctx := context.Background()

	// draft：删行 + 删文件。
	draft := createAppBundleForTest(t, "tenant-a", model.AppBundlePlatformAndroid, "0.9.0")
	draftRel := strings.TrimPrefix(strings.TrimPrefix(draft.FilePath, "./"), "files/")
	if err := svc.DeleteAppBundle(ctx, draft.ID, "tenant-a"); err != nil {
		t.Fatalf("delete draft: %v", err)
	}
	if _, err := os.Stat(filepath.Join("files", filepath.FromSlash(draftRel))); !os.IsNotExist(err) {
		t.Fatalf("draft file should be removed from disk, stat err=%v", err)
	}
	if _, err := svc.GetAppBundle(ctx, draft.ID, "tenant-a"); bizCode(t, err) != errcode.CodeNotFound {
		t.Fatalf("deleted draft should be gone, got %v", err)
	}

	// published：拒绝删除（202005，operation=delete）。
	published := createAppBundleForTest(t, "tenant-a", model.AppBundlePlatformIOS, "1.2.0")
	if _, err := svc.PublishAppBundle(ctx, published.ID, "tenant-a"); err != nil {
		t.Fatalf("publish: %v", err)
	}
	err := svc.DeleteAppBundle(ctx, published.ID, "tenant-a")
	if bizCode(t, err) != errcode.CodeAppBundleInvalidTransition {
		t.Fatalf("delete published should be rejected, got %v", err)
	}
	// 拒绝后文件本体必须原样保留（不能顺带删盘）。
	pubRel := strings.TrimPrefix(strings.TrimPrefix(published.FilePath, "./"), "files/")
	if _, statErr := os.Stat(filepath.Join("files", filepath.FromSlash(pubRel))); statErr != nil {
		t.Fatalf("published bundle file must remain after rejected delete: %v", statErr)
	}

	// archived：允许删除。
	if _, err := svc.ArchiveAppBundle(ctx, published.ID, "tenant-a"); err != nil {
		t.Fatalf("archive: %v", err)
	}
	if err := svc.DeleteAppBundle(ctx, published.ID, "tenant-a"); err != nil {
		t.Fatalf("delete archived: %v", err)
	}
}

// TestAppBundleTenantIsolation 租户隔离 fail-closed + 版本唯一性是租户内概念。
func TestAppBundleTenantIsolation(t *testing.T) {
	setupAppBundleServiceTestDB(t)
	svc := &MobileAppBundleService{}
	ctx := context.Background()

	record := createAppBundleForTest(t, "tenant-a", model.AppBundlePlatformAndroid, "3.0.0")

	// tenant-b 看不到 tenant-a 的包。
	if _, err := svc.GetAppBundle(ctx, record.ID, "tenant-b"); bizCode(t, err) != errcode.CodeNotFound {
		t.Fatalf("cross-tenant get should be CodeNotFound, got %v", err)
	}
	if _, err := svc.PublishAppBundle(ctx, record.ID, "tenant-b"); bizCode(t, err) != errcode.CodeNotFound {
		t.Fatalf("cross-tenant publish should be CodeNotFound, got %v", err)
	}
	if err := svc.DeleteAppBundle(ctx, record.ID, "tenant-b"); bizCode(t, err) != errcode.CodeNotFound {
		t.Fatalf("cross-tenant delete should be CodeNotFound, got %v", err)
	}
	if _, err := svc.UpdateAppBundle(ctx, record.ID, &model.UpdateAppBundleReq{}, "tenant-b"); bizCode(t, err) != errcode.CodeNotFound {
		t.Fatalf("cross-tenant update should be CodeNotFound, got %v", err)
	}
	listB, err := svc.ListAppBundles(ctx, &model.GetAppBundleListReq{PageReq: model.PageReq{Page: 1, PageSize: 20}}, "tenant-b")
	if err != nil {
		t.Fatalf("list tenant-b: %v", err)
	}
	if listB.Total != 0 || len(listB.List) != 0 {
		t.Fatalf("tenant-b list must be empty, got total=%d", listB.Total)
	}

	// tenant-b 可以登记同平台同版本（唯一键是 (tenant_id, platform, version)）。
	createAppBundleForTest(t, "tenant-b", model.AppBundlePlatformAndroid, "3.0.0")
	listA, err := svc.ListAppBundles(ctx, &model.GetAppBundleListReq{PageReq: model.PageReq{Page: 1, PageSize: 20}}, "tenant-a")
	if err != nil {
		t.Fatalf("list tenant-a: %v", err)
	}
	if listA.Total != 1 {
		t.Fatalf("tenant-a list total = %d, want 1 (isolation)", listA.Total)
	}

	// 空租户一律 fail-closed。
	emptyTenantCode := errcode.CodeNoPermission
	if _, err := svc.ListAppBundles(ctx, &model.GetAppBundleListReq{PageReq: model.PageReq{Page: 1, PageSize: 20}}, " "); bizCode(t, err) != emptyTenantCode {
		t.Fatalf("empty tenant list should be CodeNoPermission, got %v", err)
	}
	if _, err := svc.GetAppBundle(ctx, record.ID, ""); bizCode(t, err) != emptyTenantCode {
		t.Fatalf("empty tenant get should be CodeNoPermission, got %v", err)
	}
}

// TestAppBundleDownloadPathWhitelist 下载路径白名单：仅 files/apps/ 内路径可解析，逃逸路径拒绝。
func TestAppBundleDownloadPathWhitelist(t *testing.T) {
	setupAppBundleServiceTestDB(t)
	svc := &MobileAppBundleService{}
	ctx := context.Background()
	db := global.DB

	now := time.Now().UTC()
	cases := []struct {
		name     string
		filePath string
		wantErr  bool
	}{
		{"正常路径", "./files/apps/android/2026-01-01/abc.apk", false},
		{"无点斜杠前缀", "files/apps/h5/2026-01-01/abc.zip", false},
		{"非 apps 业务目录", "./files/board/2026-01-01/pic.png", true},
		{"OTA 前缀不放行", "./api/v1/ota/download/files/apps/a.apk", true},
		{"借道逃逸出 apps", "./files/apps/android/../../secret.apk", true},
		{"逃逸出存储根", "./files/apps/../../../etc/passwd", true},
		{"空路径", "  ", true},
	}
	for i, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			record := &model.MobileAppBundle{
				ID: fmt.Sprintf("dl-%d", i), TenantID: "tenant-a", Platform: "android", Version: fmt.Sprintf("1.0.%d", i),
				FileName: "a.apk", FilePath: testCase.filePath, Checksum: "x", Status: model.AppBundleStatusPublished,
				CreatedAt: &now, UpdatedAt: &now,
			}
			if err := db.Create(record).Error; err != nil {
				t.Fatalf("seed bundle: %v", err)
			}
			got, err := svc.AppBundleDownload(ctx, record.ID, "tenant-a")
			if testCase.wantErr {
				if err == nil {
					t.Fatalf("path %q should be rejected, got record %+v", testCase.filePath, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("path %q should be accepted, got %v", testCase.filePath, err)
			}
		})
	}

	// 下载同样受租户隔离。
	if _, err := svc.AppBundleDownload(ctx, "dl-0", "tenant-b"); bizCode(t, err) != errcode.CodeNotFound {
		t.Fatalf("cross-tenant download should be CodeNotFound, got %v", err)
	}
}

// TestListAppBundlesFilters 列表的 platform/status 过滤与分页夹紧。
func TestListAppBundlesFilters(t *testing.T) {
	setupAppBundleServiceTestDB(t)
	svc := &MobileAppBundleService{}
	ctx := context.Background()

	createAppBundleForTest(t, "tenant-a", model.AppBundlePlatformAndroid, "1.0.0")
	createAppBundleForTest(t, "tenant-a", model.AppBundlePlatformAndroid, "1.1.0")
	createAppBundleForTest(t, "tenant-a", model.AppBundlePlatformH5, "1.0.0")

	platform := model.AppBundlePlatformAndroid
	status := model.AppBundleStatusDraft
	got, err := svc.ListAppBundles(ctx, &model.GetAppBundleListReq{PageReq: model.PageReq{Page: 1, PageSize: 20}, Platform: &platform, Status: &status}, "tenant-a")
	if err != nil {
		t.Fatalf("list android: %v", err)
	}
	if got.Total != 2 {
		t.Fatalf("android draft total = %d, want 2", got.Total)
	}

	platformH5 := model.AppBundlePlatformH5
	gotH5, err := svc.ListAppBundles(ctx, &model.GetAppBundleListReq{PageReq: model.PageReq{Page: 1, PageSize: 20}, Platform: &platformH5}, "tenant-a")
	if err != nil {
		t.Fatalf("list h5: %v", err)
	}
	if gotH5.Total != 1 {
		t.Fatalf("h5 total = %d, want 1", gotH5.Total)
	}

	// 分页参数非法时夹紧为默认页（page=0 → 第 1 页，PageSize=0 → 20）。
	gotPage, err := svc.ListAppBundles(ctx, &model.GetAppBundleListReq{}, "tenant-a")
	if err != nil {
		t.Fatalf("list default page: %v", err)
	}
	if gotPage.Total != 3 || len(gotPage.List) != 3 {
		t.Fatalf("default page total=%d len=%d, want 3/3", gotPage.Total, len(gotPage.List))
	}
}

// TestInvalidTransitionErrorMessageTemplate 非法流转错误携带渲染变量（${operation}/${status}）。
func TestInvalidTransitionErrorMessageTemplate(t *testing.T) {
	rawErr := invalidTransitionError(AppBundleActionPublish, model.AppBundleStatusArchived)
	var bizErr *errcode.Error
	if !errors.As(rawErr, &bizErr) {
		t.Fatalf("expected errcode.Error, got %T", rawErr)
	}
	if bizErr.Variables["operation"] != AppBundleActionPublish || bizErr.Variables["status"] != model.AppBundleStatusArchived {
		t.Fatalf("variables = %+v", bizErr.Variables)
	}
	if bizErr.Code != errcode.CodeAppBundleInvalidTransition {
		t.Fatalf("code = %d", bizErr.Code)
	}
}
