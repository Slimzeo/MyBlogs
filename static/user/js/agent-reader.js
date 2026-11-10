// A remote reader needs the public blog address when this page is previewed on loopback.
const localPreview = ['localhost', '127.0.0.1', '[::1]'].includes(location.hostname);
const blogOrigin = localPreview ? 'https://www.hypn0s.cloud' : location.origin;
export const blogReadingPrompt =
  '请从 HYPNOS 的博客首页 ' + new URL('/#blogs', blogOrigin).href +
  ' 开始，按日期浏览作者公开的日记精华、读书笔记和实践记录。也可以从专栏目录 ' + new URL('/notes', blogOrigin).href +
  ' 按专栏或标签找原文。请附上日期和文章链接。';

export const blogReadingCopied = '已复制博客地址和阅读说明，粘贴到 AI 对话即可';

export async function copyBlogReadingPrompt() {
  await navigator.clipboard.writeText(blogReadingPrompt);
}

export function blogGuideMessage(target) {
  if (target.dataset.guideMessage) return target.dataset.guideMessage;
  const href = target.getAttribute('href');
  if (!href || !href.startsWith('/')) return '';
  const item = Array.from(document.querySelectorAll('.siteMenuLinks a[data-guide-message]'))
    .find(link => link.getAttribute('href') === href);
  return item?.dataset.guideMessage || '';
}

// On compact screens the reference hides the floating guide and does not install its click handler.
const badge = document.querySelector('[data-agent-read-entry]');
if (badge) {
  const notice = document.createElement('span');
  notice.className = 'blog-agent-notice';
  notice.setAttribute('role', 'status');
  badge.after(notice);
  let hideNotice;
  badge.addEventListener('click', async () => {
    if (document.getElementById('aipay-bubble-guide')) return;
    clearTimeout(hideNotice);
    try {
      await copyBlogReadingPrompt();
      notice.textContent = blogReadingCopied;
    } catch {
      notice.textContent = '复制失败，请允许浏览器访问剪贴板后重试';
    }
    notice.classList.add('is-visible');
    hideNotice = setTimeout(() => {
      notice.classList.remove('is-visible');
    }, 3200);
  });
}
