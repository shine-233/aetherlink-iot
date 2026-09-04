/**
 * Real OTA API -> MQTT -> task-detail persistence coverage.
 *
 * The fixture creates the package/task through public APIs, starts an
 * authenticated device before task creation, and requires both the device's
 * OTA inform/progress receipts and the backend-visible terminal detail row.
 * A second case exercises a device-reported failure and the support-bundle
 * readback so a broker publish alone cannot make this suite green.
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const seedData = require('../lib/seed_data');
const { validateOTAProgressReceipts } = require('../lib/mqtt_device_fixture');
const { skipIfBlocked } = require('../lib/integration_blocked');

function detailStatus(row) {
  return Number(row && (row.status ?? row.Status ?? row.task_status ?? row.TaskStatus));
}

function detailProgress(row) {
  return Number(row && (row.steps ?? row.step ?? row.Step ?? row.progress));
}

function detailDescription(row) {
  return String(row && (
    row.status_description ?? row.StatusDescription ?? row.description ?? ''
  ) || '');
}

function detailForDevice(rows, deviceId) {
  return (Array.isArray(rows) ? rows : []).find(row => row && (
    String(row.device_id || row.DeviceID || row.deviceId || '') === String(deviceId)
  ));
}

function currentVersion(row) {
  return String(row && (
    row.current_version ?? row.currentVersion ?? row.CurrentVersion ?? ''
  ) || '').trim();
}

async function readTaskDetail(taskId, accountKey = 'tenant_admin') {
  const response = await apiClient.get('/ota/task/detail', {
    page: 1,
    page_size: 100,
    ota_upgrade_task_id: taskId
  }, accountKey);
  if (!response || response.code !== 200) {
    throw new Error('read OTA task detail failed: ' + JSON.stringify(response));
  }
  return Array.isArray(response.data)
    ? response.data
    : (response.data && Array.isArray(response.data.list) ? response.data.list : []);
}

async function waitForTaskDetail(taskId, deviceId, predicate, timeoutMs = 60000) {
  const deadline = Date.now() + timeoutMs;
  let latest = [];
  while (Date.now() < deadline) {
    latest = await readTaskDetail(taskId);
    const row = detailForDevice(latest, deviceId);
    if (row && predicate(row)) return row;
    await new Promise(resolve => setTimeout(resolve, 500));
  }
  throw new Error(
    `OTA task ${taskId} did not reach the expected detail state; latest=${JSON.stringify(latest)}`
  );
}

async function waitForDeviceVersion(deviceId, expectedVersion, timeoutMs = 30000) {
  const deadline = Date.now() + timeoutMs;
  let latest = null;
  while (Date.now() < deadline) {
    const response = await apiClient.get('/device/detail/' + deviceId, {}, 'tenant_admin');
    latest = response && response.code === 200 ? response.data : null;
    if (currentVersion(latest) === expectedVersion) return latest;
    await new Promise(resolve => setTimeout(resolve, 500));
  }
  throw new Error(
    `device ${deviceId} current_version did not become ${expectedVersion}; latest=${JSON.stringify(latest)}`
  );
}

function packageVersion(packageSeed) {
  return String(packageSeed && packageSeed.row && (
    packageSeed.row.target_version ||
    packageSeed.row.targetVersion ||
    packageSeed.row.version ||
    packageSeed.row.Version ||
    ''
  ) || '').trim();
}

describe('OTA public API and authenticated MQTT runtime [32_ota_runtime]', function () {
  this.timeout(180000);

  before(async function () {
    if (!(await seedData.isMqttBrokerAvailable())) {
      skipIfBlocked(this, {
        reason: 'MQTT broker is not available on ' + seedData.mqttEndpointDescription() +
          '; OTA inform/progress runtime evidence requires a live broker',
        category: 'runtime-external',
        seedable: false
      });
    }
    await apiClient.login('tenant_admin');
  });

  after(function () {
    apiClient.clearAllTokens();
  });

  it('creates a task through the public API and persists a successful device-reported OTA rollout', async function () {
    const packageSeed = await seedData.createOtaPackageSeed('tenant_admin');
    let taskSeed = null;
    const expectedVersion = packageVersion(packageSeed);
    expect(expectedVersion, 'OTA package must expose a version').to.not.equal('');
    try {
      taskSeed = await seedData.createOtaTaskApiSeed(
        packageSeed.id,
        packageSeed.row,
        'tenant_admin',
        packageSeed,
        {
          startMqttDevice: true,
          mqttOptions: {
            version: expectedVersion,
            progressValues: [0, 10, 50, 100]
          }
        }
      );

      const receipts = validateOTAProgressReceipts(
        await taskSeed.mqttDevice.waitForOTAProgress(),
        [0, 10, 50, 100]
      );
      expect(receipts.filter(row => row.kind === 'inform')).to.have.length.at.least(1);

      const detail = await waitForTaskDetail(
        taskSeed.taskId,
        taskSeed.device.id,
        row => detailStatus(row) === 4 && detailProgress(row) >= 100
      );
      expect(detailStatus(detail)).to.equal(4);
      expect(detailProgress(detail)).to.be.at.least(100);
      expect(detailDescription(detail)).to.match(/succeed|success|completed/i);

      const device = await waitForDeviceVersion(taskSeed.device.id, expectedVersion);
      expect(currentVersion(device)).to.equal(expectedVersion);
    } finally {
      if (taskSeed) {
        await taskSeed.cleanup();
      } else {
        await packageSeed.cleanup();
      }
    }
  });

  it('persists a device-reported OTA failure and exposes it through the support bundle', async function () {
    const packageSeed = await seedData.createOtaPackageSeed('tenant_admin');
    let taskSeed = null;
    const expectedVersion = packageVersion(packageSeed);
    expect(expectedVersion, 'OTA package must expose a version').to.not.equal('');
    try {
      taskSeed = await seedData.createOtaTaskApiSeed(
        packageSeed.id,
        packageSeed.row,
        'tenant_admin',
        packageSeed,
        {
          startMqttDevice: true,
          mqttOptions: {
            version: expectedVersion,
            progressValues: [0, 25, 50],
            failure: true
          }
        }
      );

      validateOTAProgressReceipts(
        await taskSeed.mqttDevice.waitForOTAProgress(),
        [0, 25, 50]
      );

      const detail = await waitForTaskDetail(
        taskSeed.taskId,
        taskSeed.device.id,
        row => detailStatus(row) === 5
      );
      expect(detailStatus(detail)).to.equal(5);
      expect(detailProgress(detail)).to.equal(50);
      expect(detailDescription(detail)).to.match(/fail|error/i);

      const deviceResp = await apiClient.get('/device/detail/' + taskSeed.device.id, {}, 'tenant_admin');
      expect(deviceResp.code).to.equal(200);
      expect(currentVersion(deviceResp.data)).to.not.equal(expectedVersion);

      const supportResp = await apiClient.get(
        '/ota/task/' + taskSeed.taskId + '/support-bundle',
        {},
        'tenant_admin'
      );
      expect(supportResp.code).to.equal(200);
      expect(supportResp.data.failed_count).to.equal(1);
      expect(supportResp.data.failed_devices).to.have.length(1);
      const detailId = String(detail.id || detail.ID || '').trim();
      expect(detailId).to.not.equal('');
      expect(supportResp.data.failed_devices[0]).to.include({
        device_id: taskSeed.device.id,
        detail_id: detailId
      });
      expect(supportResp.data.failed_devices[0].failure_reason).to.match(/fail|error/i);
    } finally {
      if (taskSeed) {
        await taskSeed.cleanup();
      } else {
        await packageSeed.cleanup();
      }
    }
  });
});
