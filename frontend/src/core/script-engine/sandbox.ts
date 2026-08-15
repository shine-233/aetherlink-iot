/**
 * 文件用途：提供脚本安全沙箱，限制用户脚本可访问的全局对象和危险语法。
 * 核心逻辑：创建受控执行上下文，执行前做安全检查，并在超时或违规时阻断脚本。
 * 关键注意事项：任何放宽全局对象或语法规则的改动都可能扩大宿主环境暴露面。
 * 重构建议：可补充更系统的策略对象和审计日志，便于按场景配置安全级别。
 */

import type { BuiltinUtilities, IScriptSandbox, SandboxConfig } from '@/core/script-engine/types'
import { smartDeepClone } from '@/utils/deep-clone'

type SandboxTimerId = ReturnType<typeof setTimeout> | ReturnType<typeof setInterval>

/**
 * 脚本沙箱实现类
 */
export class ScriptSandbox implements IScriptSandbox {
  private config: SandboxConfig

  constructor(config: SandboxConfig) {
    this.config = config
  }

  /**
   * 创建沙箱环境
   */
  createSandbox(config: SandboxConfig): any {
    const sandbox: any = {}
    const blockedNetworkFetch = this.createBlockedNetworkFetch()
    const blockedNetworkConstructor = function (): never {
      throw new Error('SCRIPT_NETWORK_EXTERNAL_BLOCKED: script network access requires an audited adapter')
    }
    // with 作用域会向宿主全局回落；不可覆盖访问器既遮蔽网络入口，
    // 也防止 context/globals 通过 Object.assign 重新开启宿主网络能力。
    const blockedNetworkGlobals = {
      fetch: blockedNetworkFetch,
      WebSocket: blockedNetworkConstructor,
      XMLHttpRequest: blockedNetworkConstructor,
      EventSource: blockedNetworkConstructor,
      Worker: blockedNetworkConstructor,
      SharedWorker: blockedNetworkConstructor,
      BroadcastChannel: blockedNetworkConstructor
    }
    Object.entries(blockedNetworkGlobals).forEach(([name, blockedValue]) => {
      Object.defineProperty(sandbox, name, {
        configurable: true,
        enumerable: true,
        get: () => blockedValue,
        set: () => {}
      })
    })

    const blockedHostGlobals = {
      localStorage: 'SCRIPT_STORAGE_EXTERNAL_BLOCKED',
      sessionStorage: 'SCRIPT_STORAGE_EXTERNAL_BLOCKED',
      indexedDB: 'SCRIPT_STORAGE_EXTERNAL_BLOCKED',
      caches: 'SCRIPT_STORAGE_EXTERNAL_BLOCKED',
      window: 'SCRIPT_HOST_GLOBAL_BLOCKED',
      document: 'SCRIPT_HOST_GLOBAL_BLOCKED',
      location: 'SCRIPT_HOST_GLOBAL_BLOCKED',
      navigator: 'SCRIPT_HOST_GLOBAL_BLOCKED'
    }
    Object.entries(blockedHostGlobals).forEach(([name, errorCode]) => {
      Object.defineProperty(sandbox, name, {
        configurable: true,
        enumerable: true,
        get: () => {
          throw new Error(`${errorCode}: ${name} is not available in the script sandbox`)
        },
        set: () => {}
      })
    })

    // 添加允许的全局对象
    config.allowedGlobals.forEach((global) => {
      switch (global) {
        case 'Math':
          sandbox.Math = Math
          break
        case 'Date':
          sandbox.Date = Date
          break
        case 'JSON':
          sandbox.JSON = JSON
          break
        case 'Promise':
          sandbox.Promise = Promise
          break
        case 'setTimeout':
          sandbox.setTimeout = this.createSafeSetTimeout(sandbox)
          break
        case 'clearTimeout':
          sandbox.clearTimeout = this.createSafeClearTimeout(sandbox)
          break
        case 'setInterval':
          sandbox.setInterval = this.createSafeSetInterval(sandbox)
          break
        case 'clearInterval':
          sandbox.clearInterval = this.createSafeClearInterval(sandbox)
          break
        case 'fetch':
          // 网络能力需要可取消、可审计的专用适配器；当前只保留接口并明确阻断外联。
          sandbox.fetch = this.createBlockedNetworkFetch()
          break
        case 'console':
          // 提供安全的console实现
          sandbox.console = this.createSafeConsole()
          break
        default:
          // 其他安全的全局对象
          if (typeof window !== 'undefined' && global in window) {
            const value = (window as any)[global]
            if (typeof value !== 'function' || this.isSafeFunction(global)) {
              sandbox[global] = value
            }
          }
      }
    })

    // 添加安全的内置工具
    sandbox._utils = this.createBuiltinUtils()

    return sandbox
  }

