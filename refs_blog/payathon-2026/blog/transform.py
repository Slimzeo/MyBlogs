"""Generate the local home page without modifying the reference distribution."""
from html.parser import HTMLParser
from hashlib import sha256
from pathlib import Path
import re
import sys

from dataclasses import dataclass
from html import escape
import ast

ROOT = Path(__file__).resolve().parent
source = (ROOT.parent / 'dist/community/events/payathon-2026/index.html').read_text()
theme_root = Path(sys.argv[1]) if len(sys.argv) > 1 else ROOT.parents[2]
brand = 'HYPNOS'
brand_width = (len(brand) - 1) * 66 + 62

@dataclass
class Element:
    tag: str
    attrs: dict
    start: int
    inner_start: int
    inner_end: int
    end: int

    def has_class(self, name):
        return name in self.attrs.get("class", "").split()


class PageMarkup(HTMLParser):
    """Track source ranges so unrelated SVG, Astro hooks and whitespace survive."""
    void = set("area base br col embed hr img input link meta param source track wbr".split())

    def __init__(self, document):
        super().__init__(convert_charrefs=False)
        self.document = document
        self.lines = [0] + [m.end() for m in re.finditer("\n", document)]
        self.pending = {}
        self.elements = []
        self.feed(document)
        self.close()

    def source_position(self):
        line, column = self.getpos()
        return self.lines[line - 1] + column

    def handle_starttag(self, tag, attrs):
        start = self.source_position()
        end = start + len(self.get_starttag_text())
        node = Element(tag, dict(attrs), start, end, end, end)
        if tag in self.void:
            self.elements.append(node)
        else:
            self.pending.setdefault(tag, []).append(node)

    def handle_startendtag(self, tag, attrs):
        start = self.source_position()
        end = start + len(self.get_starttag_text())
        self.elements.append(Element(tag, dict(attrs), start, end, end, end))

    def handle_endtag(self, tag):
        if self.pending.get(tag):
            node = self.pending[tag].pop()
            node.inner_end = self.source_position()
            node.end = self.document.index(">", node.inner_end) + 1
            self.elements.append(node)


def blog_topbar(script_url):
    """Keep the original badge, speech and motion code; supply the blog's reading action."""
    if not script_url.startswith("/assets/site/_astro/"):
        raise ValueError("Expected a local navigation module")
    module = (ROOT.parent / "dist" / script_url.lstrip("/")).read_text()
    command = r'(title:"让 Agent 帮我读本站",hint:"复制阅读指令",success:"[^"]+",command:)"(?:[^"\\]|\\.)*"'
    module, count = re.subn(command, r'\g<1>blogReadingPrompt', module)
    product_action = ',{spm:"products",title:"选择产品接入",hint:"5 个产品",kind:"products"}'
    if count != 1 or module.count(product_action) != 1:
        raise ValueError("Check the reading command and guide menu against the updated reference module")
    module = module.replace(product_action, "")
    # The campaign already suppresses the separate support widget at its original pathname.
    # On the Go homepage, use the same page's HTML id so the widget stays off at '/' as well.
    support_guard = r'/^\/community\/events\/payathon-2026(?:\/|$)/.test(location.pathname)||document.body.append('
    if module.count(support_guard) != 1:
        raise ValueError("Check the campaign's support-widget guard")
    module = module.replace(support_guard, 'document.documentElement.id==="payathon-page"||' + support_guard)
    # Do not initialize the original account/chat launcher when the blog navigation loads.
    launcher = r'import\{o as (\w+)\}from"\./AssistantLauncherScript\.[^"]+\.js";'
    module, count = re.subn(launcher, r'const \1=copyBlogReadingPrompt;', module)
    if count != 1:
        raise ValueError("Check the navigation's assistant import")
    # The generated module lives next to our adapter; its other imports stay in the pinned distribution.
    module = re.sub(r'"\./([\w.-]+\.js)"', r'"/assets/site/_astro/\1"', module)
    digest = sha256((ROOT / "assets/agent-reader.js").read_bytes()).hexdigest()[:12]
    adapter = f'import {{blogReadingPrompt,copyBlogReadingPrompt}} from "/assets/blog/agent-reader.js?v={digest}";\n'
    (ROOT / "assets/agent-topbar.js").write_text(adapter + module)
    return '<script type="module" src="/assets/blog/agent-topbar.js"></script>'


