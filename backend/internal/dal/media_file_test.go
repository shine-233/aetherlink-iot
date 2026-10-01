// 文件用途：媒体库 DAL 层（TB-41）单元测试。
// 核心逻辑：租户隔离的分页检索（search/mime 过滤）、按路径幂等查询、
//
//	引用扫描在三张引用面（boards/scada_documents/ota_upgrade_packages）上的命中与跨租户不误报。
//
// 关键注意事项：全部用例走 sqlite 内存库并 AutoMigrate 仅相关表；
//
//	引用扫描是 LIKE 包含匹配，测试同时覆盖带 "./" 与不带前缀的两种引用写法。
//
// 重构建议：引用面扩展时在这里补对应用例，保证新扫描面始终带 tenant_id 过滤。
package dal

import (
	"context"
	"fmt"
	"strings"
	"testing"

	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	dalMediaTenantA = "dal-tenant-a"
	dalMediaTenantB = "dal-tenant-b"
	dalMediaPath    = "./files/board/2026-09-25/0123abcdef567890.png"
)

// setupMediaFileDALTestDB 建立仅含媒体登记与引用面表的 sqlite 内存库。
func setupMediaFileDALTestDB(t *testing.T) {
	t.Helper()
	oldDB := global.DB
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:media_dal_%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))), &gorm.Config{})
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

func TestListMediaFilesForScopePaginationAndFilters(t *testing.T) {
	setupMediaFileDALTestDB(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		require.NoError(t, CreateMediaFile(&model.MediaFile{
			ID:       testMediaID(i),
			TenantID: dalMediaTenantA,
			FileName: "photo.png",
			FilePath: "./files/board/2026-09-25/img" + string(rune('a'+i)) + ".png",
			FileSize: 100,
			Mime:     "image/png",
		}))
	}
	require.NoError(t, CreateMediaFile(&model.MediaFile{
		ID:       testMediaID(9),
		TenantID: dalMediaTenantB,
		FileName: "other.png",
		FilePath: "./files/board/2026-09-25/other.png",
		Mime:     "image/png",
	}))

	page := 1
	pageSize := 2
	total, list, err := ListMediaFilesForScope(ctx, &model.GetMediaFileListReq{
		PageReq: model.PageReq{Page: page, PageSize: pageSize},
	}, dalMediaTenantA)
	require.NoError(t, err)
	require.EqualValues(t, 3, total, "租户 A 只有 3 条登记")
	require.Len(t, list, 2, "每页 2 条")
	for _, row := range list {
		require.Equal(t, dalMediaTenantA, row.TenantID, "分页结果不得混入其他租户")
	}

	search := "photo"
	_, bySearch, err := ListMediaFilesForScope(ctx, &model.GetMediaFileListReq{
		PageReq: model.PageReq{Page: 1, PageSize: 10}, Search: &search,
	}, dalMediaTenantA)
	require.NoError(t, err)
	require.Len(t, bySearch, 3)

	// search 是 LIKE 字面量匹配：通配符不得放大结果。
	wildcard := "%"
	_, byWildcard, err := ListMediaFilesForScope(ctx, &model.GetMediaFileListReq{
		PageReq: model.PageReq{Page: 1, PageSize: 10}, Search: &wildcard,
	}, dalMediaTenantA)
	require.NoError(t, err)
	require.Len(t, byWildcard, 0, "通配符搜索必须被转义为字面量")

	mime := "application/pdf"
	_, byMime, err := ListMediaFilesForScope(ctx, &model.GetMediaFileListReq{
		PageReq: model.PageReq{Page: 1, PageSize: 10}, Mime: &mime,
	}, dalMediaTenantA)
	require.NoError(t, err)
	require.Len(t, byMime, 0)
}