  /**
   * 在沙箱中执行代码
   */
  async executeInSandbox(code: string, sandbox: any, timeout: number = 5000): Promise<any> {
    // 检查代码安全性
    const securityCheck = this.checkCodeSecurity(code)
    if (!securityCheck.safe) {
      throw new Error(`代码安全检查失败: ${securityCheck.issues.join(', ')}`)
    }

    return new Promise((resolve, reject) => {
      const timeoutId = setTimeout(() => {
        reject(new Error('脚本执行超时'))
      }, timeout)

      try {
        // 创建安全的执行函数
        const wrappedCode = this.wrapCodeForExecution(code)
        const executor = new Function(
          'sandbox',
          `
          with (sandbox) {
            return (async function() {
              ${wrappedCode}
            })();
          }
        `
        )

        // 执行代码
        const result = executor(sandbox)

        if (result instanceof Promise) {
          result
            .then((value) => {
              clearTimeout(timeoutId)
              resolve(value)
            })
            .catch((error) => {
              clearTimeout(timeoutId)
              reject(error)
            })
        } else {
          clearTimeout(timeoutId)
          resolve(result)
        }
      } catch (error) {
        clearTimeout(timeoutId)
        reject(error)
      }
    })
  }

  /**
   * 销毁沙箱
   */
  destroySandbox(sandbox: any): void {
    // 清理沙箱中的定时器
    if (sandbox._timers) {
      sandbox._timers.forEach((timer: any) => {
        if (timer.type === 'timeout') {
          clearTimeout(timer.id)
        } else if (timer.type === 'interval') {
          clearInterval(timer.id)
        }
      })
      sandbox._timers = []
    }

    // 清空沙箱对象
    Object.keys(sandbox).forEach((key) => {
      delete sandbox[key]
    })
  }

