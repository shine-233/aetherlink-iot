// 文件用途：TP-22 大屏轮播服务层的单元测试——token 清洗、批量解析、顺序保持、
// 缺失回填与公开面 fail-closed 语义。
// 核心逻辑：内存 SQLite 起隔离库（boards + 看板项目两张表），种已发布/未发布/
// 非原生看板后，验证 ParseCarouselTokens 与 GetPublishedBoardsForCarousel 的行为锁。
// 关键注意事项：Board DAL 走 query 包默认库，夹具必须 query.SetDefault(db) 并在
// 清理时恢复，否则会污染同进程其他用例的库句柄。
// 重构建议：新增"按项目整组轮播"等解析语义时，在本文件补对应行为锁再实现。
package service

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/errcode"
	global "aetherlink-iot/backend/pkg/global"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupBoardCarouselTestDB 为轮播用例建隔离内存库并接管 global.DB / query 默认库。
func setupBoardCarouselTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := global.DB
	dbName := "board_carousel_" + strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Board{}, &model.BoardProject{}, &model.BoardProjectMember{}); err != nil {
		t.Fatalf("migrate boards: %v", err)
	}
	global.DB = db
	query.SetDefault(db)
	t.Cleanup(func() {
		global.DB = oldDB
		if oldDB != nil {
			query.SetDefault(oldDB)
		}
	})
	return db
}

// seedCarouselBoard 直接落库一块看板，绕开服务层（夹具只关心数据形态）。
func seedCarouselBoard(t *testing.T, db *gorm.DB, id, tenantID, name string, published bool, visType, shareToken *string) {
	t.Helper()
	now := time.Now().UTC()
	board := &model.Board{
		ID:         id,
		Name:       name,
		TenantID:   tenantID,
		HomeFlag:   "N",
		CreatedAt:  now,
		UpdatedAt:  now,
		VisType:    visType,
		ShareToken: shareToken,
		Published:  published,
	}
	if err := db.Create(board).Error; err != nil {
		t.Fatalf("seed board %s: %v", id, err)
	}
}

// seedStrPtr 取字符串指针（命名避开包内既有 strPtr 助手）。
func seedStrPtr(s string) *string { return &s }

// TestParseCarouselTokens 锁定 token 清洗规则。
func TestParseCarouselTokens(t *testing.T) {
	t.Run("逗号分隔与重复参数等价", func(t *testing.T) {
		a, err := ParseCarouselTokens([]string{"tok-b, tok-a,,tok-c"})
		if err != nil {
			t.Fatalf("ParseCarouselTokens: %v", err)
		}
		b, err := ParseCarouselTokens([]string{"tok-b", "tok-a", "tok-c"})
		if err != nil {
			t.Fatalf("ParseCarouselTokens: %v", err)
		}
		if fmt.Sprint(a) != fmt.Sprint(b) || len(a) != 3 {
			t.Fatalf("parse forms diverge: %v vs %v", a, b)
		}
	})
	t.Run("去重保持首次出现顺序", func(t *testing.T) {
		out, err := ParseCarouselTokens([]string{"tok-b,tok-a,tok-b"})
		if err != nil {
			t.Fatalf("ParseCarouselTokens: %v", err)
		}
		if fmt.Sprint(out) != "[tok-b tok-a]" {
			t.Fatalf("out = %v, want [tok-b tok-a]", out)
		}
	})
	t.Run("全空参数报参数错误", func(t *testing.T) {
		for _, raw := range [][]string{nil, {""}, {" , "}} {
			_, err := ParseCarouselTokens(raw)
			assertErrcodeError(t, err, "empty token list", errcode.CodeParamError, "tokens is required (comma-separated share tokens)")
		}
	})
	t.Run("超长 token 拒绝", func(t *testing.T) {
		_, err := ParseCarouselTokens([]string{strings.Repeat("x", BoardShareTokenMaxLen+1)})
		assertErrcodeError(t, err, "oversized token", errcode.CodeParamError, "share token is too long")
	})
	t.Run("超上限数量拒绝", func(t *testing.T) {
		raw := make([]string, 0, BoardCarouselMaxTokens+1)
		for i := 0; i < BoardCarouselMaxTokens+1; i++ {
			raw = append(raw, fmt.Sprintf("tok-%02d", i))
		}
		_, err := ParseCarouselTokens(raw)
		if err == nil || err.(*errcode.Error).Code != errcode.CodeParamError {
			t.Fatalf("err = %v, want param error", err)
		}
		// 上限值本身必须可用。
		if _, err := ParseCarouselTokens(raw[:BoardCarouselMaxTokens]); err != nil {
			t.Fatalf("exactly max tokens must pass: %v", err)
		}
	})
}

