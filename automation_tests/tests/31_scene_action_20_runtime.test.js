/**
 * 文件用途：验证场景自动化 action 20 的真实 API、MQTT 在线触发和数据库日志闭环。
 * 核心逻辑：通过 API 创建 action 20 自动化和普通目标场景，使用真实认证设备
 * MQTT online 状态触发，回读自动化日志、目标场景日志和告警历史，并核对 PostgreSQL。
 * 关键注意事项：仅 MQTT publish/online 成功不算执行成功；必须看到 S 日志和目标副作用。
 * 重构建议：若后续提供受控审计日志清理 API，可替换当前严格隔离库的清理 SQL。
 */

const { expect } = require('chai');
const fs = require('fs');
const { execFileSync } = require('child_process');

const apiClient = require('../lib/api_client');
const {
  createSimulationDevice,
  ensureScene,
  isMqttBrokerAvailable,
  listFromResponse,
  pickAlarmHistoryId,
  pickId,
  requireSuccess,
  resolveOtaSeedDatabaseOptions
} = require('../lib/seed_data');
const { startMqttCommandDevice } = require('../lib/mqtt_device_fixture');
const { skipIfBlocked } = require('../lib/integration_blocked');

const UUID_RE = /^[0-9a-f-]{36}$/i;

function strictDatabaseOptions() {
  if (String(process.env.AETHERLINK_STRICT_DB_TARGET || '').trim() !== '1') {
    throw new Error('AETHERLINK_STRICT_DB_TARGET=1 is required for runtime scene action 20 evidence');
  }

  const psqlPath = String(process.env.AETHERLINK_PSQL_PATH || '').trim();
  if (!psqlPath || !fs.existsSync(psqlPath)) {
    throw new Error('AETHERLINK_PSQL_PATH must point to the psql executable for runtime scene action 20 evidence');
  }

  return {
    psqlPath,
    database: resolveOtaSeedDatabaseOptions(process.env)
  };
}

function assertUuid(value, label) {
  expect(String(value), label).to.match(UUID_RE);
  return String(value);
}

function runStrictPsql(sql) {
  const { psqlPath, database } = strictDatabaseOptions();
  const psqlEnv = { ...process.env };
  if (database.password) psqlEnv.PGPASSWORD = database.password;

  return execFileSync(psqlPath, [
    '-X',
    '-w',
    '-v', 'ON_ERROR_STOP=1',
    '-At',
    '-F', '|',
    '-h', String(database.host),
    '-p', String(database.port),
    '-U', String(database.user),
    '-d', String(database.database),
    '-c', sql
  ], {
    encoding: 'utf8',
    env: psqlEnv,
    timeout: 15000,
    windowsHide: true
  }).trim();
}

function queryAutomationLogsFromDatabase(sceneAutomationId) {
  const id = assertUuid(sceneAutomationId, 'scene automation id');
  return runStrictPsql([
    'SELECT scene_automation_id, execution_result, detail',
    'FROM scene_automation_log',
    `WHERE scene_automation_id = '${id}'`,
    'ORDER BY executed_at DESC;'
  ].join(' '));
}

function querySceneLogsFromDatabase(sceneId) {
  const id = assertUuid(sceneId, 'scene id');
  return runStrictPsql([
    'SELECT scene_id, execution_result, detail',
    'FROM scene_log',
    `WHERE scene_id = '${id}'`,
    'ORDER BY executed_at DESC;'
  ].join(' '));
}

function deleteAutomationLogsFromStrictDatabase(sceneAutomationId) {
  const id = assertUuid(sceneAutomationId, 'scene automation id');
  // scene_automation_log intentionally uses ON DELETE RESTRICT to retain the
  // audit trail. This DELETE is limited to the newly-created UUID and is only
  // allowed after the caller opted into the explicitly isolated DB target.
  return runStrictPsql([
    'DELETE FROM scene_automation_log',
    `WHERE scene_automation_id = '${id}';`,
    'SELECT COUNT(*) FROM scene_automation_log',
    `WHERE scene_automation_id = '${id}';`
  ].join(' '));
}

function deleteAlarmHistoryFromStrictDatabase(historyId) {
  const id = assertUuid(historyId, 'alarm history id');
  if (String(process.env.AETHERLINK_STRICT_DB_CLEANUP || '').trim() !== '1') {
    throw new Error(
      'AETHERLINK_STRICT_DB_CLEANUP=1 is required before deleting retained alarm history from the isolated database'
    );
  }

  // The product intentionally keeps alarm history for audit and rejects the
  // public DELETE route.  This exact UUID cleanup is allowed only in the
  // caller-selected strict isolated database, never in a shared deployment.
  return runStrictPsql([
    'DELETE FROM alarm_history',
    `WHERE id = '${id}';`,
    'SELECT COUNT(*) FROM alarm_history',
    `WHERE id = '${id}';`
  ].join(' '));
}

