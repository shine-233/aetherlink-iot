/**
 * 文件用途：维护前端表单和业务校验正则。
 * 核心逻辑：导出手机号、邮箱、账号等输入校验表达式。
 * 关键注意事项：正则会直接影响用户输入边界，变更需补充正反例测试。
 * 重构建议：可把复杂正则包装成具名校验函数，提高错误提示和测试可读性。
 */
/**
 * 文件：表单校验正则常量。
 * 作用：维护用户名、手机号、密码、邮箱、验证码和 URL 的通用校验规则。
 * 依赖：无运行时依赖，供表单、工具函数和测试直接复用。
 * 维护：修改正则前补充输入样例，避免放宽或收紧规则时影响登录注册流程。
 */

export const REG_DEFAULT = /^.*$/
export const REG_USER_NAME = /^[\u4E00-\u9FA5a-zA-Z0-9_-]{4,16}$/

/** Phone reg */
export const REG_PHONE = /^1((3[0-9])|(4[01456789])|(5[012356789])|(6[2567])|(7[0-8])|(8[0-9])|(9[012356789]))[0-9]{8}$/

/** Phone with country code reg */
export const REG_PHONE_WITH_COUNTRY_CODE = /^\d{7,15}$/

/** Password reg: 8-20 chars, including uppercase, lowercase, number, and special char. */
export const REG_PWD =
  /^(?=.{8,20}$)(?=.*[a-z])(?=.*[A-Z])(?=.*\d)(?=.*[!@#$%^&*()_+\-=[\]{};\\':"|,./<>?])[A-Za-z\d!@#$%^&*()_+\-=[\]{};\\':"|,./<>?]+$/

/** Email reg */
export const REG_EMAIL = /^\w+([-+.]\w+)*@\w+([-.]\w+)*\.\w+([-.]\w+)*$/

/** Six digit code reg */
export const REG_CODE_SIX = /^\d{6}$/

/** Four digit code reg */
export const REG_CODE_FOUR = /^\d{4}$/

/** Url reg */
export const REG_URL =
  /(((^https?:(?:\/\/)?)(?:[-;:&=+$,\w]+@)?[A-Za-z0-9.-]+(?::\d+)?|(?:www.|[-;:&=+$,\w]+@)[A-Za-z0-9.-]+)((?:\/[+~%/.\w-_]*)?\??[-+=&;%@.\w_]*#?\w*)?)$/
