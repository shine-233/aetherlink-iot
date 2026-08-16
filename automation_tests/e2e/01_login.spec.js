/**
 * 文件用途：用于提供登录认证 E2E 流程的 Playwright 用户可见证据。
 * 核心逻辑：通过共享 fixture 登录指定角色，访问本地或预览路由，并断言页面内容、权限边界或种子状态。
 * 关键注意事项：只有结合真实本地账号、稳定种子数据和可见状态断言时，才可计入对应业务信心。
 * 已落地：登录成功用例同时通过 /board/user/info API 交叉校验角色/邮箱，并硬断言落地页业务文案可见，
 * 不再保留路由冒烟式弱断言。
 */

const { test, expect } = require('./fixtures');
const config = require('../lib/network_runtime');
const selectors = require('./selectors');

const LOGIN_URL = /\/login(?:[/?#]|$)/;
const AUTH_KEYS = ['token', 'userInfo', 'token_expires_in'];
const AUTH_LANDING_PAGES = {
  SYS_ADMIN: {
    path: '/management/setting',
    // 后端 i18n 默认中文；接受中英文任一渲染
    text: /System Setting|系统设置/
  },
  TENANT_ADMIN: {
    path: '/device/manage',
    text: /Device Management|设备管理/
  }
};
const PREFERRED_LANGUAGE_API = '/proxy-default/user/prefer-lang';
const LOCALE_STORAGE_KEYS = ['lang'];
const LOCALE_BUTTON_TEXT = /^(中文|English|Francais|Français|Espanol|Español)$/;

function loginControls(page) {
  return {
    email: page.locator(selectors.login.email).first(),
    password: page.locator(selectors.login.password).first(),
    submit: page.locator(selectors.login.submit).first()
  };
}

function languageButton(page) {
  return page.getByRole('button', { name: LOCALE_BUTTON_TEXT }).first();
}

function forgotPasswordButton(page) {
  return page.locator(selectors.login.forgotPassword).first();
}

async function readStorageEntry(page, storageName, key) {
  return page.evaluate(
    ({ storageName, key }) => {
      try {
        const storage = window[storageName];
        const value = storage.getItem(key);
        if (value === null) {
          return null;
        }

        try {
          return JSON.parse(value);
        } catch (error) {
          if (!(error instanceof SyntaxError)) {
            throw error;
          }
          return value;
        }
      } catch (error) {
        if (error && error.name === 'SecurityError') {
          return null;
        }
        throw error;
      }
    },
    { storageName, key }
  );
}

async function readAuthStorage(page) {
  const state = {};

  for (const key of AUTH_KEYS) {
    state['local:' + key] = await readStorageEntry(page, 'localStorage', key);
    state['session:' + key] = await readStorageEntry(page, 'sessionStorage', key);
  }

  return state;
}

async function expectLoginPage(page) {
  const { email, password, submit } = loginControls(page);

  await expect(page).toHaveURL(LOGIN_URL);
  await expect(email).toBeEditable();
  await expect(password).toBeEditable();
  await expect(submit).toBeVisible();
  await expect(submit).toBeEnabled();
}

async function gotoLogin(page) {
  await page.goto('/login', { waitUntil: 'domcontentloaded' });
  await expectLoginPage(page);
}

async function forceAnonymousEnglishLoginPage(page) {
  await page.context().clearCookies();
  await gotoLogin(page);

  await page.evaluate(({ authKeys, localeKeys }) => {
    try {
      for (const key of [...authKeys, ...localeKeys]) {
        localStorage.removeItem(key);
        sessionStorage.removeItem(key);
      }
      localStorage.setItem('lang', JSON.stringify('en-US'));
    } catch (error) {
      if (!error || error.name !== 'SecurityError') {
        throw error;
      }
    }
  }, { authKeys: AUTH_KEYS, localeKeys: LOCALE_STORAGE_KEYS });

  await page.goto('/login?redirect=/home', { waitUntil: 'domcontentloaded' });
  await expectLoginPage(page);
}

async function fillAndSubmitLogin(page, account) {
  const { email, password, submit } = loginControls(page);

  await email.fill(account.email);
  await expect(email).toHaveValue(account.email);
  await password.fill(account.password);
  await expect(password).toHaveValue(account.password);
  await submit.click();
}

async function expectLoggedIn(page, account, api, accountKey) {
  await expect
    .poll(async () => {
      const state = await readAuthStorage(page);
      return Boolean(state['local:token'] && state['local:userInfo'] && state['local:token_expires_in']);
    }, { timeout: 30000 })
    .toBe(true);

  const state = await readAuthStorage(page);
  const userInfo = state['local:userInfo'] || {};
  const roles = Array.isArray(userInfo.roles) ? userInfo.roles : [userInfo.authority].filter(Boolean);
  const landing = AUTH_LANDING_PAGES[account.role];

  expect(String(state['local:token'] || '')).not.toBe('');
  expect(Number(state['local:token_expires_in'])).toBeGreaterThan(Date.now());
  expect(roles).toContain(account.role);

  // API 交叉校验：用同一账号通过后端 /board/user/info 验证认证态与角色匹配，
  // 避免仅凭 localStorage 残留就判定登录成功。
  if (api && accountKey) {
    const profileResp = await api.get('/board/user/info', {}, accountKey);
    expect(profileResp.code, 'board/user/info should succeed with the same account').toBe(200);
    expect(profileResp.data, 'profile data must contain the authenticated account').toEqual(
      expect.objectContaining({
        authority: account.role,
        email: account.email,
        name: expect.any(String)
      })
    );
    const apiAuthority = String(profileResp.data.authority || '').trim();
    const apiEmail = String(profileResp.data.email || '').trim();
    expect(apiAuthority, 'API authority must match the logged-in role').toBe(account.role);
    expect(apiEmail, 'API email must match the login email').toBe(account.email);
    expect(String(profileResp.data.name || '').trim(), 'API name must be non-empty').not.toBe('');
  }

  if (landing) {
    // 登录后前端会自动导航（通常到 /home）。先等待自动导航完成，在当前页面验证认证 shell。
    await expect(page).not.toHaveURL(LOGIN_URL, { timeout: 15000 });

    // 认证 shell 渲染验证：用户名或菜单文本可见，证明认证 layout 已加载。
    const userName = String(userInfo.name || userInfo.userName || userInfo.user_name || '').trim();
    const shellText = userName || 'System Management|设备管理|Application Management|Visualization|Home';
    await expect(page.getByText(new RegExp(shellText, 'i')).first()).toBeVisible({ timeout: 15000 });

    // 访问落地页并硬断言业务文案可见，暴露前端动态导入缺陷。
    // 仅捕获 ERR_ABORTED（SPA 路由守卫中断），其他导航失败必须暴露为测试失败。
    try {
      await page.goto(landing.path, { waitUntil: 'domcontentloaded', timeout: 15000 });
    } catch (e) {
      const msg = String((e && e.message) || '');
      if (!msg.includes('ERR_ABORTED')) {
        throw e;
      }
    }
    await expect(page).not.toHaveURL(LOGIN_URL, { timeout: 15000 });
    // 落地页业务文案必须可见（硬断言），暴露前端动态导入或菜单权限缺陷
    await expect(page.getByText(landing.text).first()).toBeVisible({ timeout: 15000 });
    return;
  }

  await expect(page).not.toHaveURL(LOGIN_URL, { timeout: 15000 });
  const fallbackName = String(userInfo.name || userInfo.userName || userInfo.user_name || '').trim();
  const fallbackText = fallbackName || 'System Management|设备管理|Application Management|Visualization|Home';
  await expect(page.getByText(new RegExp(fallbackText, 'i')).first()).toBeVisible({ timeout: 15000 });
}

async function expectNoAuthStorage(page) {
  await expect
    .poll(async () => {
      const state = await readAuthStorage(page);
      return AUTH_KEYS.every(key => !state['local:' + key] && !state['session:' + key]);
    }, { timeout: 15000 })
    .toBe(true);
}

async function clearAuthStorage(page) {
  await page.evaluate(keys => {
    try {
      for (const key of keys) {
        localStorage.removeItem(key);
        sessionStorage.removeItem(key);
      }
    } catch (error) {
      if (!error || error.name !== 'SecurityError') {
        throw error;
      }
    }
  }, AUTH_KEYS);

  await expectNoAuthStorage(page);
}

test.describe('login auth module', () => {
  test('anonymous language switch stays local and forgot password stays usable', async ({ page }) => {
    const preferredLanguageRequests = [];
    const failedPreferredLanguageRequests = [];

    page.on('request', request => {
      if (request.url().includes(PREFERRED_LANGUAGE_API)) {
        preferredLanguageRequests.push(request.url());
      }
    });
    page.on('requestfailed', request => {
      if (request.url().includes(PREFERRED_LANGUAGE_API)) {
        failedPreferredLanguageRequests.push({
          url: request.url(),
          failure: request.failure()?.errorText || ''
        });
      }
    });

    await forceAnonymousEnglishLoginPage(page);
    await expect(languageButton(page)).toBeVisible();
    expect(await readStorageEntry(page, 'localStorage', 'lang')).toBe('en-US');

    await languageButton(page).click();

    await expect(languageButton(page)).toHaveText(/Français|Fran莽ais/);
    await expect
      .poll(() => readStorageEntry(page, 'localStorage', 'lang'))
      .toBe('fr-FR');
    await expect(page.locator(selectors.common.errorMessage)).toHaveCount(0);
    await expect
      .poll(() => preferredLanguageRequests.length, { timeout: 3000 })
      .toBe(0);
    expect(failedPreferredLanguageRequests).toEqual([]);

    await expect(forgotPasswordButton(page)).toBeVisible({ timeout: 15000 });
    await forgotPasswordButton(page).click();

    await expect(page).toHaveURL(/\/login\/reset-pwd(?:[/?#]|$)/, { timeout: 15000 });
    await expect(page.getByPlaceholder(/请输入邮件地址|Enter email address|Saisir adresse courriel|Ingrese direccion de correo/)).toBeVisible();
  });

  test('password-login module route renders the real password form', async ({ page }) => {
    await forceAnonymousEnglishLoginPage(page);
    await page.goto('/login/pwd-login', { waitUntil: 'domcontentloaded' });

    await expect(page).toHaveURL(/\/login\/pwd-login(?:[/?#]|$)/);
    const { email, password, submit } = loginControls(page);
    await expect(email).toBeEditable();
    await expect(password).toBeEditable();
    await expect(submit).toBeVisible();
    await expect(page.getByRole('button', { name: /Forgot Password/i })).toBeVisible();
  });

  test('phone-register module route renders validation and returns to password login', async ({ page }) => {
    await forceAnonymousEnglishLoginPage(page);
    await page.goto('/login/register', { waitUntil: 'domcontentloaded' });

    await expect(page).toHaveURL(/\/login\/register(?:[/?#]|$)/);
    await expect(page.getByPlaceholder(/Phone number/i)).toBeEditable();
    await expect(page.getByPlaceholder(/Verification code/i)).toBeEditable();
    await expect(page.getByPlaceholder(/^Enter password$/i)).toBeEditable();
    await expect(page.getByPlaceholder(/Confirm password/i)).toBeEditable();

    // Submitting empty data exercises the actual form rules without creating
    // an account or calling an external SMS provider.
    await page.getByRole('button', { name: /^Confirm$/i }).click();
    await expect(page.locator('.n-form-item-feedback-wrapper').first()).toBeVisible();

    await page.getByRole('button', { name: /^Back$/i }).click();
    await expect(page).toHaveURL(/\/login\/pwd-login(?:[/?#]|$)/);
    await expect(page.getByTestId('login-submit')).toBeVisible();
  });

  test('super admin login reaches the authenticated shell', async ({ page, api }) => {
    test.slow();
    const account = config.accounts.super_admin;

    await gotoLogin(page);
    await fillAndSubmitLogin(page, account);
    await expectLoggedIn(page, account, api, 'super_admin');
  });

  test('tenant admin login reaches the authenticated shell', async ({ page, api }) => {
    test.slow();
    const account = config.accounts.tenant_admin;

    await gotoLogin(page);
    await fillAndSubmitLogin(page, account);
    await expectLoggedIn(page, account, api, 'tenant_admin');
  });

  test('second tenant admin login reaches the authenticated shell', async ({ page, api }) => {
    test.slow();
    const account = config.accounts.tenant_admin_b;

    await gotoLogin(page);
    await fillAndSubmitLogin(page, account);
    await expectLoggedIn(page, account, api, 'tenant_admin_b');
  });

  test('tenant user login reaches the authenticated shell', async ({ page, api }) => {
    test.slow();
    const account = config.accounts.tenant_user;

    await gotoLogin(page);
    await fillAndSubmitLogin(page, account);
    await expectLoggedIn(page, account, api, 'tenant_user');
  });

  test('additional tenant user login reaches the authenticated shell', async ({ page, api }) => {
    test.slow();
    const account = config.accounts.readonly_user;

    await gotoLogin(page);
    await fillAndSubmitLogin(page, account);
    await expectLoggedIn(page, account, api, 'readonly_user');
  });

  test('email-change tenant login reaches the authenticated shell', async ({ page, api }) => {
    test.slow();
    const account = config.accounts.email_change_tenant;

    await gotoLogin(page);
    await fillAndSubmitLogin(page, account);
    await expectLoggedIn(page, account, api, 'email_change_tenant');
  });

  test('wrong password keeps the user on the login page', async ({ page }) => {
    const account = config.accounts.tenant_admin;
    await gotoLogin(page);

    const { email, password, submit } = loginControls(page);

    await email.fill(account.email);
    await password.fill('WrongPassword@999');
    await submit.click();

    await expect(page.locator(selectors.login.errorMessage).first()).toBeVisible({ timeout: 15000 });
    await expectLoginPage(page);
    await expectNoAuthStorage(page);
  });

  test('clearing auth state forces the next protected route back to login', async ({ rolePage }) => {
    await rolePage.goto('/', { waitUntil: 'domcontentloaded' });
    // 通过自带等待的可见性断言确认认证 shell 已完成渲染。
    await expect(rolePage.getByText(/System Management|设备管理|Application Management|Visualization/i).first()).toBeVisible({ timeout: 15000 });
    await expect
      .poll(async () => {
        const state = await readAuthStorage(rolePage);
        return Boolean(state['local:token']);
      }, { timeout: 15000 })
      .toBe(true);

    await clearAuthStorage(rolePage);
    // SPA 路由在检测到无 token 时会中断导航并重定向到 /login，goto 可能抛 ERR_ABORTED。
    // 捕获后验证最终落在登录页即可。
    try {
      await rolePage.goto('/device/manage', { waitUntil: 'domcontentloaded' });
    } catch (e) {
      const message = String((e && e.message) || e);
      if (!message.includes('ERR_ABORTED')) {
        throw e;
      }
    }
    await expectLoginPage(rolePage);
  });

  test('visiting a protected route without a token redirects to login', async ({ browser }) => {
    const context = await browser.newContext({ baseURL: config.frontendURL });
    const page = await context.newPage();

    try {
      await page.goto('/login', { waitUntil: 'domcontentloaded' });
      await clearAuthStorage(page);
      await page.goto('/device/manage', { waitUntil: 'domcontentloaded' });
      await expectLoginPage(page);
    } finally {
      await context.close();
    }
  });
});
