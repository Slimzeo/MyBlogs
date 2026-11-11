// A native scroll area retains touch, trackpad and fragment navigation without JS.
export function mountProjects(gallery) {
  const rail = gallery.querySelector('[data-project-rail]');
  if (!rail || mounted.has(rail)) return;
  mounted.add(rail);

  const controls = gallery.querySelector('.project-rail-footer');
  const previous = controls.querySelector('[data-project-prev]');
  const next = controls.querySelector('[data-project-next]');
  const cards = Array.from(rail.querySelectorAll('[data-project-card]'));
  const motion = window.matchMedia('(prefers-reduced-motion: reduce)');

  function update() {
    const max = rail.scrollWidth - rail.clientWidth;
    const atStart = rail.scrollLeft < 2;
    const atEnd = rail.scrollLeft >= max - 2;
    if ((atStart && document.activeElement === previous) || (atEnd && document.activeElement === next)) {
      rail.focus({preventScroll: true});
    }
    controls.hidden = cards.length < 2 || max < 2;
    previous.disabled = atStart;
    next.disabled = atEnd;
  }

  // Use real, untransformed layout offsets. At the end the browser clamps the
  // final card to the viewport's right gutter, including at intermediate sizes.
  function move(forward) {
    const max = rail.scrollWidth - rail.clientWidth;
    const stops = cards.map(card => Math.min(card.offsetLeft - cards[0].offsetLeft, max));
    const target = forward ? stops.find(x => x > rail.scrollLeft + 2)
      : stops.reverse().find(x => x < rail.scrollLeft - 2);
    scrollTo(target ?? (forward ? max : 0));
  }
  function scrollTo(left) {
    rail.scrollTo({left, behavior: motion.matches ? 'instant' : 'smooth'});
  }
  previous.addEventListener('click', () => move(false));
  next.addEventListener('click', () => move(true));
  rail.addEventListener('keydown', event => {
    if (event.altKey || event.ctrlKey || event.metaKey || event.shiftKey) return;
    switch (event.key) {
      case 'ArrowLeft': move(false); break;
      case 'ArrowRight': move(true); break;
      case 'Home':
      case 'End':
        if (event.target !== rail) return;
        scrollTo(event.key === 'Home' ? 0 : rail.scrollWidth);
        break;
      default: return;
    }
    event.preventDefault();
  });
  rail.addEventListener('scroll', update, {passive: true});
  const observer = new ResizeObserver(update);
  observer.observe(rail);
  document.addEventListener('blog:page-change', () => observer.disconnect(), {once: true});
  gallery.setAttribute('data-enhanced', '');
  update();
}

// The blog caches DOM snapshots for page changes. A cloned data attribute
// cannot tell whether the newly mounted rail has event listeners.
const mounted = new WeakSet();
