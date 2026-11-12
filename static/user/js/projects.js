// A native scroll rail keeps touch, trackpad and fragment navigation working without JS.
export function mountProjects(gallery) {
  if (!gallery?.isConnected || mounted.has(gallery)) return;

  const rail = gallery.querySelector('[data-project-rail]');
  const footer = gallery.querySelector('.project-rail-footer');
  if (!rail) return;
  const prev = footer.querySelector('[data-project-prev]');
  const next = footer.querySelector('[data-project-next]');
  clearState(gallery, rail);
  const cards = [...rail.querySelectorAll('[data-project-card]')];
  const count = cards.length;
  footer.hidden = count < 2;
  if (!count) return;

  // A project's palette follows backend order, never its clone position.
  const canonical = new Map(cards.map((card, index) => [card, index]));
  cards.forEach((card, index) => {
    card.dataset.palette = String(index % 3);
  });
  if (count > 1) {
    const before = document.createDocumentFragment();
    const after = document.createDocumentFragment();
    cards.slice(-2).forEach(card => before.append(cloneCard(card, canonical.get(card))));
    cards.slice(0, 2).forEach(card => after.append(cloneCard(card, canonical.get(card))));
    rail.prepend(before);
    rail.append(after);
  }

  // Physical slots are fixed for this mount; clones only ever supply two per side.
  const slots = [...rail.querySelectorAll('.project-card')].map((el, slot) => {
    const clone = el.hasAttribute('data-clone');
    return {el, slot, clone, index: clone ? Number(el.dataset.cloneOf) : canonical.get(el)};
  });
  const real = slots.filter(entry => !entry.clone);
  const entryFor = index => real[((index % count) + count) % count];

  const motion = matchMedia('(prefers-reduced-motion: reduce)');
  const abort = new AbortController();
  const listen = (element, type, handler, options) => {
    element.addEventListener(type, handler, {...options, signal: abort.signal});
  };
  let settledIndex = 0;
  let requestedIndex = null;
  let direction = 1;
  let target = null;
  let focused = null;
  let rebasing = false;
  let pointerDown = false;
  let lastScrollTime = 0;
  let settleTimer = 0;
  let frame = 0;
  let railWidth = rail.clientWidth;

  function center(entry) {
    const max = Math.max(0, rail.scrollWidth - rail.clientWidth);
    const left = entry.el.offsetLeft + entry.el.offsetWidth / 2 - rail.clientWidth / 2;
    return Math.max(0, Math.min(max, left));
  }

  function nearest() {
    const middle = rail.scrollLeft + rail.clientWidth / 2;
    let best = null;
    let gap = Infinity;
    for (const entry of slots) {
      const delta = Math.abs(entry.el.offsetLeft + entry.el.offsetWidth / 2 - middle);
      if (delta < gap) {
        best = entry;
        gap = delta;
      }
    }
    return best;
  }

  function scrollTo(entry) {
    target = entry;
    lastScrollTime = performance.now();
    rail.scrollTo({left: center(entry), behavior: motion.matches ? 'instant' : 'smooth'});
    schedule();
  }

  function setFocus(entry) {
    if (focused === entry) return;
    focused?.el.classList.remove('is-focused');
    if (focused) cards[focused.index].removeAttribute('aria-current');
    entry.el.classList.add('is-focused');
    cards[entry.index].setAttribute('aria-current', 'true');
    focused = entry;
    gallery.dataset.tone = String(entry.index % 3);
  }

  // One painted frame with the class lets the clone swap without a stale snap transition.
  function teleportTo(entry) {
    clearTimeout(settleTimer);
    target = null;
    rebasing = true;
    settledIndex = entry.index;
    gallery.classList.add('is-teleporting');
    lastScrollTime = performance.now();
    rail.scrollTo({left: center(entry), behavior: 'instant'});
    setFocus(entry);
    cancelAnimationFrame(frame);
    frame = requestAnimationFrame(() => {
      frame = requestAnimationFrame(() => {
        frame = 0;
        rebasing = false;
        gallery.classList.remove('is-teleporting');
        advance();
      });
    });
  }

  function advance() {
    if (rebasing || pointerDown || target || count < 2) return;
    if (requestedIndex === null || requestedIndex === settledIndex) {
      requestedIndex = null;
      return;
    }
    const anchor = nearest();
    const nextSlot = slots[anchor.slot + direction];
    if (!nextSlot) {
      teleportTo(entryFor(anchor.index));
      return;
    }
    scrollTo(nextSlot);
  }

  function step(amount) {
    if (count < 2) return;
    const base = requestedIndex ?? nearest().index;
    requestedIndex = (base + amount + count) % count;
    direction = amount;
    advance();
  }

  function cancel() {
    target = null;
    requestedIndex = null;
    clearTimeout(settleTimer);
  }

  function moveTo(entry) {
    const index = entry.index;
    cancel();
    const active = nearest();
    if (active === entry && Math.abs(rail.scrollLeft - center(entry)) <= 1) {
      settledIndex = index;
      if (entry.clone) teleportTo(entryFor(index));
      else setFocus(entry);
      return;
    }
    requestedIndex = index;
    direction = entry.slot < active.slot ? -1 : 1;
    scrollTo(entry);
  }

  function schedule(delay = 160) {
    if (pointerDown) return;
    clearTimeout(settleTimer);
    settleTimer = setTimeout(settle, delay);
  }

  function settle() {
    if (pointerDown || rebasing || rail.clientWidth !== railWidth) return;
    const active = nearest();
    if (target && active !== target &&
        Math.abs(rail.scrollLeft - center(target)) > 2 &&
        performance.now() - lastScrollTime < 160) {
      schedule();
      return;
    }
    target = null;
    settledIndex = active.index;
    setFocus(active);
    if (active.clone) {
      teleportTo(entryFor(active.index));
      return;
    }
    if (Math.abs(rail.scrollLeft - center(active)) > 1) {
      rail.scrollTo({left: center(active), behavior: 'instant'});
    }
    advance();
  }

  function onScroll() {
    lastScrollTime = performance.now();
    if (rebasing || rail.clientWidth !== railWidth) return;
    setFocus(nearest());
    if (!pointerDown) schedule();
  }

  function onScrollEnd() {
    if (pointerDown) return;
    clearTimeout(settleTimer);
    settle();
  }

  function onDown(event) {
    if (event.pointerType === 'touch' || (event.pointerType === 'mouse' && event.button !== 0)) return;
    pointerDown = true;
    cancel();
  }

  function onUp(event) {
    if (!pointerDown || event.pointerType === 'touch' || event.touches?.length) return;
    pointerDown = false;
    schedule(120);
  }

  function onWheel(event) {
    if (!event.ctrlKey && (event.deltaX || event.shiftKey)) cancel();
  }

  function onClick(event) {
    if (event.defaultPrevented || event.button !== 0 || event.target.closest('a, button')) return;
    // Inert copies pass pointer events to the rail. Hit-test their visible bounds.
    const entry = slots.find(({el}) => {
      const box = el.getBoundingClientRect();
      return event.clientX >= box.left && event.clientX <= box.right &&
        event.clientY >= box.top && event.clientY <= box.bottom;
    });
    if (entry) moveTo(entry);
  }

  function onFocusIn(event) {
    const card = event.target.closest?.('[data-project-card]');
    const index = card && canonical.get(card);
    if (index != null) moveTo(entryFor(index));
  }

  function onKeyDown(event) {
    if (event.defaultPrevented || event.altKey || event.ctrlKey || event.metaKey ||
        event.shiftKey || count < 2) return;
    if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') {
      event.preventDefault();
      step(event.key === 'ArrowRight' ? 1 : -1);
    } else if ((event.key === 'Home' || event.key === 'End') && event.target === rail) {
      event.preventDefault();
      const index = event.key === 'Home' ? 0 : count - 1;
      moveTo(entryFor(index));
    }
  }

  function hashIndex() {
    try {
      const id = decodeURIComponent(window.location.hash.slice(1));
      return id ? (canonical.get(document.getElementById(id)) ?? -1) : -1;
    } catch {
      return -1;
    }
  }

  function onHashChange() {
    const index = hashIndex();
    if (index < 0) return;
    cancel();
    teleportTo(entryFor(index));
  }

  gallery.setAttribute('data-enhanced', '');
  gallery.dataset.count = String(count);
  const initial = entryFor(Math.max(0, hashIndex()));
  teleportTo(initial);

  listen(prev, 'click', () => step(-1));
  listen(next, 'click', () => step(1));
  listen(footer, 'keydown', onKeyDown);
  listen(rail, 'keydown', onKeyDown);
  listen(rail, 'focusin', onFocusIn);
  listen(rail, 'click', onClick);
  listen(rail, 'wheel', onWheel, {passive: true});
  listen(rail, 'scroll', onScroll, {passive: true});
  listen(rail, 'scrollend', onScrollEnd);
  listen(rail, 'pointerdown', onDown);
  listen(window, 'pointerup', onUp, {passive: true});
  listen(window, 'pointercancel', onUp, {passive: true});
  // Native touch scrolling cancels pointer events before the finger actually lifts.
  listen(rail, 'touchstart', onDown, {passive: true});
  listen(window, 'touchend', onUp, {passive: true});
  listen(window, 'touchcancel', onUp, {passive: true});
  listen(window, 'hashchange', onHashChange);
  listen(document, 'blog:page-change', cleanup, {once: true});

  const observer = new ResizeObserver(() => {
    if (railWidth === rail.clientWidth) return;
    railWidth = rail.clientWidth;
    // Preserve the requested project even when layout changes mid-scroll.
    const index = requestedIndex ?? focused.index;
    cancel();
    teleportTo(entryFor(index));
  });
  observer.observe(rail);

  function cleanup() {
    abort.abort();
    observer.disconnect();
    clearTimeout(settleTimer);
    cancelAnimationFrame(frame);
    clearState(gallery, rail);
    footer.hidden = true;
    mounted.delete(gallery);
  }

  mounted.add(gallery);
}

