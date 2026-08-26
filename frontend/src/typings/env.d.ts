/**
 * 文件用途：声明 env 相关的前端类型边界。
 * 核心逻辑：通过 namespace、interface 或模块声明约束跨文件调用形态。
 * 关键注意事项：声明文件可能影响全局类型推断或生成类型边界，修改需检查全量 TS 引用。
 * 重构建议：可逐步把宽泛全局声明收敛为模块化导出，并标注生成来源。
 */
/**
 * Namespace Env
 *
 * It is used to declare the type of the import.meta object
 */
declare namespace Env {
  /** The router history mode */
  type RouterHistoryMode = 'hash' | 'history' | 'memory'

  /** Interface for import. Meta */
  interface ImportMeta extends ImportMetaEnv {
    /** The base url of the application */
    readonly VITE_BASE_URL: string
    /** The title of the application */
    readonly VITE_APP_TITLE: string
    /** The description of the application */
    readonly VITE_APP_DESC: string
    /** The router history mode */
    readonly VITE_ROUTER_HISTORY_MODE?: RouterHistoryMode
    /** The prefix of the iconify icon */
    readonly VITE_ICON_PREFIX: 'icon'
    /**
     * The prefix of the local icon
     *
     * This prefix is start with the icon prefix
     */
    readonly VITE_ICON_LOCAL_PREFIX: 'local-icon'
    /**
     * Whether to enable the http proxy
     *
     * Only valid in the development environment
     */
    readonly VITE_HTTP_PROXY?: CommonType.YesOrNo
    /** The back service env */
    readonly VITE_SERVICE_ENV?: App.Service.EnvType
    /** Optional development API origin/path override for local verification */
    readonly VITE_DEV_API_URL?: string
    /**
     * The auth route mode
     *
     * - Static: the auth routes is generated in front-end
     * - Dynamic: the auth routes is generated in back-end
     */
    readonly VITE_AUTH_ROUTE_MODE: 'static' | 'dynamic'
    /**
     * The home route key
     *
     * It only has effect when the auth route mode is static, if the route mode is dynamic, the home route key is
     * defined in the back-end
     */
    readonly VITE_ROUTE_HOME: import('@elegant-router/types').LastLevelRouteKey
    /**
     * Default menu icon if menu icon is not set
     *
     * Iconify icon name
     */
    readonly VITE_MENU_ICON: string
    /** Whether to build with sourcemap */
    readonly VITE_SOURCE_MAP?: CommonType.YesOrNo
    /**
     * Iconify api provider url
     *
     * If the project is deployed in intranet, you can set the api provider url to the local iconify server
     *
     * @link https://docs.iconify.design/api/providers.html
     */
    readonly VITE_ICONIFY_URL?: string
    /** Whether the footer may query GitHub for the latest release tag */
    readonly VITE_ENABLE_REMOTE_VERSION_CHECK?: CommonType.YesOrNo
    /** Optional Baidu Map browser SDK key; leave empty to disable the external map */
    readonly VITE_BAIDU_MAP_KEY?: string
    /** Optional AMap browser SDK key; leave empty to use the local fallback view */
    readonly VITE_AMAP_KEY?: string
    /** Optional AMap security JS code; injected before SDK load and never hardcoded into HTML */
    readonly VITE_AMAP_SECURITY_CODE?: string
    /** Optional Tencent Map browser SDK key; leave empty to disable the external map */
    readonly VITE_TENCENT_MAP_KEY?: string
    /** Auto login username for development environment */
    readonly VITE_AUTO_LOGIN_USERNAME?: string
    /** Auto login password for development environment */
    readonly VITE_AUTO_LOGIN_PASSWORD?: string
    /** Enable legacy ThingsVis routes and the standalone compatibility preview */
    readonly VITE_ENABLE_THINGSVIS_COMPAT?: CommonType.YesOrNo
    /** ThingsVis Studio HTML entry URL; used only by the optional compatibility layer */
    readonly VITE_THINGSVIS_STUDIO_URL?: string
    /** ThingsVis backend proxy target URL for Vite development proxy */
    readonly VITE_THINGSVIS_API_URL?: string
    /** Optional tenant/deployment default EZUIKit cloud recording space id */
    readonly VITE_EZUIKIT_DEFAULT_SPACE_ID?: string
    /** Optional tenant/deployment default EZUIKit playback business type */
    readonly VITE_EZUIKIT_DEFAULT_BUS_TYPE?: string
    /** Enable explicit EZUIKit playback fallback defaults for deployments that still rely on tenant-level presets */
    readonly VITE_ENABLE_EZUIKIT_COMPAT_DEFAULTS?: CommonType.YesOrNo
    readonly globEager: <T = any>(globPattern: string) => Record<string, T>
  }
}
