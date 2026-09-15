/**
 * 文件用途：锁住 createServiceConfig 的"默认环境"行为。
 *
 * 背景（2026-09-15）：该函数的默认值曾是硬编码的 'dev'，于是
 * `vite build` 只要没显式带 VITE_SERVICE_ENV，就会把绝对地址
 * http://127.0.0.1:9999/api/v1 打进产物。后果有两个：
 *   1. 真实部署指向 localhost；
 *   2. 本地预览代理下浏览器绕过代理直连 9999 被 CORS 拦，
 *      表现为**整站白屏**（SPA 不挂载），排查成本很高。
 *
 * 修复：默认值跟随 Vite 的构建模式（env.PROD）。
 * 本测试就是防止有人把它改回硬编码 'dev' —— 那种改动不会让任何
 * 现有用例变红，只会在下次构建时静默炸掉。
 */

import { describe, expect, it } from 'vitest';
import { createServiceConfig } from '~/env.config';

const DEV_URL = 'http://127.0.0.1:9999/api/v1';

/** 构造一个最小的 import.meta.env 替身 */
function makeEnv(overrides: Record<string, unknown> = {}): Env.ImportMeta {
  return { PROD: false, ...overrides } as unknown as Env.ImportMeta;
}

describe('createServiceConfig', () => {
  describe('显式指定 VITE_SERVICE_ENV', () => {
    it('dev → 直连本地后端（绝对地址）', () => {
      const config = createServiceConfig(makeEnv({ VITE_SERVICE_ENV: 'dev', PROD: true }));
      expect(config.baseURL).toBe(DEV_URL);
      expect(config.sseEndpoint).toBe('/proxy-default/events');
    });

    it('prod → 空串（同源相对路径）', () => {
      const config = createServiceConfig(makeEnv({ VITE_SERVICE_ENV: 'prod', PROD: false }));
      expect(config.baseURL).toBe('');
      expect(config.sseEndpoint).toBe('/api/v1/events');
    });

    it('显式值优先于构建模式', () => {
      // 生产构建但显式要 dev：应当尊重显式值（保留逃生口）
      expect(createServiceConfig(makeEnv({ VITE_SERVICE_ENV: 'dev', PROD: true })).baseURL).toBe(DEV_URL);
      // 开发模式但显式要 prod
      expect(createServiceConfig(makeEnv({ VITE_SERVICE_ENV: 'prod', PROD: false })).baseURL).toBe('');
    });
  });

  describe('未指定 VITE_SERVICE_ENV 时跟随构建模式', () => {
    it('vite build（PROD=true）→ prod，绝不落成绝对 localhost 地址', () => {
      const config = createServiceConfig(makeEnv({ PROD: true }));
      expect(config.baseURL).toBe('');
      expect(config.baseURL).not.toContain('127.0.0.1');
    });

    it('vite dev（PROD=false）→ dev，保持开发直连体验', () => {
      expect(createServiceConfig(makeEnv({ PROD: false })).baseURL).toBe(DEV_URL);
    });
  });

  describe('VITE_DEV_API_URL 覆盖', () => {
    it('dev 环境的地址可被覆盖，用于指向另起的后端', () => {
      const config = createServiceConfig(
        makeEnv({ VITE_SERVICE_ENV: 'dev', VITE_DEV_API_URL: 'http://10.0.0.5:8888/api/v1' })
      );
      expect(config.baseURL).toBe('http://10.0.0.5:8888/api/v1');
    });

    it('空白覆盖值应回落到默认地址，而不是变成空串', () => {
      const config = createServiceConfig(makeEnv({ VITE_SERVICE_ENV: 'dev', VITE_DEV_API_URL: '   ' }));
      expect(config.baseURL).toBe(DEV_URL);
    });
  });

  it('otherBaseURL.platform 与 baseURL 保持一致', () => {
    expect(createServiceConfig(makeEnv({ PROD: true })).otherBaseURL.platform).toBe('');
    expect(createServiceConfig(makeEnv({ PROD: false })).otherBaseURL.platform).toBe(DEV_URL);
  });
});
