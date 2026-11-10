// The opening scene exports its character interaction. Use that same local module on article links.
const source = document.documentElement.dataset.journalGlyphs;
const titles = document.querySelectorAll('[data-journal-title]');
if (source && titles.length) {
  const motion = matchMedia('(hover: hover) and (pointer: fine) and (prefers-reduced-motion: no-preference)');
  let cleanups = [];
  let onPage = true;

  function release() {
    cleanups.forEach(cleanup => cleanup());
    cleanups = [];
  }
  async function update() {
    if (!onPage || !motion.matches) { release(); return; }
    if (cleanups.length) return;
    let mount;
    try { ({m: mount} = await import(source)); }
    catch { return; } // An ungenerated or unavailable bundle leaves the CSS hover in place.
    if (!onPage || !motion.matches || cleanups.length) return;
    cleanups = Array.from(titles, title => mount(title));
  }

  motion.addEventListener('change', update);
  window.addEventListener('pagehide', () => { onPage = false; release(); });
  window.addEventListener('pageshow', () => { onPage = true; update(); });
  update();
}