  /**
   * 检查代码安全性
   */
  checkCodeSecurity(code: string): { safe: boolean; issues: string[] } {
    const issues: string[] = []

    // This is a same-thread guardrail, not a full isolation boundary. Do not
    // treat user code as safely sandboxed until it runs in a worker/AST allowlist.
    // 检查是否包含危险关键字
    const dangerousPatterns = [
      /eval\s*\(/,
      /Function\s*\(/,
      /new\s+Function/,
      /import\s*\(/,
      /require\s*\(/,
      /process\./,
      /global\./,
      /globalThis\b/,
      /window\./,
      /document\./,
      /location\./,
      /navigator\./,
      /\bthis\s*(?:\.|\[)/,
      /__proto__/,
      /(?:\.|\[['"`])\s*constructor\s*(?:['"`\]]|\.)/,
      /(?:\.|\[['"`])\s*prototype\s*(?:['"`\]]|\.)/,
      /\bwhile\s*\(\s*(?:true|1)\s*\)/,
      /\bfor\s*\(\s*;\s*;\s*\)/
    ]

    dangerousPatterns.forEach((pattern, index) => {
      if (pattern.test(code)) {
        const patternNames = [
          'eval函数调用',
          'Function构造器',
          'new Function',
          '动态import',
          'require调用',
          'process对象访问',
          'global对象访问',
          'globalThis对象访问',
          'window对象访问',
          'document对象访问',
          'location对象访问',
          'navigator对象访问',
          'this宿主对象访问',
          '__proto__属性访问',
          'constructor属性访问',
          'prototype属性访问',
          '明显无限while循环',
          '明显无限for循环'
        ]
        issues.push(`检测到危险代码模式: ${patternNames[index]}`)
      }
    })

    // 检查自定义安全策略
    if (this.config.customSecurityPolicy) {
      if (!this.config.customSecurityPolicy(code)) {
        issues.push('自定义安全策略检查失败')
      }
    }

    return {
      safe: issues.length === 0,
      issues
    }
  }

  /**
   * 包装代码以便安全执行
   */
  private wrapCodeForExecution(code: string): string {
    // Same-thread execution cannot preempt synchronous loops; reject the known
    // dangerous shapes before this wrapper and keep the user function strict.
    return `
      "use strict";
      // eval 调用由执行前安全检查拦截；严格模式禁止声明名为 eval 的绑定。
      const Function = undefined;
      const require = undefined;
      const setTimeout = undefined;
      const clearTimeout = undefined;
      const setInterval = undefined;
      const clearInterval = undefined;
      const globalThis = undefined;
      // 注意：不能定义 import 变量，因为它是保留字

      // 用户代码
      ${code}
    `
  }

  /**
   * 创建安全的console实现
   */
  private createSafeConsole() {
    const logs: any[] = []

    return {
      log: (...args: any[]) => {
        logs.push({ level: 'log', args, timestamp: Date.now() })
      },
      warn: (...args: any[]) => {
        logs.push({ level: 'warn', args, timestamp: Date.now() })
      },
      error: (...args: any[]) => {
        logs.push({ level: 'error', args, timestamp: Date.now() })
      },
      info: (...args: any[]) => {
        logs.push({ level: 'info', args, timestamp: Date.now() })
      },
      debug: (...args: any[]) => {
        logs.push({ level: 'debug', args, timestamp: Date.now() })
      },
      _getLogs: () => logs
    }
  }

  private createBlockedNetworkFetch(): typeof fetch {
    return async () => {
      throw new Error('SCRIPT_NETWORK_EXTERNAL_BLOCKED: script network access requires an audited adapter')
    }
  }

  private ensureTimerStore(sandbox: any): Array<{ type: 'timeout' | 'interval'; id: SandboxTimerId }> {
    if (!sandbox._timers) {
      sandbox._timers = []
    }
    return sandbox._timers
  }

  private removeTimer(sandbox: any, id: SandboxTimerId): void {
    if (!sandbox._timers) {
      return
    }
    sandbox._timers = sandbox._timers.filter((timer: any) => timer.id !== id)
  }

  private createSafeSetTimeout(sandbox: any) {
    return (handler: TimerHandler, timeout?: number, ...args: any[]) => {
      const timerId = setTimeout(handler, timeout, ...args)
      this.ensureTimerStore(sandbox).push({ type: 'timeout', id: timerId })
      return timerId
    }
  }

  private createSafeClearTimeout(sandbox: any) {
    return (timerId: SandboxTimerId) => {
      clearTimeout(timerId)
      this.removeTimer(sandbox, timerId)
    }
  }

  private createSafeSetInterval(sandbox: any) {
    return (handler: TimerHandler, timeout?: number, ...args: any[]) => {
      const timerId = setInterval(handler, timeout, ...args)
      this.ensureTimerStore(sandbox).push({ type: 'interval', id: timerId })
      return timerId
    }
  }

  private createSafeClearInterval(sandbox: any) {
    return (timerId: SandboxTimerId) => {
      clearInterval(timerId)
      this.removeTimer(sandbox, timerId)
    }
  }

  /**
   * 创建内置工具函数
   */
  private createBuiltinUtils(): BuiltinUtilities {
    return {
      // 数据生成工具
      mockData: {
        randomNumber: (min = 0, max = 100) => Math.random() * (max - min) + min,
        randomString: (length = 10) =>
          Math.random()
            .toString(36)
            .substring(2, length + 2),
        randomBoolean: () => Math.random() > 0.5,
        randomDate: (start?: Date, end?: Date) => {
          const startTime = start ? start.getTime() : Date.now() - 365 * 24 * 60 * 60 * 1000
          const endTime = end ? end.getTime() : Date.now()
          return new Date(startTime + Math.random() * (endTime - startTime))
        },
        randomArray: <T>(items: T[], count?: number) => {
          const actualCount = count || Math.floor(Math.random() * items.length) + 1
          const result: T[] = []
          for (let i = 0; i < actualCount; i++) {
            result.push(items[Math.floor(Math.random() * items.length)])
          }
          return result
        },
        randomObject: (template: Record<string, any>) => {
          const result: any = {}
          Object.keys(template).forEach((key) => {
            const value = template[key]
            if (typeof value === 'function') {
              result[key] = value()
            } else {
              result[key] = value
            }
          })
          return result
        }
      },

      // 数据处理工具
      dataUtils: {
        deepClone: <T>(obj: T): T => smartDeepClone(obj),
        merge: (...objects: any[]) => Object.assign({}, ...objects),
        pick: <T extends Record<string, any>>(obj: T, keys: (keyof T)[]): Partial<T> => {
          const result: Partial<T> = {}
          keys.forEach((key) => {
            if (key in obj) {
              result[key] = obj[key]
            }
          })
          return result
        },
        omit: <T extends Record<string, any>>(obj: T, keys: (keyof T)[]): Partial<T> => {
          const result: Partial<T> = {}
          Object.keys(obj).forEach((key) => {
            if (!keys.includes(key as keyof T)) {
              result[key as keyof T] = obj[key as keyof T]
            }
          })
          return result
        },
        groupBy: <T>(array: T[], key: keyof T | ((item: T) => any)): Record<string, T[]> => {
          const result: Record<string, T[]> = {}
          array.forEach((item) => {
            const groupKey = typeof key === 'function' ? key(item) : item[key]
            const groupKeyStr = String(groupKey)
            if (!result[groupKeyStr]) {
              result[groupKeyStr] = []
            }
            result[groupKeyStr].push(item)
          })
          return result
        },
        sortBy: <T>(array: T[], key: keyof T | ((item: T) => any)): T[] => {
          return [...array].sort((a, b) => {
            const aVal = typeof key === 'function' ? key(a) : a[key]
            const bVal = typeof key === 'function' ? key(b) : b[key]
            if (aVal < bVal) return -1
            if (aVal > bVal) return 1
            return 0
          })
        }
      },

      // 时间工具
      timeUtils: {
        now: () => Date.now(),
        format: (date: Date | number, format: string = 'YYYY-MM-DD HH:mm:ss') => {
          const d = new Date(date)
          const year = d.getFullYear()
          const month = String(d.getMonth() + 1).padStart(2, '0')
          const day = String(d.getDate()).padStart(2, '0')
          const hours = String(d.getHours()).padStart(2, '0')
          const minutes = String(d.getMinutes()).padStart(2, '0')
          const seconds = String(d.getSeconds()).padStart(2, '0')

          return format
            .replace('YYYY', year.toString())
            .replace('MM', month)
            .replace('DD', day)
            .replace('HH', hours)
            .replace('mm', minutes)
            .replace('ss', seconds)
        },
        addDays: (date: Date, days: number) => {
          const result = new Date(date)
          result.setDate(result.getDate() + days)
          return result
        },
        diffDays: (date1: Date, date2: Date) => {
          const timeDiff = Math.abs(date2.getTime() - date1.getTime())
          return Math.ceil(timeDiff / (1000 * 3600 * 24))
        },
        startOfDay: (date: Date) => {
          const result = new Date(date)
          result.setHours(0, 0, 0, 0)
          return result
        },
        endOfDay: (date: Date) => {
          const result = new Date(date)
          result.setHours(23, 59, 59, 999)
          return result
        }
      },

      // 保留网络工具接口；未接入可取消、可审计的适配器前明确阻断外联。
      networkUtils: {
        httpGet: (url: string, options?: RequestInit) => this.rejectExternalNetwork(url, options),
        httpPost: (url: string, data: any, options?: RequestInit) => this.rejectExternalNetwork(url, data, options),
        httpPut: (url: string, data: any, options?: RequestInit) => this.rejectExternalNetwork(url, data, options),
        httpDelete: (url: string, options?: RequestInit) => this.rejectExternalNetwork(url, options)
      },

      // 日志接口使用本地收集器，不向外部服务发送数据。
      logger: this.createSafeConsole()
    }
  }

  private rejectExternalNetwork(..._args: any[]): Promise<never> {
    return Promise.reject(new Error('SCRIPT_NETWORK_EXTERNAL_BLOCKED: script network access requires an audited adapter'))
  }

  /**
   * 检查函数是否安全
   */
  private isSafeFunction(funcName: string): boolean {
    const safeFunctions = [
      'parseInt',
      'parseFloat',
      'isNaN',
      'isFinite',
      'encodeURIComponent',
      'decodeURIComponent',
      'encodeURI',
      'decodeURI'
    ]
    return safeFunctions.includes(funcName)
  }
}

/**
 * 默认沙箱配置
 */
export const defaultSandboxConfig: SandboxConfig = {
  enabled: true,
  allowedGlobals: ['Math', 'Date', 'JSON', 'Promise', 'console', 'parseInt', 'parseFloat', 'isNaN', 'isFinite'],
  blockedGlobals: [
    'eval',
    'Function',
    'window',
    'document',
    'location',
    'navigator',
    'process',
    'global',
    'require',
    'import'
  ],
  allowEval: false,
  allowFunction: false,
  allowPrototypePollution: false
}
