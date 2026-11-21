import {test,expect} from '@playwright/test';
import {password as adminPassword, signInUnconfigured, appReady, switchProject, chat, openHome} from './helpers.mjs';

// Credential UI must never enter a Playwright trace or screenshot artifact.
test.use({trace:'off',screenshot:'off'});

test('the configured home composer sends a real model reply and reloads its saved conversation',async({page})=>{
  test.skip(process.env.AGENTFLOW_E2E_MODEL!=='1','Real model calls require AGENTFLOW_E2E_MODEL=1');
  test.setTimeout(300_000);
  const errors=[];page.on('pageerror',e=>errors.push(e.message));
  await page.goto('/login.html');
  await page.locator('[name="login"]').fill(process.env.FRIDAY_TEST_USER||'admin');
  await page.locator('[name="password"]').fill(adminPassword());
  await page.locator('button[type="submit"]').click();
  await appReady(page);
  await switchProject(page,'welcome');
  const input=await openHome(page);
  const marker=`BROWSER-LIVE-${Date.now().toString(36)}`;
  await input.fill(`这是只读的自动化测试。请回复一句话，包含 ${marker}。不要创建、修改或删除任何平台对象或文件。`);
  await chat(page).locator('.send-button').click();
  const reply=chat(page).locator('.message.assistant').last();
  const sessionId=()=>new URL(page.frames().find(f=>f.url().includes('/chatbot/'))?.url()||'http://x/').searchParams.get('session')||'';
  try {
    await expect(reply).toContainText(marker,{timeout:240_000});
    await expect.poll(sessionId).not.toBe('');
    await page.reload();
    await expect(chat(page).locator('.message.assistant').last()).toContainText(marker,{timeout:30_000});
    // The streamed reply is visible before the background generation commits it.
    await expect.poll(async()=>{
      const generation=await page.request.get(`/api/assistant/sessions/${sessionId()}/generation`);
      return generation.ok()?(await generation.json()).status:'';
    },{timeout:30_000}).toBe('completed');
    const saved=await page.request.get(`/api/assistant/sessions/${sessionId()}`);
    expect(saved.ok()).toBe(true);
    expect((await saved.json()).messages.some(m=>m.role==='assistant'&&m.content.includes(marker))).toBe(true);
    expect(errors).toEqual([]);
  } finally {
    if(sessionId())await page.request.delete(`/api/assistant/sessions/${sessionId()}`);
  }
});