function clearState(gallery, rail) {
  rail.querySelectorAll('[data-clone]').forEach(node => node.remove());
  rail.querySelectorAll('[data-project-card]').forEach(card => {
    card.classList.remove('is-focused');
    card.removeAttribute('aria-current');
    delete card.dataset.palette;
  });
  gallery.classList.remove('is-teleporting');
  gallery.removeAttribute('data-enhanced');
  delete gallery.dataset.count;
  delete gallery.dataset.tone;
}

function cloneCard(card, index) {
  // Prepare the artwork shared by the two ends of a wrap; other cards stay lazy.
  const art = card.querySelector('.project-cover');
  if (art) art.loading = 'eager';
  const clone = card.cloneNode(true);
  clone.removeAttribute('id');
  clone.removeAttribute('tabindex');
  clone.removeAttribute('data-project-card');
  clone.setAttribute('data-clone', '');
  clone.setAttribute('data-clone-of', String(index));
  clone.setAttribute('aria-hidden', 'true');
  clone.setAttribute('inert', '');
  clone.querySelectorAll('[id]').forEach(node => node.removeAttribute('id'));
  // Keep artwork and URLs, but remove every clone stop from the tab and AT order.
  clone.querySelectorAll('a, button, input, select, textarea, summary, [tabindex]')
    .forEach(node => node.setAttribute('tabindex', '-1'));
  return clone;
}

// Blog navigation caches DOM snapshots, so a data attribute cannot prove a rail
// still has listeners. Track the live gallery node instead.
const mounted = new WeakSet();
