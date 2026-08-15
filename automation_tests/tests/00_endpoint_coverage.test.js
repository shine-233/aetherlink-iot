/**
 * 文件用途：用于验证端点覆盖率采集契约测试。
 * 核心逻辑：以快速 Node 测试保护覆盖率契约、运行配置、oracle 或预检逻辑的结构和边界行为。
 * 关键注意事项：这类测试证明自动化框架契约，不等同于真实后端或浏览器业务流程通过。
 * 重构建议：当契约 schema 或分类规则变化时，应同步更新 fixture 和负向用例，避免只改快照。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const endpointCoverage = require('../lib/endpoint_coverage');
const endpointCatalog = require('../lib/endpoint-coverage/catalog');

describe('Endpoint coverage matcher [00_endpoint_coverage]', function () {
  it('reuses the complete endpoint catalog by reference', function () {
    expect(endpointCoverage.ALL_ENDPOINTS).to.equal(endpointCatalog.ALL_ENDPOINTS);
    expect(endpointCoverage.ALL_ENDPOINTS.length).to.be.greaterThan(0);
    for (const endpoint of endpointCoverage.ALL_ENDPOINTS) {
      for (const property of ['method', 'path', 'module', 'auth']) {
        expect(Object.prototype.hasOwnProperty.call(endpoint, property)).to.equal(true);
      }
    }
  });

  it('prefers static board routes over dynamic board detail routes', function () {
    const matched = endpointCoverage.findEndpoint('GET', '/api/v1/board/home');
    expect(matched).to.be.an('object');
    expect(matched.path).to.equal('/api/v1/board/home');
  });

  it('prefers static device metrics routes over parameterized metrics detail routes', function () {
    const menuMatch = endpointCoverage.findEndpoint('GET', '/api/v1/device/metrics/menu');
    expect(menuMatch).to.be.an('object');
    expect(menuMatch.path).to.equal('/api/v1/device/metrics/menu');

    const chartMatch = endpointCoverage.findEndpoint('GET', '/api/v1/device/metrics/chart');
    expect(chartMatch).to.be.an('object');
    expect(chartMatch.path).to.equal('/api/v1/device/metrics/chart');
  });

  it('prefers static current-key and config routes over generic id routes', function () {
    const telemetryMatch = endpointCoverage.findEndpoint('GET', '/api/v1/telemetry/datas/current/keys');
    expect(telemetryMatch).to.be.an('object');
    expect(telemetryMatch.path).to.equal('/api/v1/telemetry/datas/current/keys');

    const configMatch = endpointCoverage.findEndpoint('GET', '/api/v1/device_config/connect');
    expect(configMatch).to.be.an('object');
    expect(configMatch.path).to.equal('/api/v1/device_config/connect');
  });

  it('matches telemetry dead-letter operator routes', function () {
    const listMatch = endpointCoverage.findEndpoint(
      'GET',
      '/api/v1/telemetry/datas/dead-letters',
    );
    expect(listMatch).to.be.an('object');
    expect(listMatch.path).to.equal('/api/v1/telemetry/datas/dead-letters');

    const drainMatch = endpointCoverage.findEndpoint(
      'POST',
      '/api/v1/telemetry/datas/dead-letters/drain',
    );
    expect(drainMatch).to.be.an('object');
    expect(drainMatch.path).to.equal('/api/v1/telemetry/datas/dead-letters/drain');

    const statusMatch = endpointCoverage.findEndpoint(
      'PATCH',
      '/api/v1/telemetry/datas/dead-letters/dead-letter-1/status',
    );
    expect(statusMatch).to.be.an('object');
    expect(statusMatch.path).to.equal('/api/v1/telemetry/datas/dead-letters/:id/status');

    const uplinkListMatch = endpointCoverage.findEndpoint(
      'GET',
      '/api/v1/telemetry/datas/uplink-dead-letters',
    );
    expect(uplinkListMatch).to.be.an('object');
    expect(uplinkListMatch.path).to.equal('/api/v1/telemetry/datas/uplink-dead-letters');

    const uplinkDrainMatch = endpointCoverage.findEndpoint(
      'POST',
      '/api/v1/telemetry/datas/uplink-dead-letters/drain',
    );
    expect(uplinkDrainMatch).to.be.an('object');
    expect(uplinkDrainMatch.path).to.equal('/api/v1/telemetry/datas/uplink-dead-letters/drain');

    const uplinkStatusMatch = endpointCoverage.findEndpoint(
      'PATCH',
      '/api/v1/telemetry/datas/uplink-dead-letters/dead-letter-1/status',
    );
    expect(uplinkStatusMatch).to.be.an('object');
    expect(uplinkStatusMatch.path).to.equal('/api/v1/telemetry/datas/uplink-dead-letters/:id/status');
  });

  it('matches device onboarding connection guide before broader dynamic device routes', function () {
    const guideMatch = endpointCoverage.findEndpoint('GET', '/api/v1/device/dev-1/onboarding/connection-guide');
    expect(guideMatch).to.be.an('object');
    expect(guideMatch.path).to.equal('/api/v1/device/:device_id/onboarding/connection-guide');
  });

  it('matches OTA support-bundle routes before broader task id routes', function () {
    const supportBundleMatch = endpointCoverage.findEndpoint('GET', '/api/v1/ota/task/task-1/support-bundle');
    expect(supportBundleMatch).to.be.an('object');
    expect(supportBundleMatch.path).to.equal('/api/v1/ota/task/:id/support-bundle');
  });

  it('matches the monthly alarm history route before the generic history id route', function () {
    const monthlyMatch = endpointCoverage.findEndpoint(
      'GET',
      '/api/v1/alarm/info/history/monthly',
    );
    expect(monthlyMatch).to.be.an('object');
    expect(monthlyMatch.path).to.equal('/api/v1/alarm/info/history/monthly');
  });

  it('matches command job support-bundle routes before generic command data routes', function () {
    const supportBundleMatch = endpointCoverage.findEndpoint(
      'GET',
      '/api/v1/command/datas/jobs/job-1/support-bundle',
    );
    expect(supportBundleMatch).to.be.an('object');
    expect(supportBundleMatch.path).to.equal('/api/v1/command/datas/jobs/:job_id/support-bundle');
  });

  it('keeps device-model command list route available for Ready Check command drafts', function () {
    const commandListMatch = endpointCoverage.findEndpoint('GET', '/api/v1/command/datas/device-1');
    expect(commandListMatch).to.be.an('object');
    expect(commandListMatch.path).to.equal('/api/v1/command/datas/:id');
  });

  it('counts only requests that receive an HTTP response as endpoint coverage', async function () {
    const originalAuthHeaders = apiClient.authHeaders;
    const originalGet = apiClient.client.get;

    try {
      apiClient.authHeaders = async () => ({ 'x-token': 'fixture' });
      endpointCoverage.reset();
      for (const [code, message] of [
        ['ECONNABORTED', 'fixture timeout'],
        ['ECONNREFUSED', 'fixture refused']
      ]) {
        apiClient.client.get = async () => {
          const error = new Error(message);
          error.code = code;
          throw error;
        };

        const transportFailure = await apiClient.get('/device');
        expect(transportFailure).to.include({ code: -1, _requestError: true });
        expect(endpointCoverage.getStats().coveredList.some(endpoint => (
          endpoint.method === 'GET' && endpoint.path === '/api/v1/device'
        ))).to.equal(false);
      }

      apiClient.client.get = async () => {
        const error = new Error('fixture unavailable');
        error.response = { status: 503, data: { message: 'fixture unavailable' } };
        throw error;
      };

      const httpResponse = await apiClient.get('/device');
      expect(httpResponse).to.include({ code: 503, _requestError: true });
      expect(endpointCoverage.getStats().coveredList).to.deep.include({
        method: 'GET',
        path: '/api/v1/device',
        module: 'device',
        auth: true,
        hitCount: 1
      });
    } finally {
      apiClient.authHeaders = originalAuthHeaders;
      apiClient.client.get = originalGet;
      endpointCoverage.reset();
    }
  });
});
