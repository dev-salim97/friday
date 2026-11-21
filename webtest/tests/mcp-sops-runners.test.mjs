import test from 'node:test';
import assert from 'node:assert/strict';
import { loadApp, settle } from '../lib/load-app.mjs';

test('MCP import accepts common formats, validates bad input, and saves HTTP configuration', async (t) => {
  const app = await loadApp();
  t.after(app.close);
  const parsed = app.evaluate(`
    mcpImportCandidates(mcpParseJSONText(\`\\\`\\\`\\\`json
      {"mcpServers":{"docs":{"type":"streamable-http","url":"[Docs](https://mcp.example.test/api)",
      "headers":{"Authorization":"Bearer x"},"timeoutSec":90}}}
    \\\`\\\`\\\`\`))[0]
  `);
  assert.equal(parsed.id, 'docs');
  assert.equal(parsed.transport, 'http');
  assert.equal(parsed.url, 'https://mcp.example.test/api');
  assert.equal(JSON.stringify(parsed.headers), JSON.stringify({ Authorization: 'Bearer x' }));
  assert.throws(() => app.evaluate(`mcpNormalizeImportedServer({id:'bad',transport:'stdio'})`), /缺少 command/);
  assert.throws(() => app.evaluate(`mcpImportStringMap({A:{}},'headers')`), /必须是字符串/);

  app.router.post('/api/mcp-servers/custom', (call) => ({
    ...call.body.server, id: call.body.server.id, scope: call.body.scope, projectId: call.body.projectId,
  }));
  app.evaluate(`openCustomMCPModal();mcpApplyImportedServer(${JSON.stringify(parsed)})`);
  const form = app.document.querySelector('#customMcpForm');
  form.elements.scope.value = 'project';
  form.dispatchEvent(new app.window.Event('submit', { bubbles: true, cancelable: true }));
  await settle(8);
  const call = app.router.calls.find(c => c.path === '/api/mcp-servers/custom');
  assert.equal(call.method, 'POST');
  assert.equal(call.body.server.headers.Authorization, 'Bearer x');
  assert.equal(call.body.projectId, 'project-1');
});

test('generic SOP requirements reflect credentials and installed MCPs, then clone through API', async (t) => {
  const app = await loadApp();
  t.after(app.close);
  const generic = {
    id: 'generic-delivery', name: '通用交付', description: 'standard', generic: true, version: 2,
    nodes: [{ id: 'mr', name: '发起 MR', type: 'agent' }, { id: 'gate', name: '验收', type: 'approval' }],
    requirements: [
      { key: 'gongfeng-git-token', nodeId: 'mr' },
      { key: 'tapd-mcp', nodeId: 'mr' },
      { key: 'gongfeng-mcp', nodeId: 'mr' },
    ],
  };
  app.router.post('/api/workflows/generic-delivery/clone', (call) => ({
    ...generic, ...call.body, generic: false, tags: ['generic-source:generic-delivery'],
  }));
  app.evaluate(`
    state.currentUser={id:1,loginName:'owner',isPlatformAdmin:false};
    state.data.users=[{id:1,loginName:'owner',gitHttpCredentialsConfigured:true}];
    state.mcpMarket=[{id:'market-gf',name:'Gongfeng',aliases:['gongfeng'],url:'https://gongfeng'}];
    state.data.mcpServers=[{id:'gf',scope:'project',projectId:'project-1',marketId:'market-gf'}];
    state.data.workflows=[${JSON.stringify(generic)}];
    state.sopScope='generic';
    openGenericTemplateModal('generic-delivery');
  `);
  const rows = app.document.querySelector('#genericTemplateRequirements').textContent;
  assert.match(rows, /Git 访问令牌.*已配置/s);
  assert.match(rows, /TAPD MCP.*未配置/s);
  assert.match(rows, /工蜂 MCP.*已配置/s);
  const sopHTML = app.evaluate(`({
    card:genericSOPCard(state.data.workflows[0]),
    chips:genericRequirementChips(state.data.workflows[0]),
    gongfeng:gongfengMCPInstalled(),
    tapd:tapdMCPInstalled(),
    node:workflowNodeName(state.data.workflows[0],'mr')
  })`);
  assert.match(sopHTML.card, /平台维护/);
  assert.match(sopHTML.chips, /sop-badge tone-success[^>]*>.*Git 访问令牌/);
  assert.match(sopHTML.chips, /sop-badge tone-warn[^>]*>.*TAPD MCP/);
  assert.equal(sopHTML.gongfeng, true);
  assert.equal(sopHTML.tapd, false);
  assert.equal(sopHTML.node, '发起 MR');
  app.evaluate(`openGenericRequirementTarget('gongfeng-mcp')`);
  assert.equal(app.evaluate(`state.mcpQuery`), 'gongfeng');
  app.evaluate(`openGenericTemplateModal('generic-delivery')`);
  await app.evaluate(`adoptGenericTemplate()`);
  await settle(8);
  const clone = app.router.calls.find(c => c.path === '/api/workflows/generic-delivery/clone');
  assert.equal(clone.method, 'POST');
  assert.equal(clone.body.projectId, 'project-1');
  assert.equal(app.evaluate(`state.data.workflows[0].generic`), false);
});

