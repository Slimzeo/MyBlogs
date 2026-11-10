// Use a fresh Chrome profile with its usual HTTPS upgrades and certificate checks.
const os = require('node:os');
const path = require('node:path');
const { createRequire } = require('node:module');
const bundled = path.join(os.homedir(), '.cache/codex-runtimes/codex-primary-runtime/dependencies/node/package.json');
function loadChromium() {
  const loaders = [[require, 'playwright-core'], [require, 'playwright'], [createRequire(bundled), 'playwright-core']];
  for (const [load, moduleName] of loaders) {
    try { return load(moduleName).chromium; }
    catch (err) { if (err.code !== 'MODULE_NOT_FOUND') throw err; }
  }
  throw Error('Install playwright-core and Google Chrome for browser checks, or verify in a normal browser.');
}
const inputs = process.argv.slice(2);
const urls = inputs.length ? inputs : ['http://www.hypn0s.cloud/', 'https://hypn0s.cloud/'];
(async () => {
  const browser = await loadChromium().launch({ channel: 'chrome', headless: true });
  try {
    for (const input of urls) {
      const context = await browser.newContext();
      const page = await context.newPage();
      const navigation = [];
      page.on('request', request => {
        if (request.isNavigationRequest() && request.frame() === page.mainFrame()) navigation.push(request.url());
      });
      try {
        const response = await page.goto(input, { waitUntil: 'domcontentloaded', timeout: 18000 });
        if (!response || response.status() !== 200) throw Error('Main document HTTP ' + response?.status());
        const target = new URL(page.url());
        const original = new URL(input);
        const blogHosts = ['hypn0s.cloud', 'www.hypn0s.cloud'];
        if (!(blogHosts.includes(original.hostname) ? blogHosts.includes(target.hostname) : target.host === original.host) ||
            (original.protocol === 'https:' && target.protocol !== 'https:')) throw Error('Unexpected destination');
        await page.locator('#blog-topbar').waitFor({ state: 'attached', timeout: 5000 });
        const content = page.locator('#blogs, #blog-library, .fluid-reader').first();
        await content.waitFor({ state: 'attached', timeout: 5000 });
        console.log('OK', input, '→', page.url(), await page.title());
      } catch (err) {
        console.error('FAIL', navigation.join(' → ') || input, String(err.message).split('\n')[0]);
        process.exitCode = 1;
      } finally { await context.close(); }
    }
  } finally { await browser.close(); }
})().catch(err => { console.error(err.message); process.exitCode = 1; });
