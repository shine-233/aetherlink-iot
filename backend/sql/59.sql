-- 59.sql: 白标定制（ROADMAP C5）——logo 品牌表补充主题色与独立 favicon。
-- 背景：现有 logo 表已覆盖系统名称/站标/加载页/首页背景；C5 增量是
--       theme_color（全局主题色，前端注入 CSS 变量）与 favicon（页签图标 URL），
--       与 logo_cache（顶部站标）解耦，供租户级白标独立配置。
-- 边界：幂等（ADD COLUMN IF NOT EXISTS）；存量行 theme_color 默认空 = 前端回退默认主题色。
-- 回滚：ALTER TABLE logo DROP COLUMN IF EXISTS theme_color; DROP COLUMN IF EXISTS favicon;

ALTER TABLE logo
    ADD COLUMN IF NOT EXISTS theme_color VARCHAR(32) DEFAULT '',
    ADD COLUMN IF NOT EXISTS favicon VARCHAR(255) DEFAULT '';

COMMENT ON COLUMN logo.theme_color IS '租户级主题色（#RRGGBB），空值回退前端默认主题';
COMMENT ON COLUMN logo.favicon IS '页签 favicon URL，与站标 logo_cache 解耦';