func TestCreateMediaFileConflictIsIdempotent(t *testing.T) {
	setupMediaFileDALTestDB(t)
	ctx := context.Background()

	first := &model.MediaFile{ID: "m1", TenantID: dalMediaTenantA, FileName: "a.png", FilePath: dalMediaPath, Mime: "image/png"}
	require.NoError(t, CreateMediaFile(first))
	second := &model.MediaFile{ID: "m2", TenantID: dalMediaTenantA, FileName: "a.png", FilePath: dalMediaPath, Mime: "image/png"}
	require.NoError(t, CreateMediaFile(second), "唯一冲突按幂等处理不报错")

	existing, err := GetMediaFileByPathInTenant(ctx, dalMediaTenantA, dalMediaPath)
	require.NoError(t, err)
	require.Equal(t, "m1", existing.ID, "幂等语义下保留先落的那一行")

	var count int64
	require.NoError(t, global.DB.Model(&model.MediaFile{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestCountMediaReferencesForPathAcrossSurfaces(t *testing.T) {
	setupMediaFileDALTestDB(t)
	ctx := context.Background()

	// 引用面 1：看板 config（带 "./" 前缀写法）+ 租户 B 同路径（不得误报）。
	require.NoError(t, global.DB.Create(&model.Board{
		ID: "board-a", Name: "看板A", TenantID: dalMediaTenantA,
		Config: strPtrDal(`{"img":"./files/board/2026-09-25/0123abcdef567890.png"}`),
	}).Error)
	require.NoError(t, global.DB.Create(&model.Board{
		ID: "board-b", Name: "看板B", TenantID: dalMediaTenantB,
		Config: strPtrDal(`{"img":"./files/board/2026-09-25/0123abcdef567890.png"}`),
	}).Error)

	// 引用面 2：SCADA 文档 json_data（不带前缀写法）。
	require.NoError(t, global.DB.Create(&model.ScadaDocument{
		ID: "doc-a", TenantID: dalMediaTenantA, ProjectID: "p1", Name: "工艺图",
		Status: "draft", CurrentVersion: 1,
		JSONData: strPtrDal(`{"widgets":[{"img":"files/board/2026-09-25/0123abcdef567890.png"}]}`),
	}).Error)

	// 引用面 3：OTA 升级包 package_url（不命中本媒体路径）。
	require.NoError(t, global.DB.Create(&model.OtaUpgradePackage{
		ID: "ota-a", Name: "固件包", Version: "1.0.0", DeviceConfigID: "dc1", PackageType: 2,
		PackageURL: strPtrDal("./api/v1/ota/download/files/upgradePackage/2026-09-25/fw.bin"),
		TenantID:   strPtrDal(dalMediaTenantA),
	}).Error)

	referencers, err := CountMediaReferencesForPath(ctx, dalMediaTenantA, dalMediaPath)
	require.NoError(t, err)
	require.Len(t, referencers, 2, "租户 A 应命中看板+SCADA 文档两个引用方")

	kinds := map[string]string{}
	for _, ref := range referencers {
		kinds[ref.Kind] = ref.ID
	}
	require.Equal(t, "board-a", kinds["board"])
	require.Equal(t, "doc-a", kinds["scada_document"])

	// 租户 B 扫同一路径只能看到自己的看板。
	refsB, err := CountMediaReferencesForPath(ctx, dalMediaTenantB, dalMediaPath)
	require.NoError(t, err)
	require.Len(t, refsB, 1)
	require.Equal(t, "board-b", refsB[0].ID)

	// 未被引用的路径返回空列表。
	refsNone, err := CountMediaReferencesForPath(ctx, dalMediaTenantA, "./files/logo/2026-09-25/ffff0000ffff0000.png")
	require.NoError(t, err)
	require.Empty(t, refsNone)
}

// testMediaID 生成稳定的测试登记 id。
func testMediaID(i int) string {
	return "media-" + string(rune('a'+i))
}

// strPtrDal 字符串指针 helper（dal 测试包内）。
func strPtrDal(s string) *string {
	return &s
}