// TestGetPublishedBoardsForCarousel 锁定批量解析行为：
// 顺序保持、缺失回填、未发布/非原生不可见（公开面 fail-closed）。
func TestGetPublishedBoardsForCarousel(t *testing.T) {
	db := setupBoardCarouselTestDB(t)

	seedCarouselBoard(t, db, "board-a", "tenant-a", "产线大屏 A", true, seedStrPtr("native"), seedStrPtr("tok-a"))
	seedCarouselBoard(t, db, "board-b", "tenant-a", "产线大屏 B", true, seedStrPtr("native"), seedStrPtr("tok-b"))
	// 跨租户已发布：share token 是凭证，跨租户解析是设计语义而非越权。
	seedCarouselBoard(t, db, "board-c", "tenant-b", "园区大屏 C", true, seedStrPtr("native"), seedStrPtr("tok-c"))
	seedCarouselBoard(t, db, "board-u", "tenant-a", "未发布", false, seedStrPtr("native"), seedStrPtr("tok-u"))
	seedCarouselBoard(t, db, "board-n", "tenant-a", "非原生", true, seedStrPtr("thingsvis"), seedStrPtr("tok-n"))
	seedCarouselBoard(t, db, "board-p", "tenant-a", "无 token", true, seedStrPtr("native"), nil)

	result, err := (&Board{}).GetPublishedBoardsForCarousel([]string{"tok-b", "tok-gone", "tok-a", "tok-b", "tok-u", "tok-n"})
	if err != nil {
		t.Fatalf("GetPublishedBoardsForCarousel: %v", err)
	}

	gotIDs := make([]string, 0, len(result.Items))
	for _, board := range result.Items {
		gotIDs = append(gotIDs, board.ID)
	}
	// tok-b 请求在前就排在前；tok-b 重复只出现一次；tok-u/tok-n/tok-gone 不可见。
	if fmt.Sprint(gotIDs) != "[board-b board-a]" {
		t.Fatalf("items = %v, want [board-b board-a]", gotIDs)
	}
	if fmt.Sprint(result.MissingTokens) != "[tok-gone tok-u tok-n]" {
		t.Fatalf("missing_tokens = %v, want [tok-gone tok-u tok-n]", result.MissingTokens)
	}
	// 跨租户已发布看板按凭证可见（与单看板公开端点同语义）。
	foundC := false
	for _, board := range result.Items {
		if board.ID == "board-c" {
			foundC = true
		}
	}
	if foundC {
		t.Fatal("tok-c was not requested; it must not leak into items")
	}

	t.Run("全部缺失返回空 items 而非错误", func(t *testing.T) {
		res, err := (&Board{}).GetPublishedBoardsForCarousel([]string{"tok-x", "tok-y"})
		if err != nil {
			t.Fatalf("all-missing playlist must not error: %v", err)
		}
		if len(res.Items) != 0 || fmt.Sprint(res.MissingTokens) != "[tok-x tok-y]" {
			t.Fatalf("res = %+v, want empty items with both tokens missing", res)
		}
	})

	t.Run("空参数报参数错误", func(t *testing.T) {
		_, err := (&Board{}).GetPublishedBoardsForCarousel(nil)
		assertErrcodeError(t, err, "nil token list", errcode.CodeParamError, "tokens is required (comma-separated share tokens)")
	})
}
