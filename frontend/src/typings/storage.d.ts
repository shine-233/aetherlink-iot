/**
 * 文件用途：声明 storage 相关的前端类型边界。
 * 核心逻辑：通过 namespace、interface 或模块声明约束跨文件调用形态。
 * 关键注意事项：声明文件可能影响全局类型推断或生成类型边界，修改需检查全量 TS 引用。
 * 重构建议：可逐步把宽泛全局声明收敛为模块化导出，并标注生成来源。
 */
/** The storage namespace */
declare namespace StorageType {
  interface Session {
    /** The theme color */
    themeColor: string
    // /**
    //  * the theme settings
    //  */
    // themeSettings: App.Theme.ThemeSetting;
  }

  interface Local {
    /** The i18n language */
    lang: App.I18n.LangType
    /** The token */
    token: string
    /** The expires in time */
    token_expires_in: string
    /** The user info */
    userInfo: Api.Auth.UserInfo
    /** The theme color */
    themeColor: string
    /** The theme settings */
    themeSettings: App.Theme.ThemeSetting
    /**
     * The override theme flags
     *
     * The value is the build time of the project
     */
    overrideThemeFlag: string
    /** The global tabs */
    globalTabs: App.Global.Tab[]
    /** loading logo */
    logoLoading: string
    /** system name cached for loading screen */
    systemName: string
  }
}
