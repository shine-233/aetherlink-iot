// 文件用途：TP-22 大屏轮播/投屏的服务层——把 /tv-preview 公开投屏页需要的
// "多块已发布看板"按 share token 列表一次性解析出来。
// 核心逻辑：token 列表清洗（逗号分隔与重复参数两种形式、去空、按首次出现去重、
// 数量与长度上限）→ DAL 批量取已发布原生看板 → 按请求顺序重排并回填缺失清单。
// 关键注意事项：
//  1. 公开端点不接受 board id：ID 是内部标识，公开路由按 ID 取数等于把
//     "枚举 ID 看别人看板"的口子开在无认证面上。share token 是发布方主动
//     公开的凭证，泄露面只有发布者自己公开的那一块，与既有单看板公开端点同语义。
//  2. 单块看板失效（取消发布/换 token）不能炸掉整份轮播清单：解析结果显式
//     分为 items 与 missing_tokens，由投屏端决定跳过还是提示——静默丢弃与
//     整体报错都会让电视墙黑屏且无从排查。
//  3. 数量超限是参数错误而不是静默截断：截断会让第 21 块之后的看板"消失"
//     且调用方毫无感知。
//
// 重构建议：若后续要支持"按项目整组轮播"，在此层展开项目 → token 列表，
// 保持公开面只认凭证不认资源 ID 的边界。
package service

import (
	"fmt"
	"strings"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
)

// 轮播清单约束。
const (
	// BoardCarouselMaxTokens 单份轮播清单最多看板数：电视墙场景远用不到更多，
	// 上限只为把公开端点的批量取数成本封顶。
	BoardCarouselMaxTokens = 20
	// BoardShareTokenMaxLen 单个 share token 长度上限（与单看板公开端点一致）。
	BoardShareTokenMaxLen = 64
)

// BoardCarouselResult 轮播解析结果。
type BoardCarouselResult struct {
	// Items 按 token 请求顺序排列的已发布原生看板。
	Items []model.Board `json:"items"`
	// MissingTokens 请求了但解析不到（未发布/非原生/未知）的 token，保持请求顺序。
	MissingTokens []string `json:"missing_tokens"`
}

// ParseCarouselTokens 清洗轮播 token 列表。
// 入参来自 gin 的 QueryArray("tokens")：`?tokens=a,b` 与 `?tokens=a&tokens=b`
// 两种写法都要能吃下；空段跳过，重复 token 按首次出现去重。
func ParseCarouselTokens(raw []string) ([]string, error) {
	seen := make(map[string]bool)
	out := make([]string, 0, len(raw))
	for _, group := range raw {
		for _, token := range strings.Split(group, ",") {
			token = strings.TrimSpace(token)
			if token == "" {
				continue
			}
			if len(token) > BoardShareTokenMaxLen {
				return nil, errcode.NewWithMessage(errcode.CodeParamError, "share token is too long")
			}
			if seen[token] {
				continue
			}
			seen[token] = true
			out = append(out, token)
		}
	}
	if len(out) == 0 {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "tokens is required (comma-separated share tokens)")
	}
	if len(out) > BoardCarouselMaxTokens {
		return nil, errcode.NewWithMessage(errcode.CodeParamError,
			fmt.Sprintf("a carousel playlist holds at most %d boards", BoardCarouselMaxTokens))
	}
	return out, nil
}

// GetPublishedBoardsForCarousel 按 share token 列表解析轮播看板。
// 只返回"已发布且原生"的看板；解析不到的 token 原样回填 missing_tokens，
// 不因个别失效把整份清单判死。
func (*Board) GetPublishedBoardsForCarousel(rawTokens []string) (*BoardCarouselResult, error) {
	tokens, err := ParseCarouselTokens(rawTokens)
	if err != nil {
		return nil, err
	}
	rows, err := dal.GetPublishedBoardsByShareTokens(tokens)
	if err != nil {
		return nil, wrapBoardDBError(err)
	}
	byToken := make(map[string]model.Board, len(rows))
	for _, board := range rows {
		if board == nil || board.ShareToken == nil || *board.ShareToken == "" {
			continue
		}
		byToken[*board.ShareToken] = *board
	}
	result := &BoardCarouselResult{Items: []model.Board{}, MissingTokens: []string{}}
	for _, token := range tokens {
		if board, ok := byToken[token]; ok {
			result.Items = append(result.Items, board)
		} else {
			result.MissingTokens = append(result.MissingTokens, token)
		}
	}
	return result, nil
}
