// 文件用途：媒体库服务（TB-41）单元测试。
// 核心逻辑：上传落登记（租户缺失跳过/幂等）、列表与详情的租户隔离、删除的引用拒绝
//
//	（看板 config 引用 → 202004 + 引用方列表）与无引用时的文件+行删除、磁盘路径解析边界。
//
// 关键注意事项：全部 DB 用例走 sqlite 内存库并 AutoMigrate 仅媒体相关表，
//
//	不依赖真实 PostgreSQL；真实磁盘删除用例在包目录 ./files 下创建并清理临时文件。
//
// 重构建议：若引用面扩展（如 email_templates 附件），同步扩展引用扫描测试矩阵。
package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupMediaLibraryTestDB 建立仅含媒体登记与三个引用面表的 sqlite 内存库。
func setupMediaLibraryTestDB(t *testing.T) {
	t.Helper()
	oldDB := global.DB
	dbName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.MediaFile{},
		&model.Board{},
		&model.ScadaDocument{},
		&model.OtaUpgradePackage{},
	))
	global.DB = db
	t.Cleanup(func() { global.DB = oldDB })
}

const (
	mediaTestTenantA = "tenant-a-0001"
	mediaTestTenantB = "tenant-b-0002"
)

var mediaTestClaimsA = &utils.UserClaims{ID: "user-a", TenantID: mediaTestTenantA, Authority: "TENANT_ADMIN"}

func mediaTestRegistration() MediaUploadRegistration {
	return MediaUploadRegistration{
		FileName: "site-photo.png",
		FilePath: "./files/board/2026-09-25/abcdef1234567890.png",
		FileSize: 2048,
		Mime:     "image/png",
	}
}

// seedMediaFile 在租户 A 落一条登记，返回 id。
func seedMediaFile(t *testing.T, ctx context.Context) string {
	t.Helper()
	record, err := GroupApp.MediaLibrary.RegisterMediaUpload(ctx, mediaTestRegistration(), mediaTestClaimsA)
	require.NoError(t, err)
	require.NotNil(t, record)
	return record.ID
}

func TestRegisterMediaUploadSkipsWithoutTenant(t *testing.T) {
	setupMediaLibraryTestDB(t)
	ctx := context.Background()

	record, err := GroupApp.MediaLibrary.RegisterMediaUpload(ctx, mediaTestRegistration(), nil)
	require.NoError(t, err)
	require.Nil(t, record, "claims 为 nil 时应跳过登记且不报错")

	orphanClaims := &utils.UserClaims{ID: "user-x", TenantID: ""}
	record, err = GroupApp.MediaLibrary.RegisterMediaUpload(ctx, mediaTestRegistration(), orphanClaims)
	require.NoError(t, err)
	require.Nil(t, record, "claims 缺租户时应跳过登记且不报错")
}

func TestRegisterMediaUploadIsIdempotentByPath(t *testing.T) {
	setupMediaLibraryTestDB(t)
	ctx := context.Background()

	first, err := GroupApp.MediaLibrary.RegisterMediaUpload(ctx, mediaTestRegistration(), mediaTestClaimsA)
	require.NoError(t, err)
	second, err := GroupApp.MediaLibrary.RegisterMediaUpload(ctx, mediaTestRegistration(), mediaTestClaimsA)
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID, "同租户同路径应幂等返回既有登记")
}

func TestRegisterMediaUploadRejectsEmptyPath(t *testing.T) {
	setupMediaLibraryTestDB(t)
	ctx := context.Background()

	_, err := GroupApp.MediaLibrary.RegisterMediaUpload(ctx, MediaUploadRegistration{FilePath: "  "}, mediaTestClaimsA)
	require.Error(t, err)
}

