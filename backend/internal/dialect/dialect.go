// 文件用途：国产 DB 方言适配层（ROADMAP TP-20）——SQL 方言枚举、配置解析与方言能力矩阵。
// 核心逻辑：Parse 归一化显式 db.dialect 配置；FromTptodbType 把既有 grpc.tptodb_type 开关
// （NONE/TSDB/KINGBASE/POLARDB，与 internal/app externalTelemetryGRPCEnabled、internal/dal
// usesTelemetryQueryClient 的口径逐字一致）映射为方言；Effective 按"显式 db.dialect > tptodb 开关 >
// postgres 默认"取有效方言；Capabilities 给出各方言分页风格、元查询体系、标识符引用符与长度上限。
// 关键注意事项：本包是零 DB 依赖的纯 Go 适配层，不引入任何驱动（保持 CGO_ENABLED=0 交叉编译属性）；
// TDengine/KingBase 真实驱动接入与读写验证属 TP-20 residual，分支接线点见
// internal/dal/telemetry_datas.go usesTelemetryQueryClient 注释与 docs/deployment-domestic-db.md。
// 重构建议：原生驱动落地后，把 Effective 的结果接到 gorm Dialector 选择与元查询执行器；
// 届时只需扩充 Capabilities 与新增方言常量，调用方 API 不变。
package dialect

import (
	"errors"
	"fmt"
	"strings"
)

// Dialect SQL 方言标识。取值与 backend/internal/dialect 包内常量一一对应，
// 同时是 configs/conf.example.yml 中 db.dialect 的合法取值。
type Dialect string

const (
	// DialectPostgres 本地默认库 PostgreSQL（LIMIT n OFFSET m 分页 + information_schema 元查询）。
	DialectPostgres Dialect = "postgres"
	// DialectTDengine TDengine 时序库 3.x（LIMIT n OFFSET m 分页 + 内置 INFORMATION_SCHEMA 的
	// ins_tables/ins_columns 元查询视图，表名上限 192 字节）。
	DialectTDengine Dialect = "tdengine"
	// DialectKingbase KingbaseES（Oracle 兼容模式：ROWNUM 包裹分页 + user_tables/user_tab_columns
	// 字典视图，标识符按 Oracle 习惯大写归一）。
	DialectKingbase Dialect = "kingbase"
	// DialectPolardb PolarDB（按 MySQL 兼容版对待：反引号引用 + information_schema；
	// 若部署为 PolarDB-PG 版，运维可在 db.dialect 显式填 postgres 覆盖）。
	DialectPolardb Dialect = "polardb"
)

// ErrUnknownDialect 方言名不可识别（Parse 的哨兵错误，调用方可用 errors.Is 判定）。
var ErrUnknownDialect = errors.New("dialect: unknown dialect name")

// Parse 把显式方言配置串（db.dialect）归一化为 Dialect。
// 大小写不敏感、容忍首尾空白；空串视为未配置，返回错误（由 Effective 决定回退口径）。
// 注意 "tsdb"/"kingbase" 这类 tptodb_type 取值不属于本函数的合法输入——
// tptodb 开关请走 FromTptodbType，两者刻意分开以免语义混串。
func Parse(raw string) (Dialect, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "postgres", "postgresql", "pg":
		return DialectPostgres, nil
	case "tdengine", "td":
		return DialectTDengine, nil
	case "kingbase", "kingbasees":
		return DialectKingbase, nil
	case "polardb":
		return DialectPolardb, nil
	case "":
		return "", fmt.Errorf("%w: empty dialect name", ErrUnknownDialect)
	default:
		return "", fmt.Errorf("%w: %q", ErrUnknownDialect, raw)
	}
}

// FromTptodbType 把 grpc.tptodb_type 开关映射为方言。
// TSDB→TDengine、KINGBASE→KingBase、POLARDB→PolarDB；NONE/空串/未知值返回 (零值,false)，
// 表示未启用外置库（本地 PostgreSQL 默认口径）。归一化口径（ToUpper+TrimSpace）与
// internal/dal usesTelemetryQueryClient 保持逐字一致，避免两处开关判定漂移。
func FromTptodbType(raw string) (Dialect, bool) {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "TSDB":
		return DialectTDengine, true
	case "KINGBASE":
		return DialectKingbase, true
	case "POLARDB":
		return DialectPolardb, true
	default:
		return "", false
	}
}

// Effective 取当前部署的有效方言，优先级：
//  1. 显式 db.dialect（Parse 成功才采纳；配置了但不认识 = fail closed 返回错误，绝不静默回退）；
//  2. grpc.tptodb_type 推导（FromTptodbType）；
//  3. 都未配置 → DialectPostgres（本地库默认口径）。
//
// 注意：db.dialect 与 grpc.tptodb_type 同时配置且指向不同方言时以 db.dialect 为准，
// 运维应保持两者一致；本函数无法校验服务端实际方言，一致性靠部署清单约束（见部署文档）。
func Effective(dialectConfig, tptodbType string) (Dialect, error) {
	if strings.TrimSpace(dialectConfig) != "" {
		d, err := Parse(dialectConfig)
		if err != nil {
			return "", err
		}
		return d, nil
	}
	if d, ok := FromTptodbType(tptodbType); ok {
		return d, nil
	}
	return DialectPostgres, nil
}

// PageStyle 分页语式：LIMIT/OFFSET 直列，或 Oracle 系 ROWNUM 包裹。
type PageStyle string

