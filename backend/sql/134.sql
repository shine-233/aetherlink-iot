-- 134.sql — TB-47 白标（租户翻译覆盖 + 自定义 CSS）：后端面建表与 Casbin 登记
--
-- 背景：
--   1. 对标 ThingsBoard PE 白标（White Labeling）全套：平台已有租户级品牌资产
--      （logo 表：系统名称/站标/登录图/主题色/favicon，59.sql + 64.sql），但
--      "界面文案翻译覆盖"与"Advanced CSS"缺位——租户无法把内置界面词条改成本地
--      术语，也无法注入自有样式（ROADMAP §4.1 TB-47）；
--   2. tenant_translations：租户级 UI 翻译覆盖，与前端静态四语言目录
--      （zh-cn / en-us / es-es / fr-fr）并存，覆盖项以后端值为准（前端在 i18n
--      初始化后合并）。UNIQUE(tenant_id, lang, "key") 保证同租户同语言同键唯一，
--      重复写入走 UPSERT 覆盖；tenant_id 为空串保留给系统全局行（SYS_ADMIN 作用域，
--      语义同 logo 表的全局兜底行）；
--   3. tenant_custom_css：租户级自定义 CSS，tenant_id 主键单行（无 CSS 即无行），
--      css TEXT 上限 64KiB（服务层强制）。注入方式由前端用 textContent 写入
--      style 标签（禁止 innerHTML），后端另拒绝 '</style' 序列做纵深防御；
--   4. Casbin 路由登记（g2）与角色授权（p）：
--      - 管理面（PUT/GET/DELETE translations、GET/PUT custom-css）仅
--        SYS_ADMIN / TENANT_ADMIN，TENANT_USER fail-closed（口径同 136.sql）；
--      - GET whitelabel/overrides 是"登录后可读"的覆盖获取端点：任何登录用户
--        （含 TENANT_USER）都需要读取本租户的翻译覆盖与 CSS 才能渲染界面，
--        按登录态 claims 租户隔离返回，故对三个内置角色都放行。
--   5. 内置菜单改名/隐藏/按角色（tenant_dashboard_menus 扩展）不在本迁移范围
--      （TB-47 residual，后续批次单独评审）。
--
-- 回滚：
--   DROP TABLE IF EXISTS public.tenant_custom_css;
--   DROP TABLE IF EXISTS public.tenant_translations;
--   DELETE FROM casbin_rule WHERE v1 LIKE 'api/v1/whitelabel/%';

-- ---- 1. 租户翻译覆盖表 (tenant_translations) ----
CREATE TABLE IF NOT EXISTS public.tenant_translations (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id VARCHAR(36) NOT NULL,
    lang VARCHAR(35) NOT NULL,
    "key" VARCHAR(200) NOT NULL,
    value TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_tenant_translations_tenant_lang_key UNIQUE (tenant_id, lang, "key")
);

COMMENT ON TABLE public.tenant_translations IS '租户级 UI 翻译覆盖（TB-47；与静态四语言目录并存，覆盖项优先；UNIQUE(tenant_id,lang,key)）';
COMMENT ON COLUMN public.tenant_translations.lang IS '语言标签（小写连字符形态，与前端 locale 目录一致：zh-cn/en-us/es-es/fr-fr，服务层白名单校验）';
COMMENT ON COLUMN public.tenant_translations."key" IS 'i18n 词条键（如 page.customer.title，服务层限制字母/数字/./-/_ 且 ≤200 字符）';
COMMENT ON COLUMN public.tenant_translations.value IS '覆盖后的译文（≤4000 字符，允许换行）';
COMMENT ON COLUMN public.tenant_translations.tenant_id IS '租户 ID；空串为系统全局行（SYS_ADMIN 作用域，语义同 logo 全局兜底行）';

-- ---- 2. 租户自定义 CSS 表 (tenant_custom_css，tenant_id 主键单行) ----
CREATE TABLE IF NOT EXISTS public.tenant_custom_css (
    tenant_id VARCHAR(36) PRIMARY KEY,
    css TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

COMMENT ON TABLE public.tenant_custom_css IS '租户级自定义 CSS（TB-47；tenant_id 主键单行，无行=未配置）';
COMMENT ON COLUMN public.tenant_custom_css.css IS '自定义样式文本（≤64KiB；前端必须 textContent 注入 style 标签，后端拒绝 </style 序列做纵深防御）';

-- ---- 3. Casbin 路由登记 (g2) ----
INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/whitelabel/translations'),
  ('api/v1/whitelabel/custom-css'),
  ('api/v1/whitelabel/overrides')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

-- ---- 4. 授权角色 (p) ----
-- 管理面：仅 SYS_ADMIN / TENANT_ADMIN；GET overrides：登录后可读，三内置角色放行。
INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/whitelabel/translations'),
  ('SYS_ADMIN',    'api/v1/whitelabel/custom-css'),
  ('SYS_ADMIN',    'api/v1/whitelabel/overrides'),
  ('TENANT_ADMIN', 'api/v1/whitelabel/translations'),
  ('TENANT_ADMIN', 'api/v1/whitelabel/custom-css'),
  ('TENANT_ADMIN', 'api/v1/whitelabel/overrides'),
  ('TENANT_USER',  'api/v1/whitelabel/overrides')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);
