"""Generate the local home page without modifying the reference distribution."""
from html.parser import HTMLParser
from hashlib import sha256
from pathlib import Path
import json
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
# Each key is the accent in one native creature palette. Keep its other inks,
# and use the same Huashu accent for the lettering, Chinese motto and tide.
# https://github.com/alchaincyf/huashu-design/blob/master/references/design-styles.md
huashu_accents = {
    '#60b5ff': '#4EA7FC',  # Glassmorphism Bento, blue
    '#ffe29b': '#F4D35E',  # Editorial Reader, highlighter yellow
    '#bd9bff': '#B59AFF',  # Glassmorphism Bento, violet
    '#9baaff': '#7C7CFF',  # Dark Hairline Terminal, indigo
    '#ff9c83': '#59D499',  # Command Palette, jade
    '#ff97c6': '#EC4899',  # Utility-First Colorful Docs, pink
    '#ffb06f': '#FF7A2D',  # Playful Tactile, orange
    '#ffd18b': '#9BE0CC',  # Gentle Encouragement, mint
    '#f8d581': '#C9A24A',  # Shanghai Moderne Deco, brass
    '#ffa17a': '#FF6363',  # Command Palette, coral
    '#7faaff': '#38BDF8',  # Utility-First Colorful Docs, sky
    '#efd0a0': '#F4D0A8',  # Gentle Encouragement, peach
}
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
    command = r'(title:"让 Agent 帮我读本站",hint:"复制阅读指令",)success:"[^"]+",command:"(?:[^"\\]|\\.)*"'
    module, count = re.subn(command, r'\g<1>success:blogReadingCopied,command:blogReadingPrompt', module)
    product_action = ',{spm:"products",title:"选择产品接入",hint:"5 个产品",kind:"products"}'
    if count != 1 or module.count(product_action) != 1:
        raise ValueError("Check the reading command and guide menu against the updated reference module")
    module = module.replace(product_action, "")
    # The campaign already suppresses the separate support widget at its original pathname.
    # On the Go homepage, use the same page's HTML id so the widget stays off at '/' as well.
    support_guard = r'/^\/community\/events\/payathon-2026(?:\/|$)/.test(location.pathname)||document.body.append('
    if module.count(support_guard) != 1:
        raise ValueError("Check the campaign's support-widget guard")
    module = module.replace(support_guard, 'document.documentElement.hasAttribute("data-blog-site")||' + support_guard)
    # Do not initialize the original account/chat launcher when the blog navigation loads.
    launcher = r'import\{o as (\w+)\}from"\./AssistantLauncherScript\.[^"]+\.js";'
    module, count = re.subn(launcher, r'const \1=copyBlogReadingPrompt;', module)
    if count != 1:
        raise ValueError("Check the navigation's assistant import")
    # Page links (including repeated links in the footer) use the blog's own
    # descriptions; the native guide still handles motion, sizing and focus.
    module = replace_once(module, 'if(e.dataset.guideMessage)return e.dataset.guideMessage;',
                          'const hint=blogGuideMessage(e);if(hint)return hint;')
    module = replace_once(module, 'if(o.includes("about"))return"AI付团队介绍";', '')
    module = replace_once(module, 'document.documentElement.matches("#payathon-page, #payathon-center")',
                          'document.documentElement.hasAttribute("data-blog-site")')
    # The generated module lives next to our adapter; its other imports stay in the pinned distribution.
    module = re.sub(r'"\./([\w.-]+\.js)"', r'"/assets/site/_astro/\1"', module)
    digest = sha256((theme_root / "static/user/js/agent-reader.js").read_bytes()).hexdigest()[:12]
    adapter = f'import {{blogReadingPrompt,blogReadingCopied,copyBlogReadingPrompt,blogGuideMessage}} from "/user/js/agent-reader.js?v={digest}";\n'
    (ROOT / "assets/agent-topbar.js").write_text(adapter + module)
    return '<script type="module" src="/assets/blog/agent-topbar.js"></script>'


