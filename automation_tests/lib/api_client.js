/**
 * 文件用途：用于支撑 automation_tests 的API 自动化共享客户端。
 * 核心逻辑：封装自动化运行所需的配置、客户端、覆盖率、报告、种子数据或断言能力，供 API 与 E2E 套件复用。
 * 关键注意事项：共享库变更会影响多类自动化套件，必须保持错误信息和前置条件可诊断。
 * 重构建议：继续按职责拆分深模块，避免把运行配置、业务断言和报告生成耦合在同一入口。
 */

const axios = require('axios');
const FormData = require('form-data');
const endpointCoverage = require('./endpoint_coverage');
const networkRuntime = require('./network_runtime');

function resolveRuntimeURL(name, fallback) {
  return networkRuntime.validateTrustedURL(
    process.env[name] || fallback,
    name
  );
}

function resolveRuntimeTimeout() {
  const timeout = Number(process.env.API_TIMEOUT_MS || '15000');
  return Number.isFinite(timeout) && timeout > 0 ? timeout : 15000;
}

// Keep the request client independent of runtime_config.js. That module reads
// config.json and is useful for file-backed test fixtures, but network clients
// must derive destinations and credentials only from validated environment values.
const config = {
  baseURL: resolveRuntimeURL('API_BASE_URL', 'http://127.0.0.1:9999/api/v1'),
  healthURL: resolveRuntimeURL('HEALTH_URL', 'http://127.0.0.1:9999/health'),
  frontendURL: resolveRuntimeURL('FRONTEND_URL', 'http://127.0.0.1:5002'),
  timeout: resolveRuntimeTimeout(),
  e2e: networkRuntime.e2e,
  accounts: networkRuntime.accounts,
  accountEnvOverrides: networkRuntime.accountEnvOverrides,
  releaseRequiredAccounts: networkRuntime.releaseRequiredAccounts,
  validateTrustedURL: networkRuntime.validateTrustedURL
};

function assertRelativeAPIPath(url) {
  if (typeof url !== 'string' || !url.startsWith('/') || url.startsWith('//')) {
    throw new Error('Automation API calls must use a relative path beginning with /');
  }
  return url;
}

// 后端防护性限流的业务码（backend/pkg/errcode/code.go CodeRateLimit）。
// 命中时测试侧仅做一次退避后重试以容忍防护性限流，不构成绕过。
const RATE_LIMIT_CODE = 201003;
const RATE_LIMIT_BACKOFF_MS = 1200;
const TOKEN_EXPIRED_CODE = 40102;

// 每租户 HTTP 429 限流（middleware.TenantRateLimit，含 Retry-After 头）。
// 与 201003 同口径：测试侧退避重试以容忍防护性限流，不构成绕过；
// 最多重试 4 次（覆盖批处理用例的突发窗口），尊重 Retry-After（上限 5s），持续超限时如实失败。
const HTTP_RATE_LIMIT_STATUS = 429;
const HTTP_RATE_LIMIT_MAX_RETRIES = 4;
const HTTP_RATE_LIMIT_BACKOFF_MS = 1500;
const HTTP_RATE_LIMIT_BACKOFF_CAP_MS = 5000;

// MQTT debug 会话创建（backend OpenCooldown 默认 2s，按 device+user scope 冷却）。
// 套件连跑间隔过短时会命中重开冷却返回 201003，属防护性限流而非业务失败。
const MQTT_DEBUG_SESSION_CREATE_PATH_RE = /^\/device\/[^/]+\/mqtt-debug\/session$/;