def adapt_home(source, theme_root):
    # Read the established blog copy directly from its theme at regeneration time.
    original = (theme_root / "static/user/js/home.js").read_text()
    lines = ast.literal_eval(re.search(r"var ABOUT = (\[.*?\n    \]);", original, re.S).group(1))
    terminal_lines = []
    for line in lines:
        kind = " blog-python-comment" if line.lstrip().startswith("#") else ""
        if line.startswith("$ "):
            kind = " blog-python-command"
        terminal_lines.append(f'<div class="terminal-line{kind}" data-astro-cid-l7ur5rbf="">' +
                              escape(line or "\u00a0", quote=False) + "</div>")
    terminal = "".join(terminal_lines)
    tabs = [("Blogs", "/", None), ("Notes", "/notes", "notes"),
            ("Archives", "/archives", "archives"), ("Friends", "/links", "links"),
            ("About", "/about", "about")]
    nav = "".join(f'<a href="{href}" ' +
                  ('aria-current="page"' if module is None else f'data-blog-module="{module}"') +
                  f' style="--nav-reveal-delay:{520 - index * 45}ms">{label}</a>'
                  for index, (label, href, module) in enumerate(tabs))
    search = '<button type="button" class="navButton navButtonDark" data-blog-search-open aria-haspopup="dialog" aria-controls="blog-search">搜索</button>'
    parsed = PageMarkup(source)
    changes = []
    for node in parsed.elements:
        if any(node.has_class(c) for c in ("payathon-aipay-logo", "payathon-subtitle",
                                           "overview-description-end", "contents-brand-year")):
            changes.append((node.start, node.end, ""))
        elif node.has_class("overview-blackhole"):
            # The scene uses this box as its depth target even without a CTA.
            anchor = '<i class="blackhole-button blog-scene-anchor" data-astro-cid-45h6wvi2="" aria-hidden="true"></i>'
            changes.append((node.inner_start, node.inner_end, anchor))
        elif node.has_class("siteMenuLinks"):
            changes.append((node.inner_start, node.inner_end, nav))
        elif node.has_class("navActions"):
            changes.append((node.inner_start, node.inner_end, search))
        elif node.has_class("brand") and node.tag == "a":
            changes.append((node.start, node.inner_start, '<a class="brand" href="/" aria-label="HYPNOS 首页">'))
        elif node.tag == "template" and "data-terminal-source" in node.attrs:
            changes.append((node.inner_start, node.inner_end, terminal))
        elif node.tag == "button" and node.has_class("payathon-button") and "data-registration-open" in node.attrs:
            scope = " ".join(f'{key}=""' for key in node.attrs if key.startswith("data-astro-cid-"))
            contents = source[node.inner_start:node.inner_end].replace("立即报名", "阅读文章")
            changes.append((node.start, node.end, f'<a class="payathon-button" href="#showcase" {scope}>{contents}</a>'))
        elif node.tag == "script" and "script_index_3_lang." in node.attrs.get("src", "") and "/payathon-2026.astro_" in node.attrs["src"]:
            # The campaign timer hardcodes the signup deadline. The blog clock owns this hook.
            changes.append((node.start, node.end, '<script type="module" src="/assets/blog/hero.js"></script>'))
        elif node.tag == "script" and "/ReleaseTopBar.astro_" in node.attrs.get("src", ""):
            changes.append((node.start, node.end, blog_topbar(node.attrs["src"])))
        elif node.tag == "title":
            changes.append((node.inner_start, node.inner_end, "HYPNOS · Blog"))
        elif node.tag == "link" and node.attrs.get("rel") == "icon":
            changes.append((node.start, node.end, '<link rel="icon" href="/user/img/favicon.png"><link rel="apple-touch-icon" href="/user/img/apple-touch-icon.png">'))
        elif node.tag == "meta" and node.attrs.get("name") == "description":
            changes.append((node.start, node.end, '<meta name="description" content="HYPNOS 的个人博客">'))
    for start, end, value in sorted(changes, reverse=True):
        source = source[:start] + value + source[end:]
    replacements = {
        '让你的产品</span>': '<span class="motto-lead">如果这个宇宙的结局是注定的，</span><span class="motto-main">是消亡，是归于死寂；</span></span>',
        '走向真实生意</span>': '<span class="motto-lead">或许这更说明，</span><span class="motto-main">只有<b>过程</b>才是<b>最重要的</b></span></span>',
        'aria-label="PAYATHON 命令行"': 'aria-label="关于我，以 Python 写成"',
        'aria-label="正在计算报名倒计时"': 'aria-label="已建站时长"',
        'title="报名截止：2026/12/01 23:59:00（UTC+8）"': 'data-site-launched="2026-07-20T00:01:52+08:00"',
        '>报名中</span>': '>已建站</span>',
        '>距报名截止</span>': '>运行时长</span>',
        'aria-label="PAYATHON 2026"': 'aria-label="HYPNOS"',
        '01 活动介绍': '01 博客介绍', '02 赛道与奖励': '02 专栏精选', '03 参赛作品': '03 最近文章',
        '04 如何参赛': '04 阅读指南', '05 评审与规则': '05 写作原则', '06 参赛权益': '06 热门文章', '07 合作伙伴': '07 友情链接',
        'data-contents-current="">活动介绍': 'data-contents-current="">博客介绍',
    }
    for old, new in replacements.items():
        source = source.replace(old, new)
    source = source.replace('</head>', '<link rel="stylesheet" href="/assets/blog/home.css"></head>', 1)
    dialog = """<dialog id="blog-search" aria-labelledby="blog-search-label">
  <form id="blog-search-form" role="search">
    <label id="blog-search-label" for="blog-search-input">搜索文章</label>
    <button type="button" data-blog-search-close aria-label="关闭搜索">×</button>
    <div><input id="blog-search-input" name="q" type="search" required autocomplete="off" autofocus><button type="submit">搜索</button></div>
  </form>
</dialog>"""
    return source.replace('</body>', dialog + '</body>', 1)