def replace_once(source, old, new):
    if source.count(old) != 1:
        raise ValueError(f"Scene bundle differs from the pinned reference near {old!r}")
    return source.replace(old, new)


def home_palette(markup):
    # Retain the reference's pre-paint draw and its palette index for the scene.
    original = json.dumps(list(huashu_accents), separators=(',', ':'))
    accents = json.dumps(list(huashu_accents.values()), separators=(',', ':'))
    markup = replace_once(markup, f'const accents={original};', f'const accents={accents};')
    set_accent = 'root.style.setProperty("--payathon-accent",accents[index]);'
    choose_letters = f'''
const letters = {json.dumps(brand)};
// Each bit selects one white letter. Exclude the all-accent and all-white masks.
const whiteMask = 1 + Math.floor(Math.random() * (2 ** letters.length - 2));
for (let position = 0; position < letters.length; position++) {{
    if (whiteMask & (1 << position)) {{
        root.style.setProperty("--blog-title-ink-" + letters[position], "#fff");
    }}
}}
'''
    return replace_once(markup, set_accent, set_accent + choose_letters)


def scene_bundle(url):
    if not url.startswith("/assets/site/_astro/"):
        raise ValueError("Expected a local scene module")
    return (ROOT.parent / "dist" / url.lstrip("/")).read_text()


def write_generated_module(name, source):
    directory = ROOT / "assets/generated"
    directory.mkdir(exist_ok=True)
    data = source.encode()
    (directory / name).write_bytes(data)
    return f'/assets/blog/generated/{name}?v={sha256(data).hexdigest()[:12]}'


def with_tide_contacts(module):
    digest = sha256((ROOT / 'assets/tide-contact.js').read_bytes()).hexdigest()[:12]
    return f'import {{contactsFor,pointerRadius}} from "/assets/blog/tide-contact.js?v={digest}";\n' + module


