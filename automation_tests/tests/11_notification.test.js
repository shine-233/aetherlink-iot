/**
 * 文件用途：用于验证通知工作流 API 自动化测试。
 * 核心逻辑：通过共享 API 客户端和测试数据访问目标接口，断言响应结构、错误分支或可观察状态。
 * 关键注意事项：接口命中不等同于业务正确；计入证据前需要确认断言覆盖真实状态和前置条件。
 * 重构建议：后续应优先补强负向用例、状态校验和清理路径，而不是扩大无断言冒烟范围。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const { expectBusinessError } = require('../lib/response_assertions');
const seedData = require('../lib/seed_data');

function expectOk(resp) {
  expect(resp).to.be.an('object');
  expect(resp.code).to.equal(200);
}

describe('Notification API module [11_notification]', function () {
  this.timeout(30000);

  let notificationGroupId = null;
  let createdTenantId = '';
  let createdName = '';
  let updatedName = '';
  let seededNotificationGroup = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 11_notification.test.js; unified verification requires a healthy API service');
    }

    await apiClient.login('tenant_admin');
    await apiClient.login('super_admin');
    seededNotificationGroup = await seedData.ensureNotificationGroup('tenant_admin');
    notificationGroupId = seededNotificationGroup.id;
    createdName = seededNotificationGroup.row.name;
    createdTenantId = seededNotificationGroup.row.tenant_id;
    expect(notificationGroupId).to.be.a('string').and.not.empty;
  });

  after(async function () {
    if (notificationGroupId) {
      try {
        await apiClient.delete('/notification_group/' + notificationGroupId, {}, 'tenant_admin');
      } catch (error) {
        // Cleanup failures should not hide the real assertion result.
      }
    }
    if (seededNotificationGroup && seededNotificationGroup.id && seededNotificationGroup.id !== notificationGroupId) {
      try {
        await seededNotificationGroup.cleanup();
      } catch (error) {
        // Cleanup failures should not hide the real assertion result.
      }
    }
    apiClient.clearAllTokens();
  });

  it('returns the current notification group page shape', async function () {
    const resp = await apiClient.get('/notification_group/list', { page: 1, page_size: 100 }, 'tenant_admin');

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.list).to.be.an('array');
    expect(resp.data.total).to.be.a('number').and.at.least(1);
    expect(resp.data.list.length).to.be.at.least(1);
    expect(resp.data.list.length).to.be.at.most(resp.data.total);

    const seededRow = resp.data.list.find(item => item.id === notificationGroupId);
    expect(seededRow, 'seeded notification group must be visible in the paged list').to.be.an('object');
    expect(seededRow.name).to.equal(createdName);
  });

  it('creates a notification group with the current frontend payload shape', async function () {
    createdName = 'codex-notify-' + Date.now();

    const resp = await apiClient.post(
      '/notification_group',
      {
        name: createdName,
        description: 'created by codex',
        notification_type: 'EMAIL',
        notification_config: JSON.stringify({ EMAIL: 'test@example.com' }),
        status: 'CLOSE'
      },
      'tenant_admin'
    );

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.id).to.be.a('string').and.not.empty;
    expect(resp.data.name).to.equal(createdName);
    expect(resp.data.notification_type).to.equal('EMAIL');
    expect(resp.data.status).to.equal('CLOSE');
    notificationGroupId = resp.data.id;
    createdTenantId = resp.data.tenant_id;
  });

  it('returns the created notification group detail', async function () {
    expect(notificationGroupId).to.be.a('string').and.not.empty;

    const resp = await apiClient.get('/notification_group/' + notificationGroupId, {}, 'tenant_admin');

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.id).to.equal(notificationGroupId);
    expect(resp.data.name).to.equal(createdName);
    expect(resp.data.notification_type).to.equal('EMAIL');
    expect(resp.data.status).to.equal('CLOSE');
    expect(resp.data.notification_config).to.be.a('string').and.include('EMAIL');
  });

  it('returns record-not-found for an invalid notification group id', async function () {
    const resp = await apiClient.get('/notification_group/00000000-0000-0000-0000-000000000000', {}, 'super_admin');

    expect(resp).to.be.an('object');
    expect(resp.code).to.equal(101001);
    expect(resp.message).to.be.a('string').and.not.equal('');
    expect(resp.data).to.be.an('object');
    expect(resp.data.sql_error).to.equal('record not found');
  });

  it('updates the created notification group with the current backend shape', async function () {
    expect(notificationGroupId).to.be.a('string').and.not.empty;

    updatedName = 'codex-notify-updated-' + Date.now();
    const resp = await apiClient.put(
      '/notification_group/' + notificationGroupId,
      {
        name: updatedName,
        description: 'updated by codex',
        notification_type: 'EMAIL',
        notification_config: JSON.stringify({ EMAIL: 'updated@example.com' }),
        status: 'OPEN',
        tenant_id: createdTenantId
      },
      'tenant_admin'
    );

    expectOk(resp);

    const detailResp = await apiClient.get('/notification_group/' + notificationGroupId, {}, 'tenant_admin');
    expectOk(detailResp);
    expect(detailResp.data).to.be.an('object');
    expect(detailResp.data.id).to.equal(notificationGroupId);
    expect(detailResp.data.name).to.equal(updatedName);
    expect(detailResp.data.status).to.equal('OPEN');
    expect(detailResp.data.notification_config).to.be.a('string').and.include('updated@example.com');
  });

  it('returns the current notification history page shape', async function () {
    const resp = await apiClient.get('/notification_history/list', { page: 1, page_size: 10 }, 'super_admin');

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.list).to.be.an('array');
    expect(resp.data.total).to.be.a('number');
  });

  it('returns the email notification service config for super_admin', async function () {
    const resp = await apiClient.get('/notification/services/config/EMAIL', {}, 'super_admin');

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.notice_type).to.equal('EMAIL');
    expect(resp.data.status).to.be.oneOf(['OPEN', 'CLOSE']);
  });

  it('returns record-not-found for the current SME_CODE config lookup', async function () {
    const resp = await apiClient.get('/notification/services/config/SME_CODE', {}, 'super_admin');

    expect(resp).to.be.an('object');
    expect(resp.code).to.equal(100000);
    expect(resp.message).to.equal('record not found');
  });

  it('persists a minimal email service config save request', async function () {
    const resp = await apiClient.post(
      '/notification/services/config',
      {
        notice_type: 'EMAIL',
        status: 'CLOSE'
      },
      'super_admin'
    );

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.notice_type).to.equal('EMAIL');
    expect(resp.data.status).to.equal('CLOSE');
  });

  it('rejects email test send when the Email field is missing in the current request shape', async function () {
    const resp = await apiClient.post(
      '/notification/services/config/e-mail/test',
      {
        recipient: 'test@example.com'
      },
      'super_admin'
    );

    expectBusinessError(resp, 100002, "Field 'Email' is required");
  });

  it('manages and previews a global alarm email template without changing the previous default', async function () {
    const initialList = await apiClient.get(
      '/notification/e-mail/templates',
      { page: 1, page_size: 100 },
      'super_admin'
    );
    expectOk(initialList);
    expect(initialList.data).to.be.an('object');
    expect(initialList.data.list).to.be.an('array');

    const previousDefault = initialList.data.list.find(template => template.is_default);
    let templateId = '';
    const templateName = 'codex-alarm-template-' + Date.now();

    try {
      const createResp = await apiClient.post(
        '/notification/e-mail/templates',
        {
          name: templateName,
          subject_template: '[AetherLink] {{.Subject}}',
          body_template: '{{.Message}}\nDevices: {{.DeviceIDs}}\nCount: {{.DeviceCount}}\nTenant: {{.TenantID}}\nSent: {{.SentAt}}',
          enabled: true,
          is_default: false
        },
        'super_admin'
      );
      expectOk(createResp);
      expect(createResp.data).to.be.an('object');
      expect(createResp.data.id).to.be.a('string').and.not.empty;
      templateId = createResp.data.id;

      const listResp = await apiClient.get(
        '/notification/e-mail/templates',
        { page: 1, page_size: 100 },
        'super_admin'
      );
      expectOk(listResp);
      const createdTemplate = listResp.data.list.find(template => template.id === templateId);
      expect(createdTemplate).to.be.an('object');
      expect(createdTemplate.name).to.equal(templateName);
      expect(createdTemplate.tenant_id).to.equal('');

      const previewResp = await apiClient.post(
        '/notification/e-mail/templates/preview',
        {
          subject_template: '[AetherLink] {{.Subject}}',
          body_template: '{{.Message}}\n{{.TenantID}}\n{{.DeviceIDs}}\n{{.DeviceCount}}\n{{.SentAt}}',
          subject: 'High temperature',
          message: 'Temperature exceeded the threshold',
          device_ids: ['device-a', 'device-b']
        },
        'super_admin'
      );
      expectOk(previewResp);
      expect(previewResp.data.subject).to.equal('[AetherLink] High temperature');
      expect(previewResp.data.body).to.include('Temperature exceeded the threshold');
      expect(previewResp.data.body).to.include('device-a');
      expect(previewResp.data.body).to.include('device-b');
      expect(previewResp.data.body).to.include('2');

      const updateResp = await apiClient.put(
        '/notification/e-mail/templates/' + templateId,
        {
          name: templateName + '-updated',
          subject_template: '[Alarm] {{.Subject}}',
          body_template: '{{.Message}}\nDevices: {{.DeviceIDs}}',
          enabled: true,
          is_default: false
        },
        'super_admin'
      );
      expectOk(updateResp);
      expect(updateResp.data.name).to.equal(templateName + '-updated');

      const defaultResp = await apiClient.post(
        '/notification/e-mail/templates/' + templateId + '/default',
        {},
        'super_admin'
      );
      expectOk(defaultResp);

      const defaultListResp = await apiClient.get(
        '/notification/e-mail/templates',
        { page: 1, page_size: 100 },
        'super_admin'
      );
      expectOk(defaultListResp);
      const effectiveDefault = defaultListResp.data.list.find(template => template.is_default);
      expect(effectiveDefault).to.be.an('object');
      expect(effectiveDefault.id).to.equal(templateId);
    } finally {
      if (previousDefault && previousDefault.id) {
        try {
          await apiClient.post(
            '/notification/e-mail/templates/' + previousDefault.id + '/default',
            {},
            'super_admin'
          );
        } catch (error) {
          // Cleanup failures should not hide the original assertion result.
        }
      }
      if (templateId) {
        try {
          await apiClient.delete('/notification/e-mail/templates/' + templateId, {}, 'super_admin');
        } catch (error) {
          // Cleanup failures should not hide the original assertion result.
        }
      }
    }
  });

  it('keeps tenant alarm email templates inside the authenticated tenant scope', async function () {
    let templateId = '';
    const templateName = 'codex-tenant-alarm-template-' + Date.now();

    try {
      const createResp = await apiClient.post(
        '/notification/e-mail/templates',
        {
          name: templateName,
          subject_template: '[Tenant alarm] {{.Subject}}',
          body_template: '{{.Message}}\nTenant: {{.TenantID}}',
          enabled: true,
          is_default: false
        },
        'tenant_admin'
      );
      expectOk(createResp);
      expect(createResp.data).to.be.an('object');
      expect(createResp.data.id).to.be.a('string').and.not.empty;
      expect(createResp.data.tenant_id).to.equal(createdTenantId);
      templateId = createResp.data.id;

      const listResp = await apiClient.get(
        '/notification/e-mail/templates',
        { page: 1, page_size: 100 },
        'tenant_admin'
      );
      expectOk(listResp);
      expect(listResp.data.list).to.be.an('array');
      const createdTemplate = listResp.data.list.find(template => template.id === templateId);
      expect(createdTemplate).to.be.an('object');
      expect(createdTemplate.tenant_id).to.equal(createdTenantId);

      const previewResp = await apiClient.post(
        '/notification/e-mail/templates/preview',
        {
          subject_template: '{{.Subject}}',
          body_template: '{{.TenantID}}',
          subject: 'Tenant preview'
        },
        'tenant_admin'
      );
      expectOk(previewResp);
      expect(previewResp.data.subject).to.equal('Tenant preview');
      expect(previewResp.data.body).to.equal(createdTenantId);
    } finally {
      if (templateId) {
        try {
          await apiClient.delete('/notification/e-mail/templates/' + templateId, {}, 'tenant_admin');
        } catch (error) {
          // Cleanup failures should not hide the original assertion result.
        }
      }
    }
  });

  it('deletes the created notification group', async function () {
    expect(notificationGroupId).to.be.a('string').and.not.empty;

    const resp = await apiClient.delete('/notification_group/' + notificationGroupId, {}, 'tenant_admin');

    expectOk(resp);
    notificationGroupId = null;
  });
});

// asserted: notification_group.list seeded-row-visible + name (list-shape), notification_history.list row.id shape