async function waitForValue(label, load, predicate, timeoutMs = 45000) {
  const deadline = Date.now() + timeoutMs;
  let lastValue;
  while (Date.now() < deadline) {
    lastValue = await load();
    if (predicate(lastValue)) return lastValue;
    await new Promise(resolve => setTimeout(resolve, 500));
  }
  throw new Error(`${label} did not reach the expected state within ${timeoutMs}ms: ${JSON.stringify(lastValue)}`);
}

function executionResult(row) {
  return row && (row.execution_result || row.ExecutionResult);
}

function rowDetail(row) {
  return String(row && (row.detail || row.Detail || '') || '');
}

function matchingAlarmHistory(row, sceneSeed) {
  const alarmConfigId = row && (row.alarm_config_id || row.AlarmConfigID);
  const sceneId = row && (row.scene_automation_id || row.SceneAutomationID || row.scene_id || row.SceneID);
  return String(alarmConfigId || '') === String(sceneSeed.alarmConfigId) &&
    String(sceneId || '') === String(sceneSeed.id);
}

async function readAutomationLogs(sceneAutomationId) {
  const response = await apiClient.get('/scene_automations/log', {
    scene_automation_id: sceneAutomationId,
    page: 1,
    page_size: 100
  }, 'tenant_admin');
  requireSuccess(response, 'read action 20 automation execution log');
  return listFromResponse(response);
}

async function readSceneLogs(sceneId) {
  const response = await apiClient.get('/scene/log', {
    id: sceneId,
    page: 1,
    page_size: 100
  }, 'tenant_admin');
  requireSuccess(response, 'read nested scene execution log');
  return listFromResponse(response);
}

async function readAlarmHistory() {
  const response = await apiClient.get('/alarm/info/history', {
    page: 1,
    page_size: 100
  }, 'tenant_admin');
  requireSuccess(response, 'read nested scene alarm history');
  return listFromResponse(response);
}

async function cleanupMatchingAlarmHistory(sceneSeed) {
  const rows = await readAlarmHistory();
  const matching = rows.filter(row => matchingAlarmHistory(row, sceneSeed));
  for (const row of matching) {
    const id = pickAlarmHistoryId(row);
    if (!id) continue;
    const response = await apiClient.delete('/alarm/info/history/' + id, {}, 'tenant_admin');
    if (response && response.code === 201002) {
      deleteAlarmHistoryFromStrictDatabase(id);
      continue;
    }
    if (response && response.code !== 200 && response.code !== 100404) {
      throw new Error(`delete action 20 alarm history ${id} failed: ${JSON.stringify(response)}`);
    }
  }
  return matching;
}