def tide_script(url):
    # Share the footer's native tide renderer. Each surface has its own visibility,
    # pointer, canvas fallback and reduced-motion state.
    module = replace_once(scene_bundle(url),
        'let N;function O(){N?.();const i=document.querySelector("[data-tide-footer]");if(!i)return;',
        'function mountTide(i){const bodies=i.matches(".payathon-hero")?contactsFor(i):null;')
    module = replace_once(module, 'i.addEventListener("pointerleave",L),N=()=>',
                          'i.addEventListener("pointerleave",L);return ()=>')
    # The photograph is the CSS background. Both renderers paint only the
    # wave glyphs; WebGL's compositor expects premultiplied RGB and true alpha.
    module = replace_once(module, 'getContext("webgl",{alpha:!1', 'getContext("webgl",{alpha:!0')
    module = replace_once(module, 'e.clearColor(20/255,20/255,20/255,1)', 'e.clearColor(0,0,0,0)')
    module = replace_once(module, 'e.blendFunc(e.SRC_ALPHA,e.ONE_MINUS_SRC_ALPHA)',
                          'e.blendFuncSeparate(e.SRC_ALPHA,e.ONE_MINUS_SRC_ALPHA,e.ONE,e.ONE_MINUS_SRC_ALPHA)')
    module = replace_once(module, 'M.getContext("2d",{alpha:!1})', 'M.getContext("2d")')
    module = replace_once(module, 'a&&(a.fillStyle="#141414",a.fillRect(0,0,y,h))',
                          'a&&a.clearRect(0,0,y,h)')
    # The native solver advances the spring and the pointer first. Project the
    # remaining points against the moving bodies before either backend draws.
    module = replace_once(module, 'if(u<60){const E=60-u,', 'if(u<pointerRadius){const E=pointerRadius-u,')
    module = replace_once(module, 'for(const d of p){const S=t.x+8',
                          'if(bodies)bodies.repel(t,o>0,_);for(const d of p){const S=t.x+8')
    module = replace_once(module, 'c[T+6]=R.homeX}', 'c[T+6]=R.homeX,c[T+7]=R.tideHidden?0:1}')
    module = replace_once(module, '/max(opacity,.00001);', '/max(opacity,.00001);opacity*=effect.w;')
    module = replace_once(module, 'if(e)continue;const n=40+', 'if(e||t.tideHidden)continue;const n=40+')
    # The reduced-motion renderer paints on demand, including when the first
    # geometry arrives after its initial static draw.
    module = replace_once(module, 'V.observe(i);',
                          'if(bodies)bodies.stillFrame=()=>{if(v.matches&&U&&!C&&!document.hidden)D()};V.observe(i);')
    module = replace_once(module, 'return ()=>{C=!0,cancelAnimationFrame(x)',
                          'return ()=>{if(bodies)bodies.stillFrame=null;C=!0,cancelAnimationFrame(x)')
    # The accent signal is independent from its gray underlay. Preserve the full
    # original underlay anywhere the accent contributes a visible output level.
    idle_ink, lit_threshold = '0.012', '0.001'
    module = replace_once(module, 'float base=.18*fade',
        f'float neutral=mix({idle_ink},.18,smoothstep(0.,{lit_threshold},active));float base=neutral*fade')
    module = replace_once(module, 'a.globalAlpha=.18*g',
        f'a.globalAlpha=({idle_ink}+(.18-{idle_ink})*R(0,{lit_threshold},I*g))*g')
    wave_start, wave_full = '0.06', '0.76'
    module = replace_once(module, 'max(wave*.72,effect.y*.98)',
                          f'max(smoothstep({wave_start},{wave_full},wave),effect.y*.98)')
    module = replace_once(module, 'Math.max(B*.72,t.energy*.98)',
                          f'Math.max(R({wave_start},{wave_full},B),t.energy*.98)')
    mount_all = '''
let N;
function O() {
    N?.();
    const surfaces = document.querySelectorAll(".payathon-hero, [data-tide-footer]");
    const cleanups = Array.from(surfaces, mountTide);
    N = () => cleanups.forEach(cleanup => cleanup?.());
}
'''
    init = 'O();document.addEventListener("astro:page-load",O);'
    module = replace_once(module, init, mount_all + init)
    return f'<script type="module" src="{write_generated_module("tide.js", with_tide_contacts(module))}"></script>'


