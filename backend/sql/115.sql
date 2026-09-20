-- 115.sql — P1.3 SCADA 控制审计：confirmation_token 列加宽
--
-- 背景（2026-09-19 活栈真实下发联调暴露）：
--   88.sql 把 scada_control_audits.confirmation_token 建成 varchar(64)，
--   但 ConfirmationIssuer 签发的令牌格式是 "<过期秒>.<64位HMAC-SHA256 hex>"
--   （约 75 字符）。审计先于执行落库是 P1.3 的刻意设计（写不进去就拒绝执行），
--   于是**每一条带确认令牌的控制命令都在审计写入处 22001 失败**——
--   确认后的控制执行在运行期 100% 不可用，而"无令牌"的拒绝路径反而正常
--   （空令牌列装得下）。API 单测从未同时覆盖"带令牌执行+审计落库"，
--   所以该缺陷一直未暴露。63 号活栈契约测试（真实下发联调）首次把它抓出来。
--
-- 修复：加宽到 varchar(255)。令牌是 5 分钟有效的 HMAC 工件，
-- 全文留审计便于事后核对"哪一次确认对应哪一次执行"。

ALTER TABLE public.scada_control_audits
    ALTER COLUMN confirmation_token TYPE varchar(255);

COMMENT ON COLUMN public.scada_control_audits.confirmation_token IS
    '签发的二次确认令牌全文（<过期秒>.<HMAC-SHA256 hex>，约 75 字符）；denied 空串';
