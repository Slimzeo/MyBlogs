(() => {
  'use strict';
  const {data, description} = window.blogHome;
  const title = (window.blogHome.title || 'HYPNOS').replace(/hypn0s/gi, word => word === word.toUpperCase() ? 'HYPNOS' : 'Hypnos');
  const columns = data.HomeContent.Columns || [];
  const categories = data.PublicSummary.Categories || [];
  const posts = data.Articles?.list || [];
  const all = (selector, root = document) => [...root.querySelectorAll(selector)];
  const text = (node, value) => { if (node && node.textContent !== String(value ?? '')) node.textContent = value ?? ''; };
  const set = (selector, value, root) => all(selector, root).forEach(node => text(node, value));
  const categoryURL = meta => '/category/' + encodeURIComponent(meta.name);
  const postURL = post => '/article/' + post.cid;
  const link = (name, url) => { const node = document.createElement('a'); node.textContent = name; node.href = url; return node; };
  const bindLink = (node, url) => { if (node) { node.href = url; node.dataset.blogUrl = url; node.removeAttribute('target'); } };
  const word = (node, name, url) => {
    if (!node) return;
    const bold = document.createElement('strong'); bold.textContent = name;
    node.replaceChildren(bold);
    if (node.tagName === 'A') bindLink(node, url);
    else { const anchor = link('', url); anchor.append(bold); node.replaceChildren(anchor); }
  };
  document.title = title;
  if (description) document.querySelector('meta[name="description"]').content = description;
  all('a[href]').forEach(node => {
    const href = node.getAttribute('href');
    if (/aipay|alipay|act-protocol|community\/events/.test(href)) {
      let url = '/';
      if (/about/.test(href)) url = '/about';
      else if (/developers|docs|sandbox/.test(href)) url = '/notes';
      else if (/act-protocol/.test(href)) url = '/archives';
      else if (/\/act\//.test(href)) url = '/topics';
      else if (/community/.test(href) && !/events/.test(href)) url = '/links';
      bindLink(node, url);
    }
  });
  all('[data-blog-module]').forEach(node => {
    if (window.blogHome.modules?.[node.dataset.blogModule] === false) node.remove();
  });
  all('.showcase-more[href]').forEach(node => bindLink(node, '/archives'));
  all('.race-partners-list').forEach(list => {
    const sample = list.firstElementChild;
    while (list.children.length < (data.Tags || []).length) list.append(sample.cloneNode(true));
  });
  all('.race-partners-list li').forEach((node, index) => {
    const tag = (data.Tags || [])[index % (data.Tags || []).length];
    if (tag) word(node, tag.name, '/tag/' + encodeURIComponent(tag.name));
    else node.hidden = true;
  });
  const counter = document.querySelector('[data-prize-count]');
  const removeCurrency = () => all('.prize-symbol', counter).forEach(node => {
    if (node.textContent === '¥') node.hidden = true;
  });
  new MutationObserver(removeCurrency).observe(counter, {childList: true, subtree: true});
  set('[data-prize-count]', data.PublicSummary.Articles);
  all('[data-prize-count]').forEach(node => node.dataset.prizeCount = String(data.PublicSummary.Articles));
  all('[data-prize-label]').forEach(node => { node.dataset.prizeLabel = String(data.PublicSummary.Articles); node.setAttribute('aria-label', node.dataset.prizeLabel); });
  all('.reward-medal-track').forEach((node, index) => {
    const column = columns[index];
    if (!column) { node.hidden = true; return; }
    const heading = node.querySelector('.reward-medal-heading h3');
    all('span', heading).forEach((span, i) => text(span, i === 0 ? column.Meta.name : ''));
    heading.setAttribute('aria-label', column.Meta.name);
    all('.reward-medal-amount strong', node).forEach((amount, i) => text(amount, column.Top?.[i]?.hits ?? column.Meta.count));
  });
  all('.track-panel').forEach((node, index) => {
    const column = columns[index];
    if (!column) { node.hidden = true; return; }
    set('.track-toggle > span:first-child, .track-detail-title', column.Meta.name, node);
    set('.track-detail-head p', column.Meta.description, node);
    node.querySelector('.track-entry').dataset.blogUrl = categoryURL(column.Meta);
    set('.track-entry > span:first-child', '进入专栏', node);
    bindLink(node.querySelector('.track-product-link'), '/topics');
    all('.track-road-sign dd', node).forEach(list => {
      list.replaceChildren(...(column.Latest || []).map(post => {
        const anchor = link(post.title, postURL(post)); anchor.className = 'track-delivery-option'; return anchor;
      }));
    });
  });
  const popular = data.HomeContent.MostRead || [];
  all('.partner-benefit-card').forEach((node, index) => {
    const post = popular[index % popular.length];
    if (!post) { node.hidden = true; return; }
    word(node.querySelector('.partner-benefit-logo'), post.categories, postURL(post));
    set('.partner-benefit-copy h4', post.title, node);
    all('.partner-benefit-claim', node).forEach(anchor => { bindLink(anchor, postURL(post)); text(anchor, '阅读全文'); anchor.hidden = false; });
    all('[data-benefit-access]', node).forEach(button => button.remove());
  });
  all('.partner-wall-group').forEach(group => {
    if (!group.classList.contains('partner-wall-group--organizer')) {
      const list = group.querySelector('.partner-wall-grid');
      const sample = list.firstElementChild;
      while (list.children.length < (data.Links || []).length) list.append(sample.cloneNode(true));
    }
    all('.partner-wall-logo', group).forEach((node, index) => {
      if (group.classList.contains('partner-wall-group--organizer')) { word(node, 'HYPNOS cloud', '/about'); return; }
      const friend = (data.Links || [])[index % (data.Links || []).length];
      if (!friend) { node.hidden = true; return; }
      let url;
      try { url = new URL(friend.slug, location.href); } catch { node.hidden = true; return; }
      if (!['http:', 'https:'].includes(url.protocol)) { node.hidden = true; return; }
      word(node, friend.name, url.href);
      const anchor = node.tagName === 'A' ? node : node.querySelector('a');
      anchor.target = '_blank'; anchor.rel = 'noopener';
    });
  });
  all('.footerNavigationItems').forEach((list, index) => {
    const nav = index === 0 ? categories.map(meta => [meta.name, categoryURL(meta)]) :
      index === 1 ? [['Notes', '/notes'], ['Archives', '/archives'], ['Topics', '/topics'], ['Friends', '/links']] : [['HYPNOS cloud', '/about']];
    const prototype = list.firstElementChild;
    list.replaceChildren(...nav.map(([name, url]) => {
      const node = prototype.cloneNode(true); const anchor = node.querySelector('a');
      text(anchor, name); bindLink(anchor, url); return node;
    }));
  });
  set('.siteLegalFooterCopyright', '© ' + title + ' · Powered by Go · Theme inspired by Fluid');
  const copyright = document.querySelector('.siteLegalFooterCopyright');
  const themeLink = link('Fluid', 'https://github.com/fluid-dev/hexo-theme-fluid');
  themeLink.target = '_blank'; themeLink.rel = 'noopener noreferrer';
  copyright.replaceChildren(document.createTextNode('© ' + title + ' · Powered by Go · Theme inspired by '), themeLink);
  set('.siteLegalFooterRegistration', description);
  all('.siteLegalFooterContact, .siteLegalFooterMachineLink').forEach(node => node.hidden = true);
  const build = [data.PublicSummary.Articles, data.PublicSummary.Comments, new Date(data.PublicSummary.Latest * 1000).toLocaleString()];
  all('.buildDetails dd').forEach((node, index) => text(node, build[index]));
  // The gallery still owns card creation and its animation lifecycle.
  const days = data.HomeContent.Days || [];
  const months = data.HomeContent.Months || [];
  const dayGrid = document.querySelector('.schedule-days');
  const dayPrototype = dayGrid.querySelector('button');
  const phases = all('.schedule-caption').map(node => node.dataset.phase);
  const offset = days.length ? (new Date(days[0].Date + 'T12:00:00').getDay() + 6) % 7 : 0;
  dayGrid.replaceChildren(...days.map((day, index) => {
    const node = dayPrototype.cloneNode(true);
    node.dataset.date = day.Date;
    node.dataset.phases = day.Count ? phases[day.Month] : '';
    node.classList.toggle('schedule-day--inactive', !day.Count);
    node.tabIndex = index === 0 ? 0 : -1;
    node.style.gridColumn = String(Math.floor((index + offset) / 7) + 1);
    node.style.gridRow = String((index + offset) % 7 + 1);
    node.setAttribute('aria-label', day.Date + ' · ' + day.Count);
    return node;
  }));
  all('.schedule-months span').forEach((node, index) => {
    const month = months[index]; if (!month) return;
    text(node, month.Label);
    node.style.gridColumn = String(Math.floor((days.findIndex(day => day.Month === index) + offset) / 7) + 1);
  });
  all('.schedule-caption time').forEach((node, index) => {
    const month = months[index]; if (month) text(node, month.Label + ' · ' + month.Count);
  });
  const year = document.querySelector('.schedule-calendar-year');
  if (days.length) { text(year, days[0].Date.slice(0, 4)); year.setAttribute('aria-label', year.textContent); }
  const gallery = document.querySelector('[data-work-grid]');
  function bindGallery() {
    all('.payathon-work-card', gallery).forEach((node, index) => {
      const existingURL = node.getAttribute('href') || '';
      const match = decodeURIComponent(existingURL).match(/work\/(\d+)|article\/(\d+)/);
      const post = match ? posts.find(post => String(post.cid) === (match[1] || match[2])) : posts[index % posts.length];
      if (!post) return;
      const anchor = node.tagName === 'A' ? node : node.querySelector('a');
      if (anchor && anchor.getAttribute('href') !== postURL(post)) bindLink(anchor, postURL(post));
      all('.payathon-work-track', node).forEach(badge => text(badge, post.categories));
      const watermark = node.querySelector('.payathon-work-watermark');
      if (watermark && !watermark.dataset.blogBound) {
        watermark.dataset.blogBound = 'true';
        watermark.setAttribute('aria-label', 'HYPNOS');
        const group = document.createElementNS('http://www.w3.org/2000/svg', 'g');
        group.setAttribute('transform', 'translate(0 4) scale(.31)');
        const logo = document.querySelector('.payathon-name');
        all('[data-title-glyphs] > path', logo).forEach(glyph => {
          const copy = glyph.cloneNode(true); copy.removeAttribute('style'); copy.removeAttribute('data-title-char');
          all('[data-title-char]', copy).forEach(child => { child.removeAttribute('style'); child.removeAttribute('data-title-char'); });
          group.append(copy);
        });
        watermark.querySelector('text')?.replaceWith(group);
        text(watermark.querySelector('text'), 'HYPNOS · BLOG');
      }
    });
  }
  if (gallery) new MutationObserver(bindGallery).observe(gallery, {childList: true, subtree: true});
})();