def creature_script(url):
    entry = scene_bundle(url)
    match = re.search(r'import\("(\./payathon-jellyfish-dark\.[\w-]+\.js)"\)', entry)
    if not match:
        raise ValueError("Missing native creature module in the pinned reference")
    relative = match.group(1)
    module = scene_bundle("/assets/site/_astro/" + relative[2:])
    for original, accent in huashu_accents.items():
        module = replace_once(module, f'"{original}"', f'"{accent}"')
    # The native scene normally picks the most saturated animal ink on mount.
    # Use the page's chosen accent so that loading the scene cannot change its theme.
    entry = replace_once(entry, 'const _=J(e),S=',
        'const _=document.documentElement.style.getPropertyValue("--payathon-accent")||J(e),S=')
    # Keep the SVG creature simulation, pointer reactions and palette callback.
    # The tide supplies the water, so skip wake sampling and the original particle draw.
    module = replace_once(module, 'darkBackground:1,jellyCount:2', 'darkBackground:1,jellyCount:2,oceanEnabled:0')
    module = replace_once(module, 'n.style.background=Dt(a).background,', '')
    water_canvas = module.index('const s=document.createElement("canvas");', module.index('function ia('))
    creature_svg = module.index('const c=document.createElementNS(Vt,"svg");', water_canvas)
    module = module[:water_canvas] + module[creature_svg:]
    module = replace_once(module, 'n.append(s,c)', 'n.append(c)')
    module = replace_once(module, ',m={current:s}', '')
    module = replace_once(module, 'w=new ra(s),', '')
    module = replace_once(module, 'w.dispose(),', '')
    start = module.index('const lt=b.frame(')
    end = module.index('for(;j.length<W.length;)', start)
    module = module[:start] + module[end:]
    # Keep the model's original contour order through its glyph sort and rig
    # deformation. Open crab legs and procedural tentacles stay separate strokes.
    module = replace_once(module, 'function He(e,t){const', 'function He(e,t){let tidePath=null;const')
    module = replace_once(module, 's.push({x:h,y:o,char:r,alpha:f,part:u,',
        's.push({tideEdge:tidePath&&{id:tidePath.id,order:tidePath.order++,stroke:tidePath.stroke},x:h,y:o,char:r,alpha:f,part:u,')
    module = replace_once(module, 'n.contours.forEach((h,o)=>i(h,!1,c(o)))',
        'n.contours.forEach((h,o)=>{const part=c(o);tidePath={id:o,order:0,stroke:part.startsWith("crab-leg-")};i(h,!1,part)}),tidePath=null')
    module = replace_once(module, 'char:b.char,alpha:b.alpha*B,size:n.fontSize,exitBlock:X',
        'tideEdge:b.tideEdge,char:b.char,alpha:b.alpha*B,size:n.fontSize,exitBlock:X')
    module = replace_once(module, 'w=(v,E,X,B="#308cff",L=1,P)=>', 'w=(v,E,X,B="#308cff",L=1,P,tideEdge=bellEdge)=>')
    module = replace_once(module, 'y.push({x:v,y:at,char:X,color:B,', 'y.push({tideEdge,x:v,y:at,char:X,color:B,')
    module = replace_once(module, 'length:Math.min(a.fontSize*.9,Math.hypot(X,ot)/B*.8)})',
        'length:Math.min(a.fontSize*.9,Math.hypot(X,ot)/B*.8)},{id:v,order:L,stroke:true})')
    module = 'const bellEdge={id:"bell"};\n' + module
    module = replace_once(module, 'let x=[...t.avoidElements??[]]',
        'const bodies=contactsFor(e.closest(".payathon-hero"));let x=[...t.avoidElements??[]]')
    module = replace_once(module, 'W.forEach((A,N)=>{const R=A.subject??0;',
        'bodies.begin(o);W.forEach((A,N)=>{const R=A.subject??0;')
    module = replace_once(module, 'let K=j[N];const kt=J.line?', 'bodies.add(J);let K=j[N];const kt=J.line?')
    module = replace_once(module, 'K.setAttribute("opacity",String(J.alpha))})},k=()=>',
        'K.setAttribute("opacity",String(J.alpha))});bodies.finish()},k=()=>')
    module = replace_once(module, 'destroy(){ot||(ot=!0,cancelAnimationFrame(ut)',
        'destroy(){ot||(ot=!0,bodies.clear(),cancelAnimationFrame(ut)')
    creature_url = write_generated_module("creatures.js", with_tide_contacts(module))
    entry = replace_once(entry, f'"{relative}"', f'"{creature_url}"')
    # Once the opening wordmark has faded, creatures can cross its former position.
    entry = replace_once(entry, 'const e=n.hasAttribute("data-intro-complete")?[r,...n.querySelectorAll("[data-overview-reveal]")]:[r];',
        'const e=getComputedStyle(r).opacity==="0"?[]:[r];if(n.hasAttribute("data-intro-complete"))e.push(...n.querySelectorAll("[data-overview-reveal]"));')
    entry = replace_once(entry, 'b=e=>{e.animationName==="overview-arrive"',
        'b=e=>{if(e.target===r&&e.animationName==="blog-wordmark-out")F();e.animationName==="overview-arrive"')
    entry = re.sub(r'"\./([\w.-]+\.js)"', r'"/assets/site/_astro/\1"', entry)
    return f'<script type="module" src="{write_generated_module("hero-scene.js", entry)}"></script>'


