// Fragment links also support direct links, Back, and browsing with JavaScript disabled.
export function mountProjects(gallery) {
  if (gallery.dataset.bound) return;
  gallery.dataset.bound = '1';

  function step(slide, direction) {
    slide.querySelector('.project-' + direction).click();
  }

  gallery.addEventListener('keydown', event => {
    if (event.altKey || event.ctrlKey || event.metaKey || event.shiftKey) return;
    const slide = event.target.closest('.project-slide');
    if (!slide || (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight')) return;
    event.preventDefault();
    step(slide, event.key === 'ArrowRight' ? 'next' : 'prev');
  });

  let start;
  gallery.addEventListener('touchstart', event => {
    start = event.touches.length === 1 ? event.touches[0] : null;
  }, {passive: true});
  gallery.addEventListener('touchcancel', () => { start = null; }, {passive: true});
  gallery.addEventListener('touchend', event => {
    const slide = event.target.closest('.project-slide');
    if (!start || !slide) return;
    const dx = event.changedTouches[0].clientX - start.clientX;
    const dy = event.changedTouches[0].clientY - start.clientY;
    start = null;
    if (Math.abs(dx) > 48 && Math.abs(dx) > 1.5 * Math.abs(dy)) {
      step(slide, dx < 0 ? 'next' : 'prev');
    }
  }, {passive: true});
}