symbols = {}
for key in brand:
    name = 'letter-' + key
    match = re.search(r'<symbol id="payathon-' + name + r'"[^>]*>(.*?)</symbol>', source, re.S)
    if not match:
        raise ValueError('Missing source glyph: ' + key)
    symbols[key] = match.group(1)

def outline(key, x, scale):
    glyph = symbols[key]
    paths = re.findall(r'<path\b[^>]*>', glyph)
    result = []
    for path in paths:
        d = re.search(r' d="([^"]+)"', path).group(1)
        transform = re.search(r'translate\(([-\d.]+)', path)
        shift = float(transform.group(1)) if transform else 0
        tokens = re.findall(r'[A-Za-z]|[-+]?(?:\d*\.\d+|\d+)(?:[eE][-+]?\d+)?', d)
        output = []
        command = None
        index = 0
        for token in tokens:
            if token.isalpha():
                if token not in 'MCLHVZ':
                    raise ValueError('Unsupported outline command: ' + token)
                command = token
                index = 0
                output.append(token)
            else:
                horizontal = command == 'H' or (command != 'V' and index % 2 == 0)
                value = (float(token) + shift + x) * scale if horizontal else float(token) * scale
                output.append(format(value, '.10g'))
                index += 1
        result.append(' '.join(output))
    return ' '.join(result)

def wordmark(white=False, animated=False):
    output = []
    # The native title animator measures its reveal in a 606-unit viewport.
    # Scale both axes so its final frame and the resting word have the same center.
    scale = 606 / brand_width if animated else 1
    for i, key in enumerate(brand):
        color = '#FFFFFF' if white or i >= 4 else 'var(--payathon-accent, #1677FF)'
        hook = f' data-title-char="{i}" style="opacity:0"' if animated else ''
        output.append(f'<path{hook} d="{outline(key, i * 66, scale)}" fill="{color}"></path>')
    return ''.join(output)

parser = PageMarkup(source)
patches = []
for element in parser.elements:
    if element.tag != "svg":
        continue
    start, end, attrs = element.start, element.end, element.attrs
    chunk = source[start:end]
    logo = 'data-aipay-logo' in attrs
    hero = attrs.get('class') == 'payathon-name'
    word = attrs.get('viewbox') == '0 0 464 37' and 'M151.976 35.8' in chunk
    if not (logo or hero or word):
        continue
    opening = chunk[:chunk.index('>') + 1]
    view_box = f'0 0 606 {37 * 606 / brand_width:.6f}' if hero else f'0 0 {brand_width} 37'
    opening = re.sub(r'viewBox="[^"]*"', f'viewBox="{view_box}"', opening)
    opening = re.sub(r'aria-label="[^"]*"', f'aria-label="{brand}"', opening)
    if logo:
        opening = re.sub(r'width="[^"]*"', 'width="108"', opening)
        opening = re.sub(r'height="[^"]*"', 'height="18"', opening)
    elif word:
        opening = opening.replace('width="464"', f'width="{brand_width}"')
    if hero:
        inner = '<g data-title-glyphs="">' + wordmark(animated=True) + '</g>'
        brackets = re.findall(r'<g class="payathon-bracket-[^"]+".*?</g>', chunk, re.S)
        inner = ''.join(brackets).replace('y="-3.5"', f'y="{(37 * 606 / brand_width - 44) / 2:.5f}"') + inner
    else:
        inner = wordmark(white=logo)
    patches.append((start, end, opening + inner + '</svg>'))
if len(patches) != 6:
    raise ValueError(f'Expected six source wordmarks, found {len(patches)}')
for start, end, value in sorted(patches, reverse=True):
    source = source[:start] + value + source[end:]
source = adapt_home(source, theme_root)
source = source.replace('<head>', '<head><!--BLOG_HOME_DATA--><script src="/assets/blog/shim.js"></script>', 1)
source = source.replace('</body>', '<script src="/assets/blog/bind.js"></script></body>', 1)

def version_asset(match):
    filename = match.group(1)
    digest = sha256((ROOT / "assets" / filename).read_bytes()).hexdigest()[:12]
    return f'"/assets/blog/{filename}?v={digest}"'

source = re.sub(r'"/assets/blog/([\w.-]+\.(?:css|js))"', version_asset, source)
(ROOT / 'index.html').write_text(source)
(ROOT / 'assets/brand.svg').write_text(f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {brand_width} 37">' + wordmark(white=True) + '</svg>')
print('Generated index.html and brand.svg from unchanged dist files')
