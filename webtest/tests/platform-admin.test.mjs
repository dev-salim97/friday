import test from 'node:test';
import assert from 'node:assert/strict';
import { loadApp, settle } from '../lib/load-app.mjs';

test('platform admin panel loads metrics, searches, filters, grants, revokes, and navigates', async (t) => {
  const app = await loadApp();
  t.after(app.close);
  app.evaluate(`state.currentUser={...state.data.users[0],isGuest:false,isPlatformAdmin:true}`);
  const initial = [
    { userId: 1, loginName: 'example-maintainer', chineseName: '', source: 'builtin', revocable: false },
    { userId: 2, loginName: 'reviewer', chineseName: '审阅者', source: 'granted', revocable: true },
  ];
  app.router.get('/api/platform/admins', initial);
  app.router.put('/api/platform/admins', (call) => call.body.role === 'admin'
    ? [...initial, { loginName: call.body.loginName, source: 'granted', revocable: true }]
    : initial.filter((row) => row.loginName !== call.body.loginName));
  app.evaluate(`renderPlatformAdminPanel()`);
  await settle(8);
  const host = app.document.querySelector('#platformView');
  assert.match(host.textContent, /示例用户|审阅者|2 位管理员/);
  const search = host.querySelector('#platformAdminSearch');
  search.value = '不存在';
  search.dispatchEvent(new app.window.Event('input', { bubbles: true }));
  assert.match(host.textContent, /未找到/);
  search.value = '';
  search.dispatchEvent(new app.window.Event('input', { bubbles: true }));
  host.querySelector('[data-source-filter="builtin"]')?.click();
  host.querySelector('[data-source-filter="granted"]')?.click();
  host.querySelector('[data-source-filter="all"]')?.click();

  const form = host.querySelector('#platformAdminForm');
  form.elements.loginName.value = 'newadmin';
  form.dispatchEvent(new app.window.Event('submit', { bubbles: true, cancelable: true }));
  await settle(6);
  assert.match(host.textContent, /newadmin/);

  host.querySelector('[data-revoke-admin="reviewer"]')?.click();
  await settle();
  app.document.querySelector('#confirmOkBtn').click();
  await settle(6);
  assert.ok(app.router.calls.filter((call) => call.path === '/api/platform/admins' && call.method === 'PUT').length >= 2);
  host.querySelector('[data-platform-nav="agents"]')?.click();
  assert.equal(app.evaluate(`state.currentView`), 'agents');
});

test('platform admin page splits accounts and admins into tabs', async (t) => {
  const app = await loadApp();
  t.after(app.close);
  app.evaluate(`state.currentUser={...state.data.users[0],isGuest:false,isPlatformAdmin:true}`);
  app.router.get('/api/platform/admins', [{ userId: 1, loginName: 'owner', chineseName: '', source: 'granted', revocable: true }]);
  app.router.get('/api/platform/accounts', { signupEnabled: true, accounts: [1, 2, 3].map((id) => ({ userId: id, loginName: `user${id}`, admin: id === 1, password: true })) });
  app.evaluate(`renderPlatformAdminPanel()`);
  await settle(8);
  const host = app.document.querySelector('#platformAdminPanel');
  const panel = (id) => host.querySelector(`[data-platform-panel="${id}"]`);
  const tab = (id) => host.querySelector(`[data-platform-tab="${id}"]`);
  const shown = () => ['accounts', 'admins'].filter((id) => !panel(id).hidden);

  assert.deepEqual(shown(), ['accounts'], 'accounts open first');
  assert.equal(tab('accounts').getAttribute('aria-selected'), 'true');
  assert.equal(tab('accounts').querySelector('[data-platform-tab-count]').textContent, '3');
  assert.equal(tab('admins').querySelector('[data-platform-tab-count]').textContent, '1');

  tab('admins').click();
  assert.deepEqual(shown(), ['admins']);
  assert.equal(tab('accounts').getAttribute('aria-selected'), 'false');
  host.querySelector('#platformTabs').dispatchEvent(new app.window.KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }));
  assert.deepEqual(shown(), ['accounts'], 'arrow keys wrap around');
  host.querySelector('#platformTabs').dispatchEvent(new app.window.KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }));
  assert.deepEqual(shown(), ['admins']);
  assert.ok(panel('admins').querySelector('.platform-metric-grid'), 'admin metrics live in the admins tab');

  app.evaluate(`renderPlatformAdminPanel()`);
  await settle(6);
  assert.deepEqual(shown(), ['admins'], 'the open tab survives a re-render');
});

