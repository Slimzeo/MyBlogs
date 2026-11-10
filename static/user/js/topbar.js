// One controller for the public navigation, with or without the animated guide.
(() => {
  const bar = document.querySelector('#blog-topbar');
  if (!bar) return;
  const nav = bar.querySelector('nav'), menu = nav.querySelector('[data-site-menu-toggle]');
  const panel = document.getElementById('blog-nav-panel'), compact = matchMedia('(max-width: 980px)');
  function setMenu(open, restore = false) {
    open = compact.matches && open;
    nav.classList.toggle('is-menu-open', open);
    menu.setAttribute('aria-expanded', String(open));
    menu.setAttribute('aria-label', open ? '收起导航菜单' : '展开导航菜单');
    panel.inert = compact.matches && !open;
    if (restore && compact.matches) menu.focus();
  }
  // Use the reading palette for the navigation text after the photograph leaves the viewport.
  let observed, observer;
  function observeBackdrop() {
    const banner = document.querySelector('.payathon-hero,.hx-hero,.fluid-banner:has(img)');
    if (banner === observed) return;
    observer?.disconnect(); observed = banner;
    document.documentElement.toggleAttribute('data-reading', !banner);
    if (!banner) return;
    // Use the home scroll cue to switch the header as it leaves the screen, also after a #blogs jump.
    const cue = document.querySelector('.journal-down');
    observer = new IntersectionObserver(([entry]) => {
      const reading = cue ? entry.boundingClientRect.top < bar.offsetHeight : !entry.isIntersecting;
      document.documentElement.toggleAttribute('data-reading', reading);
    }, {rootMargin: '-' + bar.offsetHeight + 'px 0px 0px', threshold: cue ? 1 : 0});
    observer.observe(cue || banner);
  }
  observeBackdrop();
  document.addEventListener('blog:page-change', observeBackdrop);
  setMenu(false);
  menu.addEventListener('click', () => setMenu(menu.getAttribute('aria-expanded') !== 'true'));
  compact.addEventListener('change', () => setMenu(false));
  document.addEventListener('pointerdown', e => { if (!nav.contains(e.target)) setMenu(false); });
  document.addEventListener('focusin', e => { if (!nav.contains(e.target)) setMenu(false); });
  const search = document.getElementById('blog-search');
  function openDialog(dialog) {
    setMenu(false);
    if (!dialog || dialog.open) return;
    dialog.showModal();
    dialog.querySelector('input')?.focus();
  }
  bar.querySelector('[data-blog-search-open]').addEventListener('click', () => openDialog(search));
  bar.querySelector('#access-key-toggle')?.addEventListener('click', () => openDialog(document.getElementById('access-key-modal')));
  document.addEventListener('keydown', e => {
    if (e.key === 'Escape' && menu.getAttribute('aria-expanded') === 'true') setMenu(false, true);
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') { e.preventDefault(); openDialog(search); }
  });
  document.addEventListener('click', e => {
    const dialog = e.target.closest('.blog-dialog');
    if (!dialog) return;
    if (e.target.closest('[data-dialog-close]') || e.target === dialog) dialog.close();
  });
  // Delegate to the current dialog, including after a Fluid article navigation.
  document.addEventListener('submit', async e => {
    const form = e.target, dialog = form.closest('.blog-dialog');
    if (!dialog) return;
    e.preventDefault();
    if (dialog === search) {
      const term = new FormData(form).get('q').trim();
      if (term) location.assign('/search/' + encodeURIComponent(term));
      return;
    }
    const revoke = form.id === 'access-key-revoke-form', status = dialog.querySelector('[role=status], #access-key-status');
    const submit = form.querySelector('[type=submit]');
    submit.disabled = true; status.textContent = revoke ? '正在撤销…' : '正在验证…';
    try {
      const response = await fetch(revoke ? '/access-key/revoke' : '/access-key', {
        method: 'POST', credentials: 'same-origin', headers: {'X-CSRF-Token': dialog.dataset.csrf},
        body: new URLSearchParams(new FormData(form))
      });
      const result = await response.json();
      if (!result.success) throw new Error(result.msg || '密钥验证失败');
      location.reload();
    } catch (err) { status.textContent = err.message; }
    finally { submit.disabled = false; }
  });
  // Fluid already schedules expiry when it swaps articles.
  if (!document.body.classList.contains('fluid-theme')) {
    const expires = Number(document.getElementById('access-key-modal')?.dataset.expires) * 1000;
    if (expires > 0) setTimeout(() => location.reload(), Math.min(Math.max(0, expires - Date.now()) + 1000, 2147483647));
  }
  window.addEventListener('pageshow', () => setMenu(false));
})();
