#!/usr/bin/env node
/**
 * 文件用途：P0.4「服务重启后调度不丢任务」重启演练的两阶段脚本。
 *
 * 为什么存在：路线图 P0.4 已 done，但「重启不丢调度」此前只有结构性保证
 * （定时器落库 91.sql + 启动 cron 重载），没有专门的重启演练。本脚本把
 * 演练拆成两个可由外部执行重启的子命令：
 *
 *   arm    创建一个**一次性定时触发的自动化**（execution_time = now + FIRE_IN 秒，
 *          动作 30 触发种子告警配置），把 automation id / 触发时刻 / 创建时刻
 *          写入状态文件。触发时刻刻意设在「重启完成之后」——重启窗口吞掉的
 *          正是这次触发，若调度丢失则它永远不会执行。
 *
 *   verify 读状态文件，轮询 /scene_automations/log 直到出现 executed_at
 *          **晚于重启完成时刻**的执行记录，断言调度在重启后照常触发。
 *
 * 之间由操作者重启后端（本机由 agent 通过后台任务重启）。
 */

const fs = require('fs');
const path = require('path');

// 自动载入 .env.local，避免由于环境变量未导出导致的邮箱密码缺失
const envLocalPath = path.resolve(__dirname, '../.env.local');
if (fs.existsSync(envLocalPath)) {
  const content = fs.readFileSync(envLocalPath, 'utf8');
  for (const line of content.split('\n')) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith('#')) continue;
    const eqIdx = trimmed.indexOf('=');
    if (eqIdx > 0) {
      const k = trimmed.slice(0, eqIdx).trim();
      const v = trimmed.slice(eqIdx + 1).trim();
      if (!process.env[k]) process.env[k] = v;
    }
  }
}

const apiClient = require('../lib/api_client');
const seedData = require('../lib/seed_data');

function parseArgs(argv) {
  // argv = process.argv.slice(2)：首个元素是子命令，其余是 --key value 对。
  const args = { _: [] };
  for (let i = 0; i < argv.length; i += 1) {
    if (argv[i].startsWith('--')) {
      args[argv[i].replace(/^--/, '')] = argv[i + 1];
      i += 1;
    } else {
      args._.push(argv[i]);
    }
  }
  return args;
}

async function main() {
  const args = parseArgs(process.argv.slice(2));
  const command = args._[0];
  const statePath = args['state-file'];
  if (!statePath || (command !== 'arm' && command !== 'verify')) {
    console.error('usage: arm --state-file <f> [--fire-in S] | verify --state-file <f> [--restart-at ISO]');
    process.exit(2);
  }

  const healthy = await apiClient.healthCheck();
  if (!healthy) {
    console.error('backend unhealthy');
    process.exit(1);
  }
  await apiClient.login('tenant_admin');

  if (command === 'arm') {
    const fireInSec = Number(args['fire-in'] || 120);
    const sceneSeed = await seedData.ensureScene('tenant_admin');
    if (sceneSeed.blocked) throw new Error('scene fixture unavailable: ' + sceneSeed.reason);

    const fireAt = new Date(Date.now() + fireInSec * 1000);
    const created = await apiClient.post('/scene_automations', {
      name: ('restart_drill_' + Date.now()).slice(0, 36),
      description: 'P0.4 restart persistence drill',
      enabled: 'Y',
      trigger_condition_groups: [[{
        trigger_conditions_type: '20',
        execution_time: fireAt.toISOString(),
        expiration_time: 10
      }]],
      actions: [{
        action_type: '20',
        action_target: sceneSeed.id,
        remark: 'restart drill: trigger scene after backend restart'
      }],
      remark: 'P0.4 restart persistence drill'
    }, 'tenant_admin');
    if (created.code !== 200) {
      throw new Error('create drill automation failed: ' + created.message);
    }
    const automationId = created.data && (created.data.scene_automation_id || created.data.id);
    const state = {
      automation_id: automationId,
      scene_id: sceneSeed.id,
      fire_at: fireAt.toISOString(),
      armed_at: new Date().toISOString()
    };
    fs.writeFileSync(statePath, JSON.stringify(state, null, 2));
    console.log('ARMED automation_id=' + automationId + ' fire_at=' + fireAt.toISOString()
      + ' -> ' + statePath);
    return;
  }

  const state = JSON.parse(fs.readFileSync(statePath, 'utf8'));
  const restartAt = args['restart-at'] ? new Date(args['restart-at']) : new Date(0);
  console.log('VERIFY automation_id=' + state.automation_id
    + ' restart_at=' + restartAt.toISOString());

  const deadline = Date.now() + 150000;
  let fired = null;
  while (Date.now() < deadline) {
    const response = await apiClient.get('/scene_automations/log', {
      scene_automation_id: state.automation_id,
      page: 1,
      page_size: 100
    }, 'tenant_admin');
    if (response.code === 200) {
      const rows = Array.isArray(response.data) ? response.data : (response.data && response.data.list) || [];
      const afterRestart = rows.find(row => row.executed_at && new Date(row.executed_at) > restartAt);
      if (afterRestart) { fired = afterRestart; break; }
    }
    await new Promise(resolve => setTimeout(resolve, 3000));
  }
  if (!fired) {
    console.error('VERDICT=FAIL no automation execution recorded after the restart');
    process.exit(1);
  }
  console.log('FIRED executed_at=' + fired.executed_at);
  console.log('VERDICT=PASS scheduled task survived the backend restart');
}

main().catch(err => {
  console.error(err.message || err);
  process.exit(1);
});
