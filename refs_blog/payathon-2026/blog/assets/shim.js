(() => {
  'use strict';
  const external = value => {
    try { return new URL(value instanceof Request ? value.url : value, location.href).origin !== location.origin; }
    catch { return true; }
  };
  const response = value => new Response(JSON.stringify(value), {status: 200, headers: {'Content-Type': 'application/json'}});
  const fetchOriginal = window.fetch.bind(window);
  window.fetch = (input, options) => {
    const url = new URL(input instanceof Request ? input.url : input, location.href);
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
})();