def terminal_script(url):
    # The static frame should introduce the same first seven lines as the live terminal.
    module = replace_once(scene_bundle(url), 'w.slice(-7).forEach((e,t)=>{const u=A(w.length-7+t);',
                          'w.slice(0,7).forEach((e,t)=>{const u=A(t);')
    # Give the native typer the same Python token colors as the fallback homepage.
    start = module.index('w=i.map(')
    end = module.index(',M=w.map(', start)
    module = module[:start] + r'w=i.map(line=>homeCodeTokens(line.textContent.replace(/\u00a0/g,"")))' + module[end:]
    module = re.sub(r'"\./([\w.-]+\.js)"', r'"/assets/site/_astro/\1"', module)
    module = 'import {homeCodeTokens} from "/user/js/home-code.js?v=1";\n' + module
    return f'<script type="module" src="{write_generated_module("terminal.js", module)}"></script>'


def adapt_home(source, theme_root):
    # Read the established blog copy directly from its theme at regeneration time.
    original = (theme_root / "static/user/js/home.js").read_text()
    lines = ast.literal_eval(re.search(r"var ABOUT = (\[.*?\n    \]);", original, re.S).group(1))
    terminal_lines = []
    for line in lines:
        terminal_lines.append('<div class="terminal-line" data-astro-cid-l7ur5rbf="">' +
                              escape(line or "\u00a0", quote=False) + "</div>")
    terminal = "".join(terminal_lines)
    parsed = PageMarkup(source)
    changes = []
    for node in parsed.elements:
        if any(node.has_class(c) for c in ("payathon-aipay-logo", "payathon-subtitle",
                                           "overview-description-end", "contents-brand-year",
                                           "overview-blackhole")):
            changes.append((node.start, node.end, ""))
        elif node.has_class("payathon-hero") and node.tag == "section":
            changes.append((node.inner_start, node.inner_start, '<canvas class="blog-hero-tide" aria-hidden="true"></canvas>'))
        elif node.tag == "header" and node.has_class("compactTopBar"):
            changes.append((node.start, node.end, "<!--BLOG_TOPBAR-->"))
        elif node.tag == "script" and node.attrs.get("src") == "/assets/site/site-theme.js":
            changes.append((node.start, node.end, '<script src="/user/js/color-scheme.js?v=1"></script>'))
        elif node.tag == "script" and any(key in source[node.inner_start:node.inner_end] for key in
                                          ("window.__releaseSiteMenuBound", "window.__releasePathBarBound")):
            changes.append((node.start, node.end, ""))
        elif node.tag == "template" and "data-terminal-source" in node.attrs:
            changes.append((node.inner_start, node.inner_end, terminal))
        elif node.tag == "button" and node.has_class("payathon-button") and "data-registration-open" in node.attrs:
            scope = " ".join(f'{key}=""' for key in node.attrs if key.startswith("data-astro-cid-"))
            contents = source[node.inner_start:node.inner_end].replace("立即报名", "阅读文章")
            changes.append((node.start, node.end, f'<a class="payathon-button" href="#blogs" {scope}>{contents}</a>'))
        elif node.tag == "script" and "script_index_3_lang." in node.attrs.get("src", "") and "/payathon-2026.astro_" in node.attrs["src"]:
            # The campaign timer hardcodes the signup deadline. The blog clock owns this hook.
            changes.append((node.start, node.end, '<script type="module" src="/assets/blog/hero.js"></script>'))
        elif node.tag == "script" and "/ReleaseTopBar.astro_" in node.attrs.get("src", ""):
            changes.append((node.start, node.end, blog_topbar(node.attrs["src"])))
        elif node.tag == "script" and "/TideFooter.astro_" in node.attrs.get("src", ""):
            changes.append((node.start, node.end, tide_script(node.attrs["src"])))
        elif node.tag == "script" and "/HeroTerminal.astro_" in node.attrs.get("src", ""):
            changes.append((node.start, node.end, terminal_script(node.attrs["src"])))
        elif node.tag == "script" and "/payathon-2026.astro_astro_type_script_index_1_lang." in node.attrs.get("src", ""):
            changes.append((node.start, node.end, creature_script(node.attrs["src"])))
        elif node.tag == "title":
            changes.append((node.inner_start, node.inner_end, "<!--BLOG_TITLE-->"))
        elif node.tag == "link" and node.attrs.get("rel") == "icon":
            changes.append((node.start, node.end, '<link rel="icon" href="/user/img/favicon.png"><link rel="apple-touch-icon" href="/user/img/apple-touch-icon.png">'))
        elif node.tag == "meta" and node.attrs.get("name") == "description":
            changes.append((node.start, node.end, '<meta name="description" content="Hypnos 的赛博日记：日记摘录、读书笔记和实践记录。">'))
    for start, end, value in sorted(changes, reverse=True):
        source = source[:start] + value + source[end:]
    replacements = {
        'data-theme-lock="dark"': 'data-blog-site',
        'if (restored) return true;': 'if (restored || location.hash === "#blogs") return true;',
        '让你的产品</span>': '<span class="motto-lead"><span>如果这个宇宙的结局是注定的，</span><span>是消亡，是归于死寂；</span><span>或许这更说明，</span></span></span>',
        '走向真实生意</span>': '<span class="motto-main">只有<b>过程</b>才是<b>最重要的</b></span></span>',
        'aria-label="PAYATHON 命令行"': 'aria-label="关于我，以 Python 写成"',
        'aria-label="正在计算报名倒计时"': 'aria-label="已建站时长"',
        'title="报名截止：2026/12/01 23:59:00（UTC+8）"': 'data-site-launched="2026-07-20T00:01:52+08:00"',
        '>报名中</span>': '>已建站</span>',
        '>距报名截止</span>': '>运行时长</span>',
        'aria-label="PAYATHON 2026"': 'aria-label="HYPNOS"',
    }
    for old, new in replacements.items():
        source = source.replace(old, new)
    source = source.replace('</head>', '<link rel="stylesheet" href="/assets/blog/home.css"></head>', 1)
    return source.replace('</body>', '<!--BLOG_DIALOGS--></body>', 1)


