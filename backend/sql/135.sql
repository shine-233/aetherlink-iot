-- 135.sql — TB-19 通用 Protobuf 载荷编解码：data_converters 增加 PROTOBUF 转换模式
--
-- 背景：
--   1. 对标 ThingsBoard Data Converter：120.sql 已建 data_converters 表并支持
--      HEX_BINARY / JSON_PATH / SCRIPT 三种解析模式，通用 Protobuf 动态编解码缺位
--      （Sparkplug 为专用手写解码，无法覆盖自定义 proto 上报，ROADMAP TB-19）；
--   2. 本迁移为 data_converters 增加 proto_schema TEXT 列：存放用户上传的 .proto
--      源文件全文（syntax/package/message 定义）。运行期由服务层用
--      github.com/jhump/protoreflect 动态解析（protoparse）后按 configuration 的
--      {telemetry:{key:字段路径}} 映射解码消息为遥测，未知字段容错；
--   3. 列可空：仅 converter_mode='PROTOBUF' 的转换器要求非空（服务层 fail-closed
--      校验：模式为 PROTOBUF 但 proto_schema 为空时仿真/管线执行直接报错，
--      不猜测默认 schema）；存量行（其他模式）不受影响，无回填需要；
--   4. 范围排除（口径见 ROADMAP TB-19 批次三记录）：proto 文件的版本管理与
--      兼容性迁移不在本期——proto_schema 为整列覆盖式存储，不做历史版本留存；
--   5. 无新增路由：复用 120.sql 已登记的 /api/v1/converters 与
--      /api/v1/data-converters 端点（Casbin 无需变更）；
--   6. 前端四件套（converter 页 modeMap 增 PROTOBUF + proto 文本编辑域 + 四语言）
--      由下一阶段交付，本迁移不动 sys_ui_elements。
--
-- 编号沿革：本迁移内容曾随 TB-19 首轮实施落在 132.sql（同内容），后按批次三
--   统一编号分配改挂 135 号位；132.sql 转为号位占位空迁移。ADD COLUMN 带
--   IF NOT EXISTS，对已执行过旧 132.sql 的库幂等，两条升级路径结果一致。

-- ---- 1. data_converters 增加 proto_schema 列 ----
ALTER TABLE public.data_converters
    ADD COLUMN IF NOT EXISTS proto_schema TEXT;

COMMENT ON COLUMN public.data_converters.proto_schema IS
    'PROTOBUF 模式专属：.proto 源文件全文（protoreflect 动态解析，TB-19）；其他模式为 NULL';