func TestListMediaFilesIsTenantScoped(t *testing.T) {
	setupMediaLibraryTestDB(t)
	ctx := context.Background()
	seedMediaFile(t, ctx)

	listReq := &model.GetMediaFileListReq{PageReq: model.PageReq{Page: 1, PageSize: 10}}

	// 租户 A 能看到自己的登记。
	own, err := GroupApp.MediaLibrary.ListMediaFiles(ctx, listReq, mediaTestClaimsA)
	require.NoError(t, err)
	require.EqualValues(t, 1, own.Total)

	// 租户 B 看不到租户 A 的登记。
	other, err := GroupApp.MediaLibrary.ListMediaFiles(ctx, listReq, &utils.UserClaims{ID: "user-b", TenantID: mediaTestTenantB})
	require.NoError(t, err)
	require.EqualValues(t, 0, other.Total)
	require.Empty(t, other.List)
}

func TestListMediaFilesSearchAndMimeFilter(t *testing.T) {
	setupMediaLibraryTestDB(t)
	ctx := context.Background()
	seedMediaFile(t, ctx)
	_, err := GroupApp.MediaLibrary.RegisterMediaUpload(ctx, MediaUploadRegistration{
		FileName: "spec.pdf",
		FilePath: "./files/board/2026-09-25/1111111111111111.pdf",
		FileSize: 1,
		Mime:     "application/pdf",
	}, mediaTestClaimsA)
	require.NoError(t, err)

	search := "site-photo"
	hit, err := GroupApp.MediaLibrary.ListMediaFiles(ctx, &model.GetMediaFileListReq{
		PageReq: model.PageReq{Page: 1, PageSize: 10}, Search: &search,
	}, mediaTestClaimsA)
	require.NoError(t, err)
	require.EqualValues(t, 1, hit.Total)
	require.Equal(t, "site-photo.png", hit.List[0].FileName)

	mime := "application/pdf"
	byMime, err := GroupApp.MediaLibrary.ListMediaFiles(ctx, &model.GetMediaFileListReq{
		PageReq: model.PageReq{Page: 1, PageSize: 10}, Mime: &mime,
	}, mediaTestClaimsA)
	require.NoError(t, err)
	require.EqualValues(t, 1, byMime.Total)
	require.Equal(t, "spec.pdf", byMime.List[0].FileName)
}

func TestGetMediaFileDetailRefreshesReferencedCount(t *testing.T) {
	setupMediaLibraryTestDB(t)
	ctx := context.Background()
	mediaID := seedMediaFile(t, ctx)

	// 无引用时详情返回 0。
	detail, err := GroupApp.MediaLibrary.GetMediaFileDetail(ctx, mediaID, mediaTestClaimsA)
	require.NoError(t, err)
	require.Equal(t, 0, detail.File.ReferencedCount)
	require.Empty(t, detail.Referencers)

	// 看板 config 引用该路径后，详情实时刷新计数并返回引用方。
	board := &model.Board{ID: "board-1", Name: "大屏一", TenantID: mediaTestTenantA, Config: strPtr(`{"img":"./files/board/2026-09-25/abcdef1234567890.png"}`)}
	require.NoError(t, global.DB.Create(board).Error)

	detail, err = GroupApp.MediaLibrary.GetMediaFileDetail(ctx, mediaID, mediaTestClaimsA)
	require.NoError(t, err)
	require.Equal(t, 1, detail.File.ReferencedCount)
	require.Len(t, detail.Referencers, 1)
	require.Equal(t, "board", detail.Referencers[0].Kind)
	require.Equal(t, "board-1", detail.Referencers[0].ID)

	// 跨租户详情不可见。
	_, err = GroupApp.MediaLibrary.GetMediaFileDetail(ctx, mediaID, &utils.UserClaims{ID: "user-b", TenantID: mediaTestTenantB})
	require.Error(t, err, "租户 B 查询租户 A 的媒体详情必须失败")
}

