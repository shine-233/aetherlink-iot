/**
 * 文件用途：声明 router 相关的前端类型边界。
 * 核心逻辑：通过 namespace、interface 或模块声明约束跨文件调用形态。
 * 关键注意事项：声明文件可能影响全局类型推断或生成类型边界，修改需检查全量 TS 引用。
 * 重构建议：可逐步把宽泛全局声明收敛为模块化导出，并标注生成来源。
 */
import 'vue-router'

declare module 'vue-router' {
  interface RouteMeta {
    /**
     * Title of the route
     *
     * It can be used in document title
     */
    title: string
    /**
     * I18n key of the route
     *
     * It's used in i18n, if it is set, the title will be ignored
     */
    i18nKey?: App.I18n.I18nKey
    /**
     * Roles of the route
     *
     * Route can be accessed if the current user has at least one of the roles
     */
    roles?: string[]
    /** Whether to cache the route */
    keepAlive?: boolean
    /**
     * Is constant route
     *
     * Does not need to login, and the route is defined in the front-end
     */
    constant?: boolean
    /**
     * Iconify icon
     *
     * It can be used in the menu or breadcrumb
     */
    singleLayout?: string
    icon?: string
    /**
     * Local icon
     *
     * In "src/assets/svg-icon", if it is set, the icon will be ignored
     */
    localIcon?: string
    /** Router order */
    order?: number
    /** The outer link of the route */
    href?: string
    /** Whether to hide the route in the menu */
    hideInMenu?: boolean
    /**
     * The menu key will be activated when entering the route
     *
     * The route is not in the menu
     *
     * @example
     *   the route is "user_detail", if it is set to "user_list", the menu "user_list" will be activated
     */
    activeMenu?: import('@elegant-router/types').RouteKey
    /** By default, the same route path will use one tab, if set to true, it will use multiple tabs */
    multiTab?: boolean
    /** If set, the route will be fixed in tabs, and the value is the order of fixed tabs */
    fixedIndexInTab?: number
    remark?: string
  }
}