function wait(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

// 计算 429 退避时长：优先尊重 Retry-After（秒），上限受 HTTP_RATE_LIMIT_BACKOFF_CAP_MS 约束。
function httpRateLimitBackoffMs(resp) {
  const retryAfter = resp && resp.headers ? Number(resp.headers['retry-after']) : NaN;
  if (Number.isFinite(retryAfter) && retryAfter >= 0) {
    return Math.min(retryAfter * 1000, HTTP_RATE_LIMIT_BACKOFF_CAP_MS);
  }
  return HTTP_RATE_LIMIT_BACKOFF_MS;
}

// 判定响应是否为每租户 HTTP 429 限流（middleware.TenantRateLimit 返回 {code:429,...}）。
function isHttpRateLimited(resp) {
  return Boolean(
    resp &&
    resp.status === HTTP_RATE_LIMIT_STATUS &&
    resp.data &&
    Number(resp.data.code) === HTTP_RATE_LIMIT_STATUS
  );
}

// 区分「业务码非 200 的主动抛出」与 axios 网络错误，避免被 catch 二次包装或误判重试。
class LoginRetrySignal extends Error {}

class ApiClient {
  constructor() {
    const trustedBaseURL = config.baseURL;
    this.baseURL = trustedBaseURL;
    this.healthURL = config.healthURL;
    this.timeout = config.timeout;
    this.tokens = {};
    this.client = axios.create({
      baseURL: trustedBaseURL,
      timeout: this.timeout,
      // 测试断言统一期望英文验证消息；后端按 Accept-Language 决定中英文（默认中文）。
      // 不设此头会导致 device-config / device-alarm-share 等用例收到中文消息而失败。
      headers: { 'Content-Type': 'application/json', 'Accept-Language': 'en' },
      paramsSerializer: { indexes: null }
    });
  }

  /**
   * 登录获取 Token
   * 登录失败时抛出 Error（与 get/post 等返回错误对象的设计不同），
   * 以便 before 钩子或调用方通过 try/catch 明确感知登录失败
   * 对业务码 201003（CodeRateLimit）做一次退避重试：这是测试侧对后端
   * 防护性限流的容忍而非绕过；仅作用于登录接口，最多重试 1 次。
   * @param {string} accountKey - network runtime account key
   * @returns {Promise<string>} 登录成功后的 token
   * @throws {Error} 账号未配置、HTTP 错误、业务码非 200 时抛出
   */
  async login(accountKey = 'tenant_admin', options = {}) {
    const account = config.accounts[accountKey];
    if (!account) {
      throw new Error(`测试账号 ${accountKey} 未通过运行环境配置`);
    }
    try {
      const resp = await this.client.post('/login', {
        email: account.email,
        password: account.password
      });
      if (resp.data && resp.data.code === 200 && resp.data.data && resp.data.data.token) {
        this.tokens[accountKey] = resp.data.data.token;
        return resp.data.data.token;
      }
      // 命中 201003（CodeRateLimit）时退避重试 1 次：测试侧对防护性限流的容忍而非绕过，
      // 仅作用于登录接口，最多重试 1 次。
      if (resp.data && resp.data.code === RATE_LIMIT_CODE && !options.rateLimitRetried) {
        await wait(RATE_LIMIT_BACKOFF_MS);
        return this.login(accountKey, { ...options, rateLimitRetried: true });
      }
      throw new LoginRetrySignal(`登录失败: ${JSON.stringify(resp.data)}`);
    } catch (err) {
      if (err instanceof LoginRetrySignal) {
        throw err;
      }
      if (err.response) {
        if (
          err.response.data &&
          err.response.data.code === RATE_LIMIT_CODE &&
          !options.rateLimitRetried
        ) {
          await wait(RATE_LIMIT_BACKOFF_MS);
          return this.login(accountKey, { ...options, rateLimitRetried: true });
        }
        throw new Error(`登录请求失败 [${err.response.status}]: ${JSON.stringify(err.response.data)}`);
      }
      throw new Error(`登录请求异常: ${err.message}`);
    }
  }

  /**
   * 获取指定账号的 Token（如未登录则自动登录）
   * 注意：Token 缓存在内存中，不会自动检测过期；如需强制重新登录请先 clearToken
   * @param {string} accountKey - network runtime account key
   * @returns {Promise<string>} token
   */
  async getToken(accountKey = 'tenant_admin') {
    if (!this.tokens[accountKey]) {
      await this.login(accountKey);
    }
    return this.tokens[accountKey];
  }

  /**
   * 清除指定账号的 Token（下次请求将触发重新登录）
   * @param {string} accountKey - network runtime account key
   */
  clearToken(accountKey) {
    delete this.tokens[accountKey];
  }

  /**
   * 清除所有账号的 Token
   */
  clearAllTokens() {
    this.tokens = {};
  }

  /**
   * 构造带认证的请求头（自动获取/复用 Token）
   * @param {string} accountKey - network runtime account key
   * @returns {Promise<object>} 包含 x-token 字段的 headers 对象
   */
  async authHeaders(accountKey = 'tenant_admin') {
    const token = await this.getToken(accountKey);
    return { 'x-token': token };
  }

  recordEndpointResponse(method, url, error = null) {
    if (!error || error.response) {
      endpointCoverage.hit(method, this.baseURL + url);
    }
  }

  isExpiredTokenError(err) {
    const responseData = err && err.response && err.response.data;
    const responseCodes = [
      responseData && responseData.code,
      responseData && responseData.data && responseData.data.code,
      responseData && responseData.error && responseData.error.code
    ];
    return Boolean(
      err &&
      err.response &&
      err.response.status === 401 &&
      responseCodes.some(code => Number(code) === TOKEN_EXPIRED_CODE)
    );
  }

  async retryExpiredToken(err, accountKey, options, retry) {
    if (!this.isExpiredTokenError(err) || options.tokenExpiryRetried) {
      return this.handleError(err);
    }

    this.clearToken(accountKey);
    await this.login(accountKey);
    return retry({ ...options, tokenExpiryRetried: true });
  }

  /**
   * 统一的可恢复错误处理：token 过期重登 → 每租户 429 限流退避重试 → 标准化错误对象。
   * 429 容忍与 201003 同口径：仅退避重试（最多 HTTP_RATE_LIMIT_MAX_RETRIES 次），
   * 尊重 Retry-After；持续超限时如实上抛错误，不构成对限流策略的绕过。
   */
  async handleRecoverableError(err, accountKey, options, retry) {
    if (this.isExpiredTokenError(err) && !options.tokenExpiryRetried) {
      return this.retryExpiredToken(err, accountKey, options, retry);
    }
    if (
      err &&
      err.response &&
      err.response.status === HTTP_RATE_LIMIT_STATUS &&
      (options.httpRateLimitRetries || 0) < HTTP_RATE_LIMIT_MAX_RETRIES
    ) {
      await wait(httpRateLimitBackoffMs(err.response));
      return retry({
        ...options,
        httpRateLimitRetries: (options.httpRateLimitRetries || 0) + 1
      });
    }
    return this.handleError(err);
  }

  /**
   * 通用 GET 请求
   * 请求失败时返回标准化错误对象（含 _requestError: true），不抛出异常
   * @param {string} url - 接口路径（相对 baseURL）
   * @param {object} params - 查询参数
   * @param {string} accountKey - 使用的账号
   * @returns {Promise<object>} 后端响应体或错误对象
   */
  async get(url, params = {}, accountKey = 'tenant_admin', options = {}) {
    assertRelativeAPIPath(url);
    const headers = await this.authHeaders(accountKey);
    try {
      const resp = await this.client.get(url, { params, headers });
      this.recordEndpointResponse('GET', url);
      return resp.data;
    } catch (err) {
      this.recordEndpointResponse('GET', url, err);
      return this.handleRecoverableError(
        err,
        accountKey,
        options,
        retryOptions => this.get(url, params, accountKey, retryOptions)
      );
    }
  }

  /**
   * 通用 POST 请求
   * 请求失败时返回标准化错误对象（含 _requestError: true），不抛出异常
   * @param {string} url - 接口路径（相对 baseURL）
   * @param {object} data - 请求体
   * @param {string} accountKey - 使用的账号
   * @returns {Promise<object>} 后端响应体或错误对象
   */
  async post(url, data = {}, accountKey = 'tenant_admin', options = {}) {
    assertRelativeAPIPath(url);
    const headers = await this.authHeaders(accountKey);
    try {
      const resp = await this.client.post(url, data, { headers });
      this.recordEndpointResponse('POST', url);
      // MQTT debug 会话创建命中 201003（会话重开冷却）时退避重试 1 次：
      // 测试侧对防护性限流的容忍而非绕过，与 login 的限流容忍模式一致，仅限该接口。
      if (
        MQTT_DEBUG_SESSION_CREATE_PATH_RE.test(url) &&
        resp.data &&
        resp.data.code === RATE_LIMIT_CODE &&
        !options.rateLimitRetried
      ) {
        await wait(RATE_LIMIT_BACKOFF_MS);
        return this.post(url, data, accountKey, { ...options, rateLimitRetried: true });
      }
      return resp.data;
    } catch (err) {
      this.recordEndpointResponse('POST', url, err);
      return this.handleRecoverableError(
        err,
        accountKey,
        options,
        retryOptions => this.post(url, data, accountKey, retryOptions)
      );
    }
  }

  async upload(url, fileContent, fields = {}, accountKey = 'tenant_admin', options = {}) {
    assertRelativeAPIPath(url);
    if (!Buffer.isBuffer(fileContent)) {
      throw new TypeError('Automation uploads must be generated fixture buffers');
    }
    const auth = await this.authHeaders(accountKey);
    const form = new FormData();
    form.append('file', fileContent, {
      filename: 'aetherlink-automation-fixture.bin',
      contentType: 'application/octet-stream'
    });
    for (const [key, value] of Object.entries(fields)) {
      form.append(key, String(value));
    }
    try {
      const resp = await this.client.post(url, form, {
        headers: { ...auth, ...form.getHeaders() }
      });
      this.recordEndpointResponse('POST', url);
      return resp.data;
    } catch (err) {
      this.recordEndpointResponse('POST', url, err);
      return this.handleRecoverableError(
        err,
        accountKey,
        options,
        retryOptions => this.upload(url, fileContent, fields, accountKey, retryOptions)
      );
    }
  }

  /**
   * 通用 PUT 请求
   * 请求失败时返回标准化错误对象（含 _requestError: true），不抛出异常
   * @param {string} url - 接口路径（相对 baseURL）
   * @param {object} data - 请求体
   * @param {string} accountKey - 使用的账号
   * @returns {Promise<object>} 后端响应体或错误对象
   */
  async put(url, data = {}, accountKey = 'tenant_admin', options = {}) {
    assertRelativeAPIPath(url);
    const headers = await this.authHeaders(accountKey);
    try {
      const resp = await this.client.put(url, data, { headers });
      this.recordEndpointResponse('PUT', url);
      return resp.data;
    } catch (err) {
      this.recordEndpointResponse('PUT', url, err);
      return this.handleRecoverableError(
        err,
        accountKey,
        options,
        retryOptions => this.put(url, data, accountKey, retryOptions)
      );
    }
  }

  async patch(url, data = {}, accountKey = 'tenant_admin', options = {}) {
    assertRelativeAPIPath(url);
    const headers = await this.authHeaders(accountKey);
    try {
      const resp = await this.client.patch(url, data, { headers });
      this.recordEndpointResponse('PATCH', url);
      return resp.data;
    } catch (err) {
      this.recordEndpointResponse('PATCH', url, err);
      return this.handleRecoverableError(
        err,
        accountKey,
        options,
        retryOptions => this.patch(url, data, accountKey, retryOptions)
      );
    }
  }

  /**
   * 通用 DELETE 请求
   * 请求失败时返回标准化错误对象（含 _requestError: true），不抛出异常
   * @param {string} url - 接口路径（相对 baseURL）
   * @param {object} data - 请求体（部分 DELETE 接口需要）
   * @param {string} accountKey - 使用的账号
   * @returns {Promise<object>} 后端响应体或错误对象
   */
  async delete(url, data = {}, accountKey = 'tenant_admin', options = {}) {
    assertRelativeAPIPath(url);
    const headers = await this.authHeaders(accountKey);
    try {
      const resp = await this.client.delete(url, { headers, data });
      this.recordEndpointResponse('DELETE', url);
      return resp.data;
    } catch (err) {
      this.recordEndpointResponse('DELETE', url, err);
      return this.handleRecoverableError(
        err,
        accountKey,
        options,
        retryOptions => this.delete(url, data, accountKey, retryOptions)
      );
    }
  }

  /**
   * 无认证 GET 请求（用于公开接口，如 /systime、/logo）
   * @param {string} url - 接口路径（相对 baseURL）
   * @param {object} params - 查询参数
   * @returns {Promise<object>} 后端响应体或错误对象
   */
  async getNoAuth(url, params = {}) {
    assertRelativeAPIPath(url);
    try {
      const resp = await this.client.get(url, { params });
      this.recordEndpointResponse('GET', url);
      return resp.data;
    } catch (err) {
      this.recordEndpointResponse('GET', url, err);
      return this.handleError(err);
    }
  }

  async getRootNoAuth(url, params = {}) {
    assertRelativeAPIPath(url);
    const rootURL = this.baseURL.replace(/\/api\/v1\/?$/, '');
    const targetURL = new URL(url, rootURL + '/').toString();
    try {
      const resp = await axios.get(targetURL, { params, timeout: this.timeout });
      endpointCoverage.hit('GET', targetURL);
      return { httpStatus: resp.status, data: resp.data };
    } catch (err) {
      if (err.response) {
        endpointCoverage.hit('GET', targetURL);
        return { httpStatus: err.response.status, data: err.response.data };
      }
      return this.handleError(err);
    }
  }

  /**
   * 无认证 POST 请求（用于登录等公开接口）
   * @param {string} url - 接口路径（相对 baseURL）
   * @param {object} data - 请求体
   * @returns {Promise<object>} 后端响应体或错误对象
   */
  async postNoAuth(url, data = {}) {
    assertRelativeAPIPath(url);
    try {
      const resp = await this.client.post(url, data);
      this.recordEndpointResponse('POST', url);
      return resp.data;
    } catch (err) {
      this.recordEndpointResponse('POST', url, err);
      return this.handleError(err);
    }
  }

  /**
   * 无认证 PUT 请求，用于验证受限接口在缺少 token 时的拒绝行为
   * @param {string} url - 接口路径（相对 baseURL）
   * @param {object} data - 请求体
   * @returns {Promise<object>} 后端响应体或错误对象
   */
  async putNoAuth(url, data = {}) {
    assertRelativeAPIPath(url);
    try {
      const resp = await this.client.put(url, data);
      this.recordEndpointResponse('PUT', url);
      return resp.data;
    } catch (err) {
      this.recordEndpointResponse('PUT', url, err);
      return this.handleError(err);
    }
  }

  /**
   * 无认证 DELETE 请求，用于验证受限接口在缺少 token 时的拒绝行为
   * @param {string} url - 接口路径（相对 baseURL）
   * @param {object} data - 请求体
   * @returns {Promise<object>} 后端响应体或错误对象
   */
  async deleteNoAuth(url, data = {}) {
    assertRelativeAPIPath(url);
    try {
      const resp = await this.client.delete(url, { data });
      this.recordEndpointResponse('DELETE', url);
      return resp.data;
    } catch (err) {
      this.recordEndpointResponse('DELETE', url, err);
      return this.handleError(err);
    }
  }

  /**
   * 后端健康检查（直接请求 healthURL，不走 baseURL）
   * @returns {Promise<boolean>} true 表示后端可用
   */
  async healthCheck() {
    try {
      const resp = await axios.get(this.healthURL, { timeout: 5000 });
      return resp.status === 200;
    } catch (err) {
      return false;
    }
  }

  /**
   * 统一错误处理：返回标准化的错误响应对象
   * 区分 HTTP 错误（有 response）与网络/超时错误（无 response）
   * @param {Error} err - axios 抛出的错误
   * @returns {object} 标准化错误对象，含 code/message/data/_requestError
   */
  handleError(err) {
    if (err.response) {
      return {
        code: err.response.status,
        message: err.response.data && err.response.data.message ? err.response.data.message : err.message,
        data: err.response.data,
        _requestError: true
      };
    }
    return {
      code: -1,
      message: err.message,
      data: null,
      _requestError: true
    };
  }

  /**
   * 获取网络运行时配置对象（同步返回）
   * @returns {object} validated environment-backed network configuration
   */
  getConfig() {
    return config;
  }

  /**
   * 探测某个测试账号是否能在当前本地环境成功登录
   * @param {string} accountKey - network runtime account key
   * @returns {Promise<boolean>} true 表示账号当前可用
   */
  async isAccountAvailable(accountKey) {
    try {
      await this.login(accountKey);
      return true;
    } catch (err) {
      return false;
    } finally {
      this.clearToken(accountKey);
    }
  }
}

const apiClient = new ApiClient();
module.exports = apiClient;