def journal_home(markup, *, with_intro):
    """Use one feed slot for every page. Only page one mounts the original scene."""
    nodes = PageMarkup(markup).elements

    def one(test):
        node, = (node for node in nodes if test(node))
        return node

    main = one(lambda n: n.tag == "main")
    hero = one(lambda n: n.tag == "section" and n.has_class("payathon-hero"))
    tide = one(lambda n: n.tag == "section" and "data-tide-footer" in n.attrs)
    shell = one(lambda n: n.has_class("footerScrollShell"))
    changes = [(hero.end if with_intro else main.inner_start, main.inner_end, "<!--BLOG_FEED-->"),
               (tide.start, tide.end, ""), (shell.start, shell.end, "")]
    if not with_intro:
        header = one(lambda n: "data-payathon-header" in n.attrs)
        start = markup[header.start:header.inner_start]
        ready = 'data-title-ready="true" data-scene-ready="true" data-ready="true"'
        changes.append((header.start, header.inner_start,
                        replace_once(start, 'inert="" aria-hidden="true"', ready)))
        defs = one(lambda n: n.tag == "svg" and n.attrs.get("width") == "0" and n.attrs.get("height") == "0")
        changes.append((defs.start, defs.end, ""))
    for node in nodes:
        if main.start <= node.start < main.end or shell.start <= node.start < shell.end:
            continue
        if node.tag == "script":
            src = node.attrs.get("src", "")
            script = markup[node.inner_start:node.inner_end]
            campaign = (src == "/assets/campaign-actions.js" or "/NavigationQueryScript." in src
                        or "initAipayChannelQuery" in script or "[data-footer-scroll-shell]" in script)
            scene = ("/payathon-2026.astro_astro_type_script_index_" in src or "/assets/blog/generated/" in src
                     or "layoutPayathonContent" in script)
            if campaign or (scene and not with_intro):
                changes.append((node.start, node.end, ""))
        elif node.tag == "link" and any(name in node.attrs.get("href", "") for name in
                                       ("FooterClosingCard.", "WatchaAssets.", "/community.", "NavigationQueryScript.")):
            changes.append((node.start, node.end, ""))
    for start, end, value in sorted(changes, reverse=True):
        markup = markup[:start] + value + markup[end:]
    if with_intro:
        markup = replace_once(markup, '</head>',
                              '<link rel="preload" as="image" href="/user/img/forest.jpg" fetchpriority="high" data-forest-source><script src="/user/js/forest-intro.js?v=1" defer></script></head>')
    else:
        markup = replace_once(markup, 'id="payathon-page"', 'id="payathon-page" data-journal-page')
    return markup


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

