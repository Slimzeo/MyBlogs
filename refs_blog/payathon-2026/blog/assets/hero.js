const clock = document.querySelector('[data-site-launched]');
if (clock) {
  const launch = Date.parse(clock.dataset.siteLaunched);
  if (!Number.isFinite(launch)) throw new Error('Invalid blog launch timestamp');
  const parts = [...clock.querySelectorAll('.countdown-part')];
  let timeout;
  let visible = true;

  function update() {
    clearTimeout(timeout);
    if (document.hidden || !visible) return;
    const age = Math.max(0, Math.floor((Date.now() - launch) / 1000));
    const values = [Math.floor(age / 86400), Math.floor(age / 3600) % 24,
      Math.floor(age / 60) % 60, age % 60];
    const days = String(values[0]).padStart(3, '0');
    // Keep the original vector digits and separators, including after day 999.
    const first = parts[0].querySelector('[data-countdown-digit]');
    while (parts[0].querySelectorAll('[data-countdown-digit]').length < days.length) {
      parts[0].prepend(first.cloneNode(true));
    }
    const digits = [...clock.querySelectorAll('[data-countdown-digit]')];
    const text = days + values.slice(1).map(n => String(n).padStart(2, '0')).join('');
    digits.forEach((digit, index) => {
      digit.dataset.countdownDigit = index;
      const value = text[index];
      if (digit.dataset.value === value) return;
      digit.dataset.value = value;
      digit.querySelector('use').setAttribute('href', '#payathon-digit-' + value);
    });
    clock.setAttribute('aria-label', '已建站 ' + values[0] + ' 天 ' + values[1] + ' 小时 ' + values[2] + ' 分 ' + values[3] + ' 秒');
    timeout = setTimeout(update, 1000 - (Date.now() - launch) % 1000 + 5);
  }

  const since = clock.dataset.siteLaunched;
  clock.title = '首次部署至服务器 · ' + since.slice(0, 10) + ' ' + since.slice(11, 19) + ' UTC+8';
  clock.closest('.registration-deadline').querySelector('.deadline-label').textContent = '自 ' + since.slice(0, 10).replaceAll('-', '.');
  update();
  const observer = new IntersectionObserver(([entry]) => { visible = entry.isIntersecting; update(); });
  observer.observe(clock.closest('.payathon-hero'));
  document.addEventListener('visibilitychange', update);
  window.addEventListener('pagehide', () => clearTimeout(timeout));
  window.addEventListener('pageshow', update);
}
