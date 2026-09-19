import { request } from '@playwright/test';

// Block the suite until the community server answers /readyz. Replaces the
// enterprise global-setup's auth/persona/workspace provisioning (community
// has none of that — the globe is open).
export default async function globalSetup() {
  const baseURL = process.env.E2E_BASE_URL ?? 'http://localhost:8090';
  const ctx = await request.newContext({ baseURL });
  const deadlineMs = Date.now() + 60_000;
  for (;;) {
    try {
      const res = await ctx.get('/readyz');
      if (res.ok()) break;
    } catch {
      // server not up yet — keep polling
    }
    if (Date.now() > deadlineMs) {
      await ctx.dispose();
      throw new Error(`community server not ready at ${baseURL} after 60s`);
    }
    await new Promise((r) => setTimeout(r, 1000));
  }
  await ctx.dispose();
  console.log(`[e2e] server ready at ${baseURL}`);
}