describe('Scene action 20 real MQTT runtime [31_scene_action_20_runtime]', function () {
  this.timeout(180000);

  before(async function () {
    if (!(await apiClient.healthCheck())) {
      skipIfBlocked(this, {
        reason: 'backend service is unavailable; scene action 20 runtime evidence requires a live API service',
        category: 'runtime-external',
        seedable: false
      });
    }
    if (!(await isMqttBrokerAvailable())) {
      skipIfBlocked(this, {
        reason: 'MQTT broker is unavailable; scene action 20 runtime evidence requires a live authenticated broker',
        category: 'runtime-external',
        seedable: false
      });
    }
    try {
      strictDatabaseOptions();
    } catch (error) {
      skipIfBlocked(this, {
        reason: error.message,
        category: 'runtime-external',
        seedable: false
      });
    }
    await apiClient.login('tenant_admin');
  });

  after(function () {
    apiClient.clearAllTokens();
  });

  it('executes scene action 20 from a real MQTT online transition and exposes automation/scene/alarm logs', async function () {
    const deviceSeed = await createSimulationDevice('tenant_admin');
    const sceneSeed = await ensureScene('tenant_admin');
    let sceneAutomationId = '';
    let mqttDevice;
    let cleanupError;

    try {
      expect(sceneSeed.blocked, sceneSeed.reason || 'nested scene fixture must be available').to.equal(false);

      const name = `runtime_scene_action_20_${Date.now()}`.slice(0, 36);
      const createResponse = await apiClient.post('/scene_automations', {
        name,
        description: 'real MQTT action 20 runtime fixture',
        enabled: 'Y',
        trigger_condition_groups: [[{
          trigger_conditions_type: '10',
          trigger_source: deviceSeed.id,
          trigger_param_type: 'STATUS',
          trigger_param: 'On-line',
          trigger_operator: '=',
          trigger_value: 'online'
        }]],
        actions: [{
          action_type: '20',
          action_target: sceneSeed.id,
          remark: 'activate nested scene through online status'
        }],
        remark: 'strict action 20 runtime fixture'
      }, 'tenant_admin');
      requireSuccess(createResponse, 'create action 20 runtime automation');
      sceneAutomationId = assertUuid(pickId(createResponse.data), 'created action 20 automation id');

      const detailResponse = await apiClient.get('/scene_automations/detail/' + sceneAutomationId, {}, 'tenant_admin');
      requireSuccess(detailResponse, 'read action 20 runtime automation detail');
      expect(detailResponse.data).to.include({
        id: sceneAutomationId,
        name,
        enabled: 'Y'
      });
      expect(detailResponse.data.actions).to.deep.include({
        action_type: '20',
        action_target: sceneSeed.id,
        action_param_type: '',
        action_param: '',
        action_value: ''
      });
      expect(detailResponse.data.trigger_condition_groups[0][0]).to.include({
        trigger_param_type: 'STATUS',
        trigger_param: 'On-line',
        trigger_value: 'online'
      });

      expect(await readAutomationLogs(sceneAutomationId), 'no execution log before MQTT online transition').to.deep.equal([]);

      // Creation refreshes the enabled cache asynchronously. Give that cache
      // a bounded settling window before publishing the real status transition.
      await new Promise(resolve => setTimeout(resolve, 1500));
      mqttDevice = await startMqttCommandDevice(deviceSeed, 'tenant_admin');

      const automationRows = await waitForValue(
        'action 20 automation execution log',
        () => readAutomationLogs(sceneAutomationId),
        rows => rows.some(row => executionResult(row) === 'S'),
        45000
      );
      const automationSuccess = automationRows.find(row => executionResult(row) === 'S');
      expect(automationSuccess).to.be.an('object');
      expect(rowDetail(automationSuccess)).to.include(`场景激活:${sceneSeed.name}`);

      const databaseAutomationLogs = queryAutomationLogsFromDatabase(sceneAutomationId)
        .split(/\r?\n/)
        .filter(Boolean);
      expect(databaseAutomationLogs.some(line => line.startsWith(`${sceneAutomationId}|S|`)),
        'PostgreSQL scene_automation_log must contain a successful action 20 execution').to.equal(true);

      const sceneRows = await waitForValue(
        'nested scene execution log',
        () => readSceneLogs(sceneSeed.id),
        rows => rows.some(row => executionResult(row) === 'S'),
        45000
      );
      const sceneSuccess = sceneRows.find(row => executionResult(row) === 'S');
      expect(sceneSuccess).to.be.an('object');
      expect(rowDetail(sceneSuccess)).to.include('manual scene alarm');

      const databaseSceneLogs = querySceneLogsFromDatabase(sceneSeed.id)
        .split(/\r?\n/)
        .filter(Boolean);
      expect(databaseSceneLogs.some(line => line.startsWith(`${sceneSeed.id}|S|`)),
        'PostgreSQL scene_log must contain the nested scene success').to.equal(true);

      const alarmRows = await waitForValue(
        'nested scene alarm history',
        readAlarmHistory,
        rows => rows.some(row => matchingAlarmHistory(row, sceneSeed)),
        45000
      );
      const alarmSuccess = alarmRows.find(row => matchingAlarmHistory(row, sceneSeed));
      expect(alarmSuccess).to.be.an('object');
      expect(alarmSuccess.alarm_status).to.equal('H');
      expect(String(alarmSuccess.content || '')).to.include('manual scene triggered alarm');
    } finally {
      try {
        if (mqttDevice) await mqttDevice.cleanup();
        await cleanupMatchingAlarmHistory(sceneSeed);
        await sceneSeed.cleanup();
        if (sceneAutomationId) {
          deleteAutomationLogsFromStrictDatabase(sceneAutomationId);
          const deleteResponse = await apiClient.delete('/scene_automations/' + sceneAutomationId, {}, 'tenant_admin');
          if (deleteResponse && deleteResponse.code !== 200 && deleteResponse.code !== 100404) {
            throw new Error(`delete action 20 runtime automation failed: ${JSON.stringify(deleteResponse)}`);
          }
        }
        await deviceSeed.cleanup();
      } catch (error) {
        cleanupError = error;
      }
    }

    if (cleanupError) throw cleanupError;
  });
});