test('platform admins create accounts and reset passwords from the page', async (t) => {
  const app = await loadApp();
  t.after(app.close);
  app.evaluate(`state.currentUser={...state.data.users[0],isGuest:false,isPlatformAdmin:true}`);
  const accounts = [
    { userId: 1, loginName: 'owner', admin: true, password: true },
    { userId: 2, loginName: 'octocat', admin: false, password: false },
  ];
  app.router.get('/api/platform/admins', []);
  app.router.get('/api/platform/accounts', { accounts, signupEnabled: false });
  app.router.post('/api/platform/accounts', (call) => ({ status: 201, body: { userId: 3, loginName: call.body.loginName, admin: call.body.admin, password: true } }));
  app.router.put('/api/platform/accounts/3/password', { userId: 3, loginName: 'alice', password: true });
  app.evaluate(`renderPlatformAdminPanel()`);
  await settle(8);
  const card = app.document.querySelector('#platformAccountsCard');
  assert.match(card.textContent, /自助注册已关闭/);
  assert.match(card.textContent, /共 2 个账号/);
  assert.ok(card.querySelector('[data-reset-account="1"]'), 'password accounts can be reset');
  assert.ok(!card.querySelector('[data-reset-account="2"]'), 'GitHub accounts have no platform password');
  assert.match(card.querySelector('[data-account-row="2"]').textContent, /无平台密码/);

  const form = card.querySelector('#platformAccountForm');
  form.elements.loginName.value = 'x';
  form.elements.password.value = 'secret-123';
  form.dispatchEvent(new app.window.Event('submit', { bubbles: true, cancelable: true }));
  await settle(4);
  assert.equal(app.router.calls.filter((call) => call.method === 'POST').length, 0, 'an invalid login is rejected before sending');

  form.elements.loginName.value = 'alice';
  card.querySelector('[data-account-generate]').click();
  const generated = form.elements.password.value;
  assert.match(generated, /^[A-Za-z0-9]{14}$/);
  form.dispatchEvent(new app.window.Event('submit', { bubbles: true, cancelable: true }));
  await settle(6);
  const created = app.router.calls.find((call) => call.method === 'POST' && call.path === '/api/platform/accounts');
  assert.deepEqual(created.body, { loginName: 'alice', password: generated, admin: false });
  assert.match(card.textContent, /共 3 个账号/);
  const note = card.querySelector('[data-account-created]');
  assert.equal(note.hidden, false);
  assert.match(note.textContent, /已创建账号 alice/);
  assert.equal(note.querySelector('code').textContent, generated);
  assert.equal(form.elements.loginName.value, '', 'the form clears after creating');

  card.querySelector('[data-reset-account="3"]').click();
  const reset = card.querySelector('[data-reset-form="3"]');
  reset.elements.password.value = 'second-pass';
  reset.dispatchEvent(new app.window.Event('submit', { bubbles: true, cancelable: true }));
  await settle(6);
  const resetCall = app.router.calls.find((call) => call.path === '/api/platform/accounts/3/password');
  assert.deepEqual(resetCall.body, { password: 'second-pass' });
  assert.match(note.textContent, /已重置账号 alice/);
  assert.ok(!card.querySelector('[data-reset-form]'), 'the inline reset closes');

  const search = card.querySelector('[data-account-search]');
  search.value = 'octo';
  search.dispatchEvent(new app.window.Event('input', { bubbles: true }));
  assert.equal(card.querySelectorAll('[data-account-row]').length, 1);
});