const (
	// PageStyleLimitOffset SQL 尾部追加 "LIMIT n OFFSET m"（PostgreSQL/TDengine/PolarDB）。
	PageStyleLimitOffset PageStyle = "limit-offset"
	// PageStyleRownum 双层 ROWNUM 包裹分页（KingBase Oracle 兼容模式）。
	PageStyleRownum PageStyle = "rownum"
)

// MetadataStyle 元查询体系：表存在性/列清单的字典视图族。
type MetadataStyle string

const (
	// MetadataInformationSchema 标准 information_schema.tables / information_schema.columns（PostgreSQL、PolarDB）。
	MetadataInformationSchema MetadataStyle = "information_schema"
	// MetadataInsSchema TDengine 3.x 内置 INFORMATION_SCHEMA 的 ins_tables / ins_columns 视图。
	MetadataInsSchema MetadataStyle = "ins_schema"
	// MetadataOracleDict Oracle 兼容字典视图 user_tables / user_tab_columns（KingBase Oracle 模式）。
	MetadataOracleDict MetadataStyle = "oracle_dict"
)

// Capabilities 方言能力矩阵（机器可读，供分页/元查询生成器与未来 Dialector 选择分流）。
// MaxIdentifierLen 为生成 SQL 时的保守安全上限（字节）：KingBase 内核为 PG 血统按 63 取值，
// 即便 Oracle 模式放宽也以部署实际为准；TDengine 表名上限 192 字节；PolarDB 按 MySQL 系 64 取值。
type Capabilities struct {
	PageStyle        PageStyle     // 分页语式
	MetadataStyle    MetadataStyle // 元查询字典视图族
	QuoteChar        byte          // 标识符引用符：'"'（PG/KingBase）或 '`'（TDengine/PolarDB）
	UpperIdentifier  bool          // 元查询绑定参数前是否需把标识符大写归一（Oracle 系默认大写存储）
	MaxIdentifierLen int           // 标识符长度上限（字节）
}

// Capabilities 返回方言能力矩阵；未注册方言返回零值（调用方应先用 IsKnown 过滤）。
func (d Dialect) Capabilities() Capabilities {
	switch d {
	case DialectTDengine:
		return Capabilities{
			PageStyle:        PageStyleLimitOffset,
			MetadataStyle:    MetadataInsSchema,
			QuoteChar:        '`',
			UpperIdentifier:  false,
			MaxIdentifierLen: 192,
		}
	case DialectKingbase:
		return Capabilities{
			PageStyle:        PageStyleRownum,
			MetadataStyle:    MetadataOracleDict,
			QuoteChar:        '"',
			UpperIdentifier:  true,
			MaxIdentifierLen: 63,
		}
	case DialectPolardb:
		return Capabilities{
			PageStyle:        PageStyleLimitOffset,
			MetadataStyle:    MetadataInformationSchema,
			QuoteChar:        '`',
			UpperIdentifier:  false,
			MaxIdentifierLen: 64,
		}
	case DialectPostgres:
		return Capabilities{
			PageStyle:        PageStyleLimitOffset,
			MetadataStyle:    MetadataInformationSchema,
			QuoteChar:        '"',
			UpperIdentifier:  false,
			MaxIdentifierLen: 63,
		}
	default:
		return Capabilities{}
	}
}

// IsKnown 判断方言是否为本包注册的已知方言。所有 SQL 生成入口都应先校验，
// 未注册方言一律拒绝（fail closed），不猜默认。
func (d Dialect) IsKnown() bool {
	switch d {
	case DialectPostgres, DialectTDengine, DialectKingbase, DialectPolardb:
		return true
	default:
		return false
	}
}

// identifierMaxLen 全局兜底上限：即便方言矩阵异常也拒绝超长标识符。
const identifierMaxLen = 192

// ValidateIdentifier 校验 SQL 标识符（表名/列名）：字母或下划线开头、仅含字母数字下划线的
// ASCII 串，且不超过方言长度上限。元查询绑定参数与 QuoteIdentifier 都先过本函数——
// 这些名字会进入 SQL 文本或字典视图匹配，绝不允许注入面（分号/引号/空白一律拒绝）。
func ValidateIdentifier(name string, caps Capabilities) error {
	if name == "" {
		return errors.New("dialect: identifier is empty")
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c == '_' || ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z'):
		case i > 0 && '0' <= c && c <= '9':
		default:
			return fmt.Errorf("dialect: invalid identifier %q (only [A-Za-z0-9_] allowed, must not start with a digit)", name)
		}
	}
	maxLen := caps.MaxIdentifierLen
	if maxLen <= 0 || maxLen > identifierMaxLen {
		maxLen = identifierMaxLen
	}
	if len(name) > maxLen {
		return fmt.Errorf("dialect: identifier %q exceeds max length %d for this dialect", name, maxLen)
	}
	return nil
}

// QuoteIdentifier 按方言给标识符加引用符（PG/KingBase 用双引号，TDengine/PolarDB 用反引号）。
// 名字先经 ValidateIdentifier 校验；本函数不做大小写归一——是否大写由调用方按
// Capabilities.UpperIdentifier 决定（见 metadata.go 的绑定参数处理）。
func QuoteIdentifier(d Dialect, name string) (string, error) {
	if !d.IsKnown() {
		return "", fmt.Errorf("%w: %q", ErrUnknownDialect, string(d))
	}
	caps := d.Capabilities()
	if err := ValidateIdentifier(name, caps); err != nil {
		return "", err
	}
	q := string(caps.QuoteChar)
	return q + name + q, nil
}
