// The photo and the title have separate clocks. Never hold up the intro for a download.
(() => {
  const hero = document.querySelector('.payathon-hero, .hx-hero');
  const source = document.querySelector('link[data-forest-source]');
  if (!hero || !source) return;
  const image = new Image();
  image.onload = async () => {
    try { await image.decode?.(); } catch { if (!image.naturalWidth) return; }
    hero.setAttribute('data-forest-ready', '');
  };
  image.src = source.href;
})();
