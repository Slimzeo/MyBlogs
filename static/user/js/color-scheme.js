// Resolve before first paint in both the animated and the reading layout.
(() => {
  const root = document.documentElement, key = 'fluid-color-scheme';
  const system = matchMedia('(prefers-color-scheme: dark)');
  const stored = () => { try { return localStorage.getItem(key); } catch { return null; } };
  let choice = stored();
  function apply() {
    const mode = ['dark', 'light'].includes(choice) ? choice : (system.matches ? 'dark' : 'light');
    root.classList.add('js');
    root.dataset.colorScheme = root.dataset.theme = mode;
    root.style.colorScheme = mode;
    const message = mode === 'dark' ? '切换到日间模式' : '切换到夜间模式';
    document.querySelectorAll('[data-blog-theme]').forEach(button => {
      button.setAttribute('aria-label', message);
      button.title = button.dataset.guideMessage = message;
    });
    document.querySelector('meta[name="theme-color"]')?.setAttribute('content', mode === 'dark' ? '#141414' : '#f8f7f3');
    window.dispatchEvent(new Event('aipay:theme-change')); // Let the animated guide resample its contrast.
  }
  apply();
  document.addEventListener('DOMContentLoaded', apply, {once: true});
  document.addEventListener('click', event => {
    if (!event.target.closest?.('[data-blog-theme],[data-site-theme-toggle]')) return;
    choice = root.dataset.colorScheme === 'dark' ? 'light' : 'dark';
    try { localStorage.setItem(key, choice); } catch {}
    apply();
  });
  window.addEventListener('storage', event => { if (event.key === key || event.key === null) { choice = stored(); apply(); } });
  window.addEventListener('pageshow', () => { choice = stored(); apply(); });
  system.addEventListener('change', apply);
})();