func TestDeleteMediaFileRejectsWhenReferenced(t *testing.T) {
	setupMediaLibraryTestDB(t)
	ctx := context.Background()
	mediaID := seedMediaFile(t, ctx)

	board := &model.Board{ID: "board-ref", Name: "引用看板", TenantID: mediaTestTenantA, Config: strPtr(`{"img":"files/board/2026-09-25/abcdef1234567890.png"}`)}
	require.NoError(t, global.DB.Create(board).Error)

	_, err := GroupApp.MediaLibrary.DeleteMediaFile(ctx, mediaID, mediaTestClaimsA)
	require.Error(t, err)
	var errcodeErr *errcode.Error
	require.True(t, errors.As(err, &errcodeErr))
	require.Equal(t, errcode.CodeMediaReferenced, errcodeErr.Code)
	require.NotNil(t, errcodeErr.Data, "拒绝删除必须携带引用方列表")
	data, ok := errcodeErr.Data.(map[string]interface{})
	require.True(t, ok)
	referencers, ok := data["referencers"].([]model.MediaReferencer)
	require.True(t, ok)
	require.Len(t, referencers, 1)
	require.Equal(t, "board-ref", referencers[0].ID)
	require.Equal(t, 1, data["referenced_count"])

	// 登记行仍在，且计数已回写。
	row, err := GroupApp.MediaLibrary.GetMediaFileDetail(ctx, mediaID, mediaTestClaimsA)
	require.NoError(t, err)
	require.Equal(t, 1, row.File.ReferencedCount)
}

func TestDeleteMediaFileRemovesFileAndRowWhenUnreferenced(t *testing.T) {
	setupMediaLibraryTestDB(t)
	ctx := context.Background()
	mediaID := seedMediaFile(t, ctx)

	// 真实落一个磁盘文件（相对当前测试包目录的 ./files/...），删除后必须消失。
	absDir := filepath.Join("files", "board", "2026-09-25")
	require.NoError(t, os.MkdirAll(absDir, 0o755))
	absFile := filepath.Join(absDir, "abcdef1234567890.png")
	require.NoError(t, os.WriteFile(absFile, []byte("png"), 0o600))
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Join("files", "board", "2026-09-25")) })

	rsp, err := GroupApp.MediaLibrary.DeleteMediaFile(ctx, mediaID, mediaTestClaimsA)
	require.NoError(t, err)
	require.True(t, rsp.Deleted)

	_, statErr := os.Stat(absFile)
	require.True(t, os.IsNotExist(statErr), "磁盘文件必须被删除")
	// 登记行必须被删除。
	var count int64
	require.NoError(t, global.DB.Model(&model.MediaFile{}).Where("tenant_id = ?", mediaTestTenantA).Count(&count).Error)
	require.EqualValues(t, 0, count)

	// 再删一次：文件与行都不存在，返回 not found。
	_, err = GroupApp.MediaLibrary.DeleteMediaFile(ctx, mediaID, mediaTestClaimsA)
	require.Error(t, err)
}

func TestDeleteMediaFileTenantIsolation(t *testing.T) {
	setupMediaLibraryTestDB(t)
	ctx := context.Background()
	mediaID := seedMediaFile(t, ctx)

	_, err := GroupApp.MediaLibrary.DeleteMediaFile(ctx, mediaID, &utils.UserClaims{ID: "user-b", TenantID: mediaTestTenantB})
	require.Error(t, err, "租户 B 删除租户 A 的媒体必须失败")

	var count int64
	require.NoError(t, global.DB.Model(&model.MediaFile{}).Where("id = ?", mediaID).Count(&count).Error)
	require.EqualValues(t, 1, count, "跨租户删除不得删行")
}

func TestResolveMediaDiskRelativePath(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "normal upload path", input: "./files/board/2026-09-25/a.png", want: "board/2026-09-25/a.png"},
		{name: "without dot slash", input: "files/board/a.png", want: "board/a.png"},
		{name: "ota download path", input: "./api/v1/ota/download/files/upgradePackage/2026-09-25/fw.bin", want: "upgradePackage/2026-09-25/fw.bin"},
		{name: "empty", input: "   ", wantErr: true},
		{name: "outside upload root", input: "./uploads/other/a.png", wantErr: true},
		{name: "traversal", input: "./files/../secret.txt", wantErr: true},
		{name: "dot segment", input: "./files/./a.png", want: "a.png"},
		{name: "windows colon", input: `files\C:\evil.png`, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveMediaDiskRelativePath(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestResolveMediaDiskRelativePathRejectsNestedTraversal(t *testing.T) {
	_, err := resolveMediaDiskRelativePath("./files/board/../../secret.txt")
	require.Error(t, err, "嵌套 .. 必须被拒绝")
}