def wordmark(white_letters=(), *, animated=False):
    output = []
    # The native title animator measures its reveal in a 606-unit viewport.
    # Scale both axes so its final frame and the resting word have the same center.
    scale = 606 / brand_width if animated else 1
    for i, key in enumerate(brand):
        color = '#FFFFFF' if key in white_letters else 'var(--payathon-accent, #1677FF)'
        if animated:
            color = f'var(--blog-title-ink-{key}, {color})'
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
        inner = wordmark(brand if logo else 'OS')
    patches.append((start, end, opening + inner + '</svg>'))
if len(patches) != 6:
    raise ValueError(f'Expected six source wordmarks, found {len(patches)}')
for start, end, value in sorted(patches, reverse=True):
    source = source[:start] + value + source[end:]
# The original title module exports its per-glyph pointer effect, including teardown.
glyph_url, = (node.attrs["src"] for node in parser.elements if node.tag == "script"
              and "/payathon-2026.astro_astro_type_script_index_4_lang." in node.attrs.get("src", ""))
if not glyph_url.startswith("/assets/site/_astro/"):
    raise ValueError("Expected a local glyph module")
source = replace_once(source, 'id="payathon-page"', f'id="payathon-page" data-journal-glyphs="{escape(glyph_url)}"')
source = adapt_home(home_palette(source), theme_root)
source = source.replace('<head>', '<head><script src="/assets/blog/shim.js"></script>', 1)
digest = sha256((theme_root / 'static/user/css/journal.css').read_bytes()).hexdigest()[:12]
source = source.replace('</head>', f'<link rel="stylesheet" href="/user/css/journal.css?v={digest}"></head>', 1)

def version_asset(match):
    filename = match.group(1)
    digest = sha256((ROOT / "assets" / filename).read_bytes()).hexdigest()[:12]
    return f'"/assets/blog/{filename}?v={digest}"'

# Inner pages load only the two local styles used by the optional animated guide.
nav_styles = [node.attrs['href'] for node in PageMarkup(source).elements if node.tag == 'link'
              and any(name in node.attrs.get('href', '') for name in ('/ReleaseTopBar.', '/compact-navigation.'))]
if len(nav_styles) != 2:
    raise ValueError("Missing local guide styles")
(ROOT / 'assets/generated/guide.css').write_text(''.join(f'@import url("{href}");\n' for href in nav_styles))
source = source.replace('</head>', '<link rel="stylesheet" href="/user/css/topbar.css?v=9">'
                        '<script src="/user/js/topbar.js?v=2" defer></script></head>', 1)
source = re.sub(r'"/assets/blog/([\w.-]+\.(?:css|js))"', version_asset, source)
for name, intro in [('index.html', True), ('journal.html', False)]:
    (ROOT / name).write_text(journal_home(source, with_intro=intro))
(ROOT / 'assets/brand.svg').write_text(f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {brand_width} 37">' + wordmark(brand) + '</svg>')
print('Generated journal pages and local scene assets from unchanged dist files')
