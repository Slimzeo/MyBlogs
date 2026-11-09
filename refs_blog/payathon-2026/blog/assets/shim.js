(() => {
  'use strict';
  const payload = JSON.parse(document.getElementById('blog-home-data').textContent);
  window.blogHome = payload;
  const data = payload.data;
  const external = value => {
    try { return new URL(value instanceof Request ? value.url : value, location.href).origin !== location.origin; }
    catch { return true; }
  };
  const excerpt = value => {
    const doc = new DOMParser().parseFromString(value || '', 'text/html');
    return (doc.body.textContent || '').replace(/!\[[^\]]*\]\([^)]*\)/g, '').slice(0, 180);
  };
  const posts = data.Articles?.list || [];
  const works = {items: posts.map(post => {
    const match = (post.content || '').match(/!\[[^\]]*\]\(([^\s)]+)|<img\b[^>]*src=["']([^"']+)/i);
    const cover = match && (match[1] || match[2]);
    return {workId: String(post.cid), titleOrName: post.title, trackCode: 'VIBE_PAY',
      summary: excerpt(post.content), iconUrl: cover && !external(cover) ? cover : null,
      stats: {viewCount: post.hits, likeCount: post.commentsNum, commentCount: post.commentsNum}};
  })};
  const response = value => new Response(JSON.stringify(value), {status: 200, headers: {'Content-Type': 'application/json'}});
  const fetchOriginal = window.fetch.bind(window);
  window.fetch = (input, options) => {
    const url = new URL(input instanceof Request ? input.url : input, location.href);
    if (url.pathname === '/assets/works.json' || /listPublicWorks/.test(url.pathname)) return Promise.resolve(response(works));
    if (external(url)) return Promise.resolve(response({success: false, authenticated: false, data: null, items: []}));
    return fetchOriginal(input, options);
  };
  const open = XMLHttpRequest.prototype.open;
  XMLHttpRequest.prototype.open = function(method, url, ...args) {
    return open.call(this, external(url) ? 'GET' : method, external(url) ? 'data:application/json,%7B%7D' : url, ...args);
  };
  const beacon = navigator.sendBeacon.bind(navigator);
  navigator.sendBeacon = (url, body) => external(url) ? true : beacon(url, body);
  const imageSource = Object.getOwnPropertyDescriptor(HTMLImageElement.prototype, 'src');
  Object.defineProperty(HTMLImageElement.prototype, 'src', {...imageSource, set(value) {
    imageSource.set.call(this, external(value) ? 'data:image/gif;base64,R0lGODlhAQABAAD/ACwAAAAAAQABAAACADs=' : value);
  }});
  const setItem = Storage.prototype.setItem;
  const getItem = Storage.prototype.getItem;
  Storage.prototype.setItem = function(key, value) { if (key !== 'aipay:device-id') setItem.call(this, key, value); };
  Storage.prototype.getItem = function(key) { return key === 'aipay:device-id' ? null : getItem.call(this, key); };
  try { localStorage.removeItem('aipay:device-id'); } catch {}
  // Capture before the reference modules install account and campaign handlers.
  document.addEventListener('click', event => {
    const target = event.target.closest('[data-registration-open], [data-nav-login], [data-benefit-access]');
    if (!target) return;
    event.preventDefault();
    event.stopImmediatePropagation();
    location.assign(target.dataset.blogUrl || (target.hasAttribute('data-nav-login') ? '/admin/login' : '#showcase'));
  }, true);
})();
