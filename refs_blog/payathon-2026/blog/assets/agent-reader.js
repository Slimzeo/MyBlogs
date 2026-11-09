// A remote reader needs the public blog address when this page is previewed on loopback.
const localPreview = ['localhost', '127.0.0.1', '[::1]'].includes(location.hostname);
const blogOrigin = localPreview ? 'https://www.hypn0s.cloud' : location.origin;
export const blogReadingPrompt =
  '请从 HYPNOS 博客的文章归档 ' + new URL('/archives', blogOrigin).href +
  ' 开始，按我感兴趣的主题找到相关文章。阅读原文，概括作者的观点和实践经验，附上对应文章的链接。';

export async function copyBlogReadingPrompt() {
  await navigator.clipboard.writeText(blogReadingPrompt);
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
      notice.textContent = '已复制阅读指令，粘贴给 Agent 即可';
    } catch {
      notice.textContent = '复制失败，请允许浏览器访问剪贴板后重试';
    }
    notice.classList.add('is-visible');
    hideNotice = setTimeout(() => {
      notice.classList.remove('is-visible');
    }, 3200);
  });
}