test('generic SOP 执行 CLI drives requirements, direct runs and adoption', async (t) => {
  const app = await loadApp();
  t.after(app.close);
  const generic = {
    id: 'generic-cli', name: '通用交付', description: 'standard', generic: true, version: 1, requirements: [],
    nodes: [{ id: 'dev', name: '开发', type: 'agent', agentId: 'generic-cli-dev' }],
  };
  app.router.post('/api/workflows/generic-cli/clone', (call) => ({ ...generic, ...call.body, generic: false }));
  app.router.post('/api/runs', (call) => ({ id: 'run-cli', status: 'running', workflowId: call.body.workflowId, nodes: {} }));
  app.evaluate(`
    state.currentUser={id:1,loginName:'owner',isPlatformAdmin:false};
    state.data.users=[{id:1,loginName:'owner',codexApiKeyConfigured:true}];
    state.data.workflows=[${JSON.stringify(generic)}];
    openGenericTemplateModal('generic-cli');
  `);
  const select = app.document.querySelector('#genericTemplateExecutor');
  const requirements = () => app.document.querySelector('#genericTemplateRequirements').textContent;
  assert.equal(select.value, '');
  assert.match(select.options[0].textContent, /按角色配置（Claude Code）/);
  assert.match(requirements(), /Claude Code API Key.*未配置/s);
  select.value = 'codex';
  select.dispatchEvent(new app.window.Event('change', { bubbles: true }));
  assert.match(requirements(), /Codex API Key.*已配置/s);
  assert.doesNotMatch(requirements(), /Claude Code/);

  app.document.querySelector('#runGenericTemplateDirect').click();
  const runChoice = app.document.querySelector('#runExecutorChoice');
  assert.equal(runChoice.value, 'codex');
  assert.match(runChoice.options[0].textContent, /按 SOP 配置（Claude Code）/);
  await app.evaluate(`startRun()`);
  await settle(8);
  const run = app.router.calls.find(c => c.path === '/api/runs' && c.method === 'POST');
  assert.equal(run.body.workflowId, 'generic-cli');
  assert.equal(run.body.executorId, 'codex');

  app.evaluate(`openGenericTemplateModal('generic-cli')`);
  assert.equal(select.value, '');
  select.value = 'claude';
  select.dispatchEvent(new app.window.Event('change', { bubbles: true }));
  await app.evaluate(`adoptGenericTemplate()`);
  await settle(8);
  const clone = app.router.calls.find(c => c.path === '/api/workflows/generic-cli/clone');
  assert.equal(clone.body.executorId, 'claude');
  assert.equal(app.evaluate(`state.workflow.executorId`), 'claude');
  assert.equal(app.document.querySelector('#workflowExecutor').value, 'claude');
  assert.match(app.evaluate(`sopRunRequirements(state.workflow).map(r=>r.label).join(',')`), /Claude Code API Key/);
});

test('runner cards expose runtime controls and persist limits, defaults, and auth mode', async (t) => {
  const app = await loadApp();
  t.after(app.close);
  const runner = {
    id: 'runner-local', name: 'Laptop', ownerUserId: 1, status: 'online', os: 'darwin', arch: 'arm64',
    runLimit: 2, lastSeenAt: new Date().toISOString(),
    runtimes: [{ provider: 'codex', status: 'online', authMode: 'local', version: '1.0' }],
  };
  app.router.put('/api/runners/runner-local/run-limit', { ...runner, runLimit: 5 });
  app.router.put('/api/runners/default', { message: 'default updated' });
  app.router.put('/api/runners/runtimes', { ok: true });
  app.router.delete('/api/runners/runner-local', { ok: true });
  app.router.post('/api/runners/bind-codes', { code: 'BIND123', expiresAt: '2099-01-01T00:00:00Z' });
  app.router.get('/api/runners', [runner]);
  app.router.get('/api/runners/default', { runnerId: '', cloudPool: { configured: true, online: 2 } });
  app.evaluate(`
    state.currentUser={id:1,loginName:'owner'};
    state.data.users=[{id:1,loginName:'owner',codexApiKeyConfigured:true}];
    state.runners=[${JSON.stringify(runner)}];state.defaultRunnerId='';state.cloudPool={configured:true,online:2};
    renderRunnersView();
  `);
  const list = app.document.querySelector('#runnersList');
  assert.match(list.textContent, /Laptop/);
  const limit = list.querySelector('[data-runner-run-limit]');
  assert.equal(limit.value, '2');
  assert.match(app.evaluate(`runnerConcurrencyLabel(state.runners[0])`), /最多同时运行 2 条 SOP/);
  limit.value = '5';
  limit.dispatchEvent(new app.window.Event('change', { bubbles: true }));
  await settle(6);
  assert.deepEqual(app.router.calls.find(c => c.path.endsWith('/run-limit')).body, { runLimit: 5 });
  await app.evaluate(`setDefaultRunner('runner-local');setRunnerRuntimeAuth('runner-local','codex','personal')`);
  await settle(6);
  assert.ok(app.router.calls.some(c => c.path === '/api/runners/default' && c.body.runnerId === 'runner-local'));
  assert.ok(app.router.calls.some(c => c.path === '/api/runners/runtimes' && c.body.authMode === 'personal'));
  await app.evaluate(`toggleRunnerRuntime('runner-local','codex',false)`);
  assert.ok(app.router.calls.some(c => c.path === '/api/runners/runtimes' && c.body.status === 'disabled'));
  app.evaluate(`window.__removeRunner=removeRunner('runner-local')`);
  await settle(2);
  app.document.querySelector('#confirmOkBtn').click();
  await app.evaluate(`window.__removeRunner`);
  assert.ok(app.router.calls.some(c => c.method === 'DELETE' && c.path === '/api/runners/runner-local'));
});
