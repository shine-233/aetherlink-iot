/**
 * 文件用途：用于支撑 automation_tests 的API 自动化共享客户端。
 * 核心逻辑：封装自动化运行所需的配置、客户端、覆盖率、报告、种子数据或断言能力，供 API 与 E2E 套件复用。
 * 关键注意事项：共享库变更会影响多类自动化套件，必须保持错误信息和前置条件可诊断。
 * 重构建议：继续按职责拆分深模块，避免把运行配置、业务断言和报告生成耦合在同一入口。
 */

const axios = require('axios');
const FormData = require('form-data');
const fs = require('fs');
const endpointCoverage = require('./endpoint_coverage');
const config = require('./runtime_config');

class ApiClient {
  constructor() {
    this.baseURL = config.baseURL;
    this.timeout = config.timeout;
    this.tokens = {};
    this.client = axios.create({
      baseURL: this.baseURL,
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
   * @param {string} accountKey - config.accounts 中的 key
   * @returns {Promise<string>} 登录成功后的 token
   * @throws {Error} 账号未配置、HTTP 错误、业务码非 200 时抛出
   */
  async login(accountKey = 'tenant_admin') {
    const account = config.accounts[accountKey];
    if (!account) {
      throw new Error(`测试账号 ${accountKey} 未在 config.json 中配置`);
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
      throw new Error(`登录失败: ${JSON.stringify(resp.data)}`);
    } catch (err) {
      if (err.response) {
        throw new Error(`登录请求失败 [${err.response.status}]: ${JSON.stringify(err.response.data)}`);
      }
      throw new Error(`登录请求异常: ${err.message}`);
    }
  }

  /**
   * 获取指定账号的 Token（如未登录则自动登录）
   * 注意：Token 缓存在内存中，不会自动检测过期；如需强制重新登录请先 clearToken
   * @param {string} accountKey - config.accounts 中的 key
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
   * @param {string} accountKey - config.accounts 中的 key
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
   * @param {string} accountKey - config.accounts 中的 key
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

  /**
   * 通用 GET 请求
   * 请求失败时返回标准化错误对象（含 _requestError: true），不抛出异常
   * @param {string} url - 接口路径（相对 baseURL）
   * @param {object} params - 查询参数
   * @param {string} accountKey - 使用的账号
   * @returns {Promise<object>} 后端响应体或错误对象
   */
  async get(url, params = {}, accountKey = 'tenant_admin') {
    const headers = await this.authHeaders(accountKey);
    try {
      const resp = await this.client.get(url, { params, headers });
      this.recordEndpointResponse('GET', url);
      return resp.data;
    } catch (err) {
      this.recordEndpointResponse('GET', url, err);
      return this.handleError(err);
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
  async post(url, data = {}, accountKey = 'tenant_admin') {
    const headers = await this.authHeaders(accountKey);
    try {
      const resp = await this.client.post(url, data, { headers });
      this.recordEndpointResponse('POST', url);
      return resp.data;
    } catch (err) {
      this.recordEndpointResponse('POST', url, err);
      return this.handleError(err);
    }
  }

  async upload(url, filePath, fields = {}, accountKey = 'tenant_admin') {
    const auth = await this.authHeaders(accountKey);
    const form = new FormData();
    form.append('file', fs.createReadStream(filePath));
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
      return this.handleError(err);
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
  async put(url, data = {}, accountKey = 'tenant_admin') {
    const headers = await this.authHeaders(accountKey);
    try {
      const resp = await this.client.put(url, data, { headers });
      this.recordEndpointResponse('PUT', url);
      return resp.data;
    } catch (err) {
      this.recordEndpointResponse('PUT', url, err);
      return this.handleError(err);
    }
  }

  async patch(url, data = {}, accountKey = 'tenant_admin') {
    const headers = await this.authHeaders(accountKey);
    try {
      const resp = await this.client.patch(url, data, { headers });
      this.recordEndpointResponse('PATCH', url);
      return resp.data;
    } catch (err) {
      this.recordEndpointResponse('PATCH', url, err);
      return this.handleError(err);
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
  async delete(url, data = {}, accountKey = 'tenant_admin') {
    const headers = await this.authHeaders(accountKey);
    try {
      const resp = await this.client.delete(url, { headers, data });
      this.recordEndpointResponse('DELETE', url);
      return resp.data;
    } catch (err) {
      this.recordEndpointResponse('DELETE', url, err);
      return this.handleError(err);
    }
  }

  /**
   * 无认证 GET 请求（用于公开接口，如 /systime、/logo）
   * @param {string} url - 接口路径（相对 baseURL）
   * @param {object} params - 查询参数
   * @returns {Promise<object>} 后端响应体或错误对象
   */
  async getNoAuth(url, params = {}) {
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
      const resp = await axios.get(config.healthURL, { timeout: 5000 });
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
   * 获取配置对象（同步返回）
   * @returns {object} config.json 解析后的配置
   */
  getConfig() {
    return config;
  }

  /**
   * 探测某个测试账号是否能在当前本地环境成功登录
   * @param {string} accountKey - config.accounts 中的 key
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
