/**
 * 文件用途: 认证与用户 API wrapper 的请求合同测试。
 * 核心逻辑: mock request 层后验证登录、用户信息、登出、验证码、用户管理和初始化接口。
 * 关键注意事项: token 生命周期、角色权限和初始化状态还需要 store、路由 guard 和后端测试共同证明。
 * 重构建议: 拆分登录、用户管理、初始化三组用例，并补充异常响应和空字段边界。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest';

const { mockGet, mockPost, mockPut, mockDelete, mockLocalStgGet } = vi.hoisted(() => ({
  mockGet: vi.fn(),
  mockPost: vi.fn(),
  mockPut: vi.fn(),
  mockDelete: vi.fn(),
  mockLocalStgGet: vi.fn(() => 'fr-FR')
}));

vi.mock('@/service/request', () => ({
  request: {
    get: mockGet,
    post: mockPost,
    put: mockPut,
    delete: mockDelete
  }
}));

vi.mock('@/utils/storage', () => ({
  localStg: {
    get: mockLocalStgGet
  }
}));

import {
  fetchLogin,
  fetchGetUserInfo,
  logout,
  fetchEmailCode,
  fetchEmailCodeByEmail,
  fetchUserList,
  addUser,
  editUser,
  delUser,
  transformUser,
  editUserPassWord,
  requestPasswordResetLink,
  fetchCompatHomeConfig,
  registerByEmail,
  fetchHasAdmin,
  fetchTenantSetupState,
  fetchSuperAdminInit
} from '../auth';

describe('Auth API 层 - auth.ts', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('fetchLogin', () => {
    it('调用 POST /login 并发送 email、password、salt', async () => {
      mockPost.mockResolvedValue({ error: null, data: { token: 't1' } });
      await fetchLogin('user@test.com', 'pass123', 'salt1');
      expect(mockPost).toHaveBeenCalledTimes(1);
      expect(mockPost).toHaveBeenCalledWith('/login', { email: 'user@test.com', password: 'pass123', salt: 'salt1' });
    });

    it('salt 为 null 时也正确传递', async () => {
      mockPost.mockResolvedValue({ error: null, data: { token: 't1' } });
      await fetchLogin('user@test.com', 'pass123', null);
      expect(mockPost).toHaveBeenCalledWith('/login', { email: 'user@test.com', password: 'pass123', salt: null });
    });
  });

  describe('fetchGetUserInfo', () => {
    it('调用 GET /user/detail', async () => {
      mockGet.mockResolvedValue({ error: null, data: { user_id: 'u1' } });
      await fetchGetUserInfo();
      expect(mockGet).toHaveBeenCalledTimes(1);
      expect(mockGet).toHaveBeenCalledWith('/user/detail');
    });
  });

  describe('logout', () => {
    it('调用 GET /user/logout', async () => {
      mockGet.mockResolvedValue({ error: null, data: null });
      await logout();
      expect(mockGet).toHaveBeenCalledTimes(1);
      expect(mockGet).toHaveBeenCalledWith('/user/logout');
    });
  });

  describe('fetchEmailCode', () => {
    it('调用 GET /verification/code 并携带 email 和 is_register=1', async () => {
      mockGet.mockResolvedValue({ error: null, data: { email: 'test@test.com', is_register: 1 } });
      await fetchEmailCode('test@test.com');
      expect(mockGet).toHaveBeenCalledTimes(1);
      expect(mockGet).toHaveBeenCalledWith('/verification/code', {
        params: { email: 'test@test.com', is_register: 1, language: 'fr-FR' }
      });
    });
  });

  describe('fetchEmailCodeByEmail', () => {
    it('调用 GET /verification/code 并携带 email 和 is_register=2', async () => {
      mockGet.mockResolvedValue({ error: null, data: { email: 'test@test.com', is_register: 2 } });
      await fetchEmailCodeByEmail('test@test.com');
      expect(mockGet).toHaveBeenCalledTimes(1);
      expect(mockGet).toHaveBeenCalledWith('/verification/code', {
        params: { email: 'test@test.com', is_register: 2, language: 'fr-FR' }
      });
    });
  });

  describe('fetchUserList', () => {
    it('调用 GET /user 并携带查询参数', async () => {
      mockGet.mockResolvedValue({ error: null, data: { list: [] } });
      const params = { page: 1, page_size: 10 };
      await fetchUserList(params);
      expect(mockGet).toHaveBeenCalledTimes(1);
      expect(mockGet).toHaveBeenCalledWith('/user', { params });
    });
  });

  describe('addUser', () => {
    it('调用 POST /user 并发送用户数据', async () => {
      mockPost.mockResolvedValue({ error: null, data: {} });
      const params = { email: 'new@test.com', password: '123456' };
      await addUser(params);
      expect(mockPost).toHaveBeenCalledTimes(1);
      expect(mockPost).toHaveBeenCalledWith('/user', params);
    });
  });

  describe('editUser', () => {
    it('调用 PUT /user 并发送编辑数据', async () => {
      mockPut.mockResolvedValue({ error: null, data: {} });
      const params = { id: 'u1', name: '新名称' };
      await editUser(params);
      expect(mockPut).toHaveBeenCalledTimes(1);
      expect(mockPut).toHaveBeenCalledWith('/user', params);
    });
  });

  describe('delUser', () => {
    it('调用 DELETE /user/{id}', async () => {
      mockDelete.mockResolvedValue({ error: null, data: {} });
      await delUser('u1');
      expect(mockDelete).toHaveBeenCalledTimes(1);
      expect(mockDelete).toHaveBeenCalledWith('/user/u1');
    });
  });

  describe('transformUser', () => {
    it('调用 POST /user/transform 并发送切换数据', async () => {
      mockPost.mockResolvedValue({ error: null, data: { token: 't2' } });
      const params = { user_id: 'u2' };
      await transformUser(params);
      expect(mockPost).toHaveBeenCalledTimes(1);
      expect(mockPost).toHaveBeenCalledWith('/user/transform', params);
    });
  });

  describe('editUserPassWord', () => {
    it('调用 POST /reset/password 并发送密码修改数据', async () => {
      mockPost.mockResolvedValue({ error: null, data: {} });
      const params = { old_password: 'old', new_password: 'new' };
      await editUserPassWord(params);
      expect(mockPost).toHaveBeenCalledTimes(1);
      expect(mockPost).toHaveBeenCalledWith('/reset/password', params);
    });
  });

  describe('requestPasswordResetLink', () => {
    it('调用 POST /reset/password/link 并发送邮箱验证码', async () => {
      mockPost.mockResolvedValue({ error: null, data: { expires_in: 900 } });
      const params = { email: 'user@test.com', verify_code: '123456' };
      await requestPasswordResetLink(params);
      expect(mockPost).toHaveBeenCalledTimes(1);
      expect(mockPost).toHaveBeenCalledWith('/reset/password/link', params);
    });
  });

  describe('fetchCompatHomeConfig', () => {
    it('调用 GET /board/home 并携带查询参数', async () => {
      mockGet.mockResolvedValue({ error: null, data: { config: '{}' } });
      const params = {};
      await fetchCompatHomeConfig(params);
      expect(mockGet).toHaveBeenCalledTimes(1);
      expect(mockGet).toHaveBeenCalledWith('/board/home', { params });
    });
  });

  describe('registerByEmail', () => {
    it('调用 POST /tenant/email/register 并携带 JSON 头', async () => {
      mockPost.mockResolvedValue({ error: null, data: {} });
      const data = {
        email: 'reg@test.com',
        verify_code: '123456',
        password: 'pass123',
        phone_prefix: '+86',
        phone_number: '13800138000'
      };
      await registerByEmail(data);
      expect(mockPost).toHaveBeenCalledTimes(1);
      expect(mockPost).toHaveBeenCalledWith('/tenant/email/register', data, {
        headers: { 'Content-Type': 'application/json' }
      });
    });
  });

  describe('fetchHasAdmin', () => {
    it('调用 GET /tenant/has-admin', async () => {
      mockGet.mockResolvedValue({ error: null, data: { has_admin: true } });
      await fetchHasAdmin();
      expect(mockGet).toHaveBeenCalledTimes(1);
      expect(mockGet).toHaveBeenCalledWith('/tenant/has-admin');
    });
  });

  describe('fetchTenantSetupState', () => {
    it('调用 GET /tenant/setup-state', async () => {
      mockGet.mockResolvedValue({ error: null, data: { has_admin: false, entry: 'register' } });
      await fetchTenantSetupState();
      expect(mockGet).toHaveBeenCalledTimes(1);
      expect(mockGet).toHaveBeenCalledWith('/tenant/setup-state');
    });
  });

  describe('fetchSuperAdminInit', () => {
    it('成功时调用 POST /tenant/super-admin/init', async () => {
      mockPost.mockResolvedValue({ error: null, data: {} });
      const data = { email: 'admin@test.com', password: 'admin123' };
      await fetchSuperAdminInit(data);
      expect(mockPost).toHaveBeenCalledTimes(1);
      expect(mockPost).toHaveBeenCalledWith('/tenant/super-admin/init', data, {
        headers: { 'Content-Type': 'application/json' }
      });
    });

    it('404 时回退到 POST /tenant/market-register', async () => {
      const error404 = { response: { status: 404, data: { code: 100404 } } };
      mockPost.mockRejectedValueOnce(error404);
      mockPost.mockResolvedValueOnce({ error: null, data: {} });
      const data = { email: 'admin@test.com', password: 'admin123' };
      await fetchSuperAdminInit(data);
      expect(mockPost).toHaveBeenCalledTimes(2);
      expect(mockPost).toHaveBeenNthCalledWith(1, '/tenant/super-admin/init', data, {
        headers: { 'Content-Type': 'application/json' }
      });
      expect(mockPost).toHaveBeenNthCalledWith(2, '/tenant/market-register', data, {
        headers: { 'Content-Type': 'application/json' }
      });
    });

    it('code 100404 时回退到 POST /tenant/market-register', async () => {
      const errorCode = { response: { status: 500, data: { code: 100404 } } };
      mockPost.mockRejectedValueOnce(errorCode);
      mockPost.mockResolvedValueOnce({ error: null, data: {} });
      const data = { email: 'admin@test.com', password: 'admin123' };
      await fetchSuperAdminInit(data);
      expect(mockPost).toHaveBeenCalledTimes(2);
      expect(mockPost).toHaveBeenNthCalledWith(2, '/tenant/market-register', data, {
        headers: { 'Content-Type': 'application/json' }
      });
    });

    it('其他错误时抛出异常', async () => {
      const otherError = { response: { status: 500, data: { code: 500 } } };
      mockPost.mockRejectedValue(otherError);
      const data = { email: 'admin@test.com', password: 'admin123' };
      await expect(fetchSuperAdminInit(data)).rejects.toBe(otherError);
    });
  });

});
