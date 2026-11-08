/* Home effects. The navbar bot is global; everything else mounts when the
   home hero is on the page and tears itself down when a client-side
   navigation swaps the hero out (footer.html calls HomeFX.mount on every page). */
(function () {
    'use strict';
    var reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    var $ = function (id) { return document.getElementById(id); };
    var cleanups = [];
    function on(target, type, fn, opts) { target.addEventListener(type, fn, opts); cleanups.push(function () { target.removeEventListener(type, fn, opts); }); }
    function esc(s) { return String(s).replace(/[&<>"]/g, function (c) { return {'&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;'}[c]; }); }
    function shuffle(a) { for (var i = a.length - 1; i > 0; i--) { var j = (Math.random() * (i + 1)) | 0; var t = a[i]; a[i] = a[j]; a[j] = t; } return a; }

    /* ---------------- the square bot (top left, every page) ---------------- */
    var botReady = false;
    function mountBot() {
        if (botReady) return;
        var bot = $('hx-bot'), home = $('hx-bot-home'), bubble = $('hx-bubble'), menu = $('hx-bot-menu'), inner = bot && bot.parentElement;
        if (!bot || !home) return;
        botReady = true;
        var typing = 0, back = 0, at = null;
        function place(x, y, instant) {
            var cur = bot._x == null ? x : bot._x;
            bot.classList.toggle('to-left', x < cur);
            if (!instant && !reduce && Math.abs(x - cur) > 8) {
                bot.classList.add('is-moving');
                window.clearTimeout(bot._mv);
                bot._mv = window.setTimeout(function () { bot.classList.remove('is-moving'); }, 420);
            }
            bot._x = x; bot._y = y;
            bot.style.transform = 'translate(' + x + 'px,' + y + 'px)';
        }
        function homePos() { var r = inner.getBoundingClientRect(), h = home.getBoundingClientRect(); return [h.left - r.left, h.top - r.top]; }
        function goHome(instant) { var p = homePos(); place(p[0], p[1], instant); }
        function say(text, x, y) {
            window.clearInterval(typing);
            bubble.style.transform = 'translate(' + (x + 36) + 'px,' + y + 'px)';
            bubble.textContent = '';
            bubble.classList.add('is-on');
            bot.classList.add('is-talking');
            var n = 0;
            typing = window.setInterval(function () {
                n++; bubble.textContent = text.slice(0, n);
                if (n >= text.length) { window.clearInterval(typing); bot.classList.remove('is-talking'); }
            }, reduce ? 0 : 45);
        }
        function quiet() { window.clearInterval(typing); bubble.classList.remove('is-on'); bot.classList.remove('is-talking'); }
        goHome(true);
        window.addEventListener('resize', function () { if (!at) goHome(true); });
        if (document.fonts && document.fonts.ready) document.fonts.ready.then(function () { if (!at) goHome(true); });

        // follow the hovered nav item and type what it leads to
        inner.addEventListener('pointerover', function (e) {
            var link = e.target.closest && e.target.closest('.fluid-navbar-menu > a[data-hint]');
            if (!link || link === at || window.innerWidth < 768) return;
            window.clearTimeout(back);
            if (at) at.classList.remove('is-hinted');
            at = link; link.classList.add('is-hinted');
            var r = inner.getBoundingClientRect(), l = link.getBoundingClientRect();
            var x = l.left - r.left + l.width / 2 - 14, y = l.bottom - r.top + 12;
            place(x, y);
            window.setTimeout(function () { if (at === link) say(link.dataset.hint, x, y); }, reduce ? 0 : 380);
        });
        inner.addEventListener('pointerleave', function () {
            back = window.setTimeout(function () { if (at) at.classList.remove('is-hinted'); at = null; quiet(); if (menu.hidden) goHome(); }, 500);
        });

        // its menu
        function toggleMenu(open) {
            menu.hidden = !open;
            bot.setAttribute('aria-expanded', open ? 'true' : 'false');
            if (open) {
                quiet(); goHome();
                var p = homePos(); menu.style.left = Math.max(0, p[0] - 8) + 'px';
                var pool = document.querySelectorAll('#hx-pool, .hx-latest');
                var links = [];
                pool.forEach(function (t) { t.content.querySelectorAll('a').forEach(function (a) { links.push(a.getAttribute('href')); }); });
                $('hx-random').href = links.length ? links[(Math.random() * links.length) | 0] : '/topics';
            }
        }
        bot.addEventListener('click', function (e) { e.stopPropagation(); toggleMenu(menu.hidden); });
        document.addEventListener('click', function (e) { if (!menu.hidden && !menu.contains(e.target)) toggleMenu(false); });
        document.addEventListener('keydown', function (e) { if (e.key === 'Escape' && !menu.hidden) { toggleMenu(false); bot.focus(); } });
        $('hx-search').addEventListener('click', function () {
            toggleMenu(false);
            var t = $('search-toggle'); if (t) t.click();
        });

        // first visit: a short binary rain falls from the bot
        if (!reduce) {
            var rain = document.createElement('div');
            rain.className = 'hx-rain';
            var p = homePos();
            rain.style.left = (p[0] - 6) + 'px';
            for (var c = 0; c < 7; c++) {
                var col = document.createElement('span'), txt = '';
                for (var k = 0; k < 6; k++) txt += (Math.random() < .3 ? '|' : Math.random() < .5 ? '1' : '0') + '\n';
                col.textContent = txt;
                col.style.animationDelay = (c * 70 + Math.random() * 120) + 'ms';
                rain.appendChild(col);
            }
            inner.appendChild(rain);
            window.setTimeout(function () { rain.remove(); }, 2200);
        }
    }

    /* ---------------- hovered characters glitch ---------------- */
    function splitGlyphs(root) {
        root.querySelectorAll('[data-glitch]').forEach(function (el) {
            if (el.dataset.split) return;
            el.dataset.split = '1';
            var walker = document.createTreeWalker(el, NodeFilter.SHOW_TEXT), nodes = [];
            while (walker.nextNode()) nodes.push(walker.currentNode);
            nodes.forEach(function (node) {
                var frag = document.createDocumentFragment();
                Array.from(node.textContent).forEach(function (ch) {
                    if (/\s/.test(ch)) { frag.appendChild(document.createTextNode(ch)); return; }
                    var s = document.createElement('span'); s.className = 'hx-ch'; s.textContent = ch; frag.appendChild(s);
                });
                node.replaceWith(frag);
            });
        });
        if (reduce) return;
        on(root, 'pointerover', function (e) {
            var ch = e.target.closest && e.target.closest('.hx-ch');
            if (!ch || ch.classList.contains('g')) return;
            ch.classList.add('g');
            window.setTimeout(function () { ch.classList.remove('g'); }, 460);
        });
    }

    /* ---------------- about me, typed as Python (loops, scrolls like a terminal) ---------------- */
    var ABOUT = [
        '$ python -i about.py',
        'class Hypnos(Idealist):',
        '    """Gets up to speed on anything new, fast."""',
        '    interests = ["mathematics"]',
        '    dream     = "help create digital life"',
        '    style     = "AI-native, unconventional"',
        '    studying  = "LLMs, in depth"',
        '',
        '    def harness(self):               # self-harness-system',
        '        rsi  = track(self.life)      # see which changes help',
        '        gold = open(world).collect() # people, cases, golden set',
        '        RL(gold).reinforce(self)     # keep what works',
        '        return explore("ME.md")      # chaos gremlin, for fun',
        '',
        '    modules   = [Express, ProjectManagement, ...]  # more to come',
        '    exploring = {"streaming LLMs", "agents", "ML", "math"}',
        '',
        '>>> Hypnos().harness()',
        '# ▶ 只有过程才是最重要的'
    ];
    function highlight(line) {
        return line.replace(/(#.*$)|("""[^"]*"""|"[^"]*")|\b(class|def|return|self)\b|\b(Hypnos|Idealist|Express|ProjectManagement|RL)\b|(>>>|\$)|([^#"\w$>]+|\w+|>)/g, function (m, c, s, k, n, p) {
            var cls = c ? 'c' : s ? 's' : k ? 'k' : n ? 'n' : p ? 'p' : '';
            return cls ? '<span class="' + cls + '">' + esc(m) + '</span>' : esc(m);
        });
    }
    function mountTyper(pre) {
        var narrow = window.innerWidth < 768;
        var lines = narrow ? ABOUT.reduce(function (out, l) {
            var m = l.match(/^(\s*)(\S.*?)\s{2,}(#.*)$/);
            return out.concat(m ? [m[1] + m[3], m[1] + m[2]] : [l]);
        }, []) : ABOUT;
        var keep = narrow ? 7 : 9;
        function render(li, ci) {
            var shown = lines.slice(0, li + 1).map(function (l, i) { return i === li ? l.slice(0, ci) : l; });
            shown = shown.slice(-keep);
            pre.innerHTML = shown.map(highlight).join('\n') + '<span class="hx-cur"></span>';
        }
        if (reduce) { render(lines.length - 1, Infinity); return; }
        var li = 0, ci = 0, timer;
        function step() {
            var line = lines[li];
            if (ci < line.length) {
                ci += /^\s+$/.test(line.slice(0, ci + 4)) ? 4 : 1;
                render(li, ci);
                timer = window.setTimeout(step, 14 + Math.random() * 30);
            } else if (li < lines.length - 1) {
                li++; ci = 0; render(li, 0);
                timer = window.setTimeout(step, 160);
            } else {
                timer = window.setTimeout(function () { li = 0; ci = 0; step(); }, 7000);
            }
        }
        timer = window.setTimeout(step, 900);
        cleanups.push(function () { window.clearTimeout(timer); });
    }

    /* ---------------- time since the last post ---------------- */
    function mountClock(hero) {
        var last = parseInt(hero.dataset.lastPost, 10);
        if (!last) return;
        var days = $('hx-days'), time = $('hx-time');
        function pad(n, w) { n = String(n); while (n.length < w) n = '0' + n; return n; }
        function tick() {
            var s = Math.max(0, Math.floor(Date.now() / 1000) - last);
            days.textContent = pad(Math.floor(s / 86400), 3);
            time.textContent = pad(Math.floor(s % 86400 / 3600), 2) + ':' + pad(Math.floor(s % 3600 / 60), 2) + ':' + pad(s % 60, 2);
        }
        tick();
        var id = window.setInterval(tick, 1000);
        cleanups.push(function () { window.clearInterval(id); });
    }

    /* ---------------- the night forest, the dot halo and the bird ---------------- */
    function rng(seed) { var a = seed >>> 0; return function () { a = a + 0x6D2B79F5 | 0; var t = Math.imul(a ^ a >>> 15, 1 | a); t = t + Math.imul(t ^ t >>> 7, 61 | t) ^ t; return ((t ^ t >>> 14) >>> 0) / 4294967296; }; }
    var PINE = ['#a9d6bf', '#6fae8c', '#3f8063'];
    var AUTUMN = ['#e2a54b', '#d98030', '#c9583c', '#e8c35a'];
    function drawTrees(specs, seed) {
        var R = rng(seed), grid = new Map(), trees = [];
        specs.forEach(function (s) {
            var t = Object.assign({}, s, {cells: [], crown: [], bend: 0, vb: 0, ph: R() * 6.28, col: AUTUMN[(R() * 4) | 0], col2: AUTUMN[(R() * 4) | 0]});
            function put(gx, gy, ch, color) { if (ch !== ' ' && gy >= 0) grid.set(gy * 4096 + gx, {x: gx, y: gy, ch: ch, color: color, a: s.a, t: t, k: (t.base - gy) / t.h}); }
            var x = t.x, base = t.base, h = t.h;
            if (t.kind === 'pine') {
                var tl = h > 12 ? 4 : 3;
                put(x, base - h, '^', PINE[t.L]);
                for (var i = 1; i < h - 2; i++) {
                    var w = Math.round(.6 + Math.floor(i / tl) * (h > 12 ? 1.05 : .8) + (i % tl) * .8);
                    for (var dx = -w; dx <= w; dx++) {
                        var r = R();
                        put(x + dx, base - h + i, dx === -w ? '/' : dx === w ? '\\' : r < .66 ? '^' : r < .76 ? '1' : r < .84 ? '0' : r < .9 ? '*' : ' ', PINE[t.L]);
                    }
                }
                for (var y = base - 2; y <= base; y++) put(x, y, '|', '#7a6a4c');
            } else {
                var rr = t.r, cy = base - Math.round(h * .3) - rr;
                for (var dy = -rr; dy <= rr; dy++) for (var ex = -Math.round(rr * 2.1); ex <= Math.round(rr * 2.1); ex++) {
                    var d = Math.hypot(ex / 2.1, dy) / rr + (R() - .5) * .3;
                    if (d > 1) continue;
                    put(x + ex, cy + dy, d < .6 ? '@%&#*'[(R() * 5) | 0] : d < .88 ? '*o0%'[(R() * 4) | 0] : ".,'"[(R() * 3) | 0], R() < .65 ? t.col : t.col2);
                    if (d < .9) t.crown.push([x + ex, cy + dy]);
                }
                for (var yy = cy + rr; yy <= base; yy++) put(x, yy, yy === cy + rr ? 'Y' : '|', '#7a6a4c');
            }
            trees.push(t);
        });
        var cells = Array.from(grid.values());
        cells.forEach(function (c) { c.t.cells.push(c); });
        return {cells: cells, trees: trees};
    }
    // a hill of trees on each side, leaving the centre column to the words
    function sideSpecs(cols, rows, ground, narrow) {
        var R = rng(7), specs = [], hgt = rows * (narrow ? .3 : .52);
        [[0, .26], [.74, 1]].forEach(function (side, si) {
            var x0 = cols * side[0], x1 = cols * side[1], span = x1 - x0;
            for (var x = x0 - 2 + R() * 2; x < x1 + 2; x += 2.4 + R() * 2.4) {
                var rise = si === 0 ? 1 - (x - x0) / span : (x - x0) / span;
                specs.push({kind: 'pine', x: Math.round(x), base: Math.round(ground - hgt * (.5 + .3 * rise)), h: Math.round(3 + R() * 3), L: 0, a: .34});
            }
            var plan = si === 0 ? [[.1, 'pine', 1], [.32, 'broad', .55], [.55, 'pine', .78], [.8, 'pine', .5]]
                                : [[.2, 'pine', .5], [.42, 'pine', .8], [.66, 'broad', .6], [.9, 'pine', 1]];
            plan.forEach(function (p) {
                var x = Math.round(x0 + span * p[0] + (R() - .5) * 2), h = Math.round(hgt * .9 * p[2]);
                if (h < 5) return;
                specs.push(p[1] === 'pine' ? {kind: 'pine', x: x, base: ground, h: h, L: p[2] > .7 ? 2 : 1, a: .95}
                                           : {kind: 'broad', x: x, base: ground, h: h, r: Math.max(2, Math.round(h * .26)), L: 1, a: 1});
            });
        });
        return specs;
    }
    var BIRD = [
        ['\\_  0  _/', "  '-1-'  "],
        ['  _ 0 _  ', "_/ '1' \\_"]
    ];
    function mountForest(hero) {
        var cv = $('hx-forest'), ctx = cv.getContext('2d'), cta = $('hx-cta'), foot = hero.querySelector('.hx-foot');
        var W, H, cw, chh, F, raf, visible = true, last = 0, t0 = performance.now(), mx = -1e4, my = -1e4;
        var leaves = [], halo = [], bird = {x: -120, y: 0, phase: 0, burst: 0};
        function fsz() { return W < 768 ? 10 : 13; }
        function size() {
            var r = hero.getBoundingClientRect(), dpr = Math.min(2, window.devicePixelRatio || 1);
            W = r.width; H = r.height; cv.width = W * dpr; cv.height = H * dpr; ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
            ctx.font = fsz() + 'px ui-monospace, Menlo, monospace';
            cw = ctx.measureText('M').width; chh = Math.round(fsz() * 1.18);
            var cols = Math.ceil(W / cw), rows = Math.ceil(H / chh), fr = foot.getBoundingClientRect();
            var ground = Math.floor((fr.top - r.top - 12) / chh);
            F = drawTrees(sideSpecs(cols, rows, ground, W < 768), 7);
            F.groups = {};
            F.cells.forEach(function (c) { (F.groups[c.color + '|' + c.a] = F.groups[c.color + '|' + c.a] || []).push(c); });
            // the halo: rings of dots around the call to action
            var cr = cta.getBoundingClientRect(), cx = cr.left - r.left + cr.width / 2, cy = cr.top - r.top + cr.height / 2;
            halo = [];
            var rings = W < 768 ? 9 : 14;
            for (var k = 1; k <= rings; k++) {
                var rx = 18 + k * (W < 768 ? 11 : 15), ry = 14 + k * (W < 768 ? 9 : 12), n = Math.round(rx * .55);
                for (var j = 0; j < n; j++) { var a = j / n * 6.283; halo.push({x: cx + Math.cos(a) * rx, y: cy + Math.sin(a) * ry, k: k / rings}); }
            }
            bird.y = H * .2;
        }
        function leaf(x, y, color, burst) {
            if (leaves.length > 80) return;
            leaves.push({x: x, y: y, vx: burst ? burst + (Math.random() - .5) * 1.4 : (Math.random() - .5) * .3, vy: burst ? -Math.random() * 1.4 : 0,
                ch: "*,'`"[(Math.random() * 4) | 0], c: color, life: 1, sway: Math.random() * 6.28, fs: 13 + Math.random() * 4});
        }
        function shed(t, n, burst) { for (var i = 0; i < n && t.crown.length; i++) { var p = t.crown[(Math.random() * t.crown.length) | 0]; leaf(p[0] * cw, p[1] * chh, Math.random() < .6 ? t.col : t.col2, burst); } }
        function frame(now) {
            var t = (now - t0) / 1000, dt = Math.min(3, (now - (last || now)) / 16.7) || 1; last = now;
            ctx.clearRect(0, 0, W, H);
            if (!F) return;
            ctx.textBaseline = 'top';
            // halo dots lean toward the cursor
            ctx.fillStyle = '#c99a58';
            halo.forEach(function (d) {
                var dx = mx - d.x, dy = my - d.y, dist = Math.hypot(dx, dy), pull = dist < 220 ? (1 - dist / 220) * 18 : 0;
                var wob = reduce ? 0 : Math.sin(t * 1.2 + d.k * 9) * 1.5;
                ctx.globalAlpha = .12 + (1 - d.k) * .3 + (pull ? .25 : 0);
                ctx.fillRect(d.x + (dist ? dx / dist * pull : 0) + wob, d.y + (dist ? dy / dist * pull : 0), 1.6, 1.6);
            });
            ctx.font = fsz() + 'px ui-monospace, Menlo, monospace';
            F.trees.forEach(function (tr) {
                var px = tr.x * cw, midY = (tr.base - tr.h * .55) * chh, d = px - mx;
                var near = Math.exp(-(d * d) / (2 * 140 * 140)) * Math.exp(-Math.pow(midY - my, 2) / (2 * 220 * 220));
                tr.vb += ((Math.sign(d || 1) * near * (tr.L + 1) * 5 - tr.bend) * .06 - tr.vb * .16) * dt; tr.bend += tr.vb * dt;
                if (!reduce && tr.kind === 'broad') { if (near > .4 && Math.random() < .14 * dt) shed(tr, 1); if (Math.random() < .012 * dt) shed(tr, 1); }
            });
            var tick = (t * 8) | 0;
            Object.keys(F.groups).forEach(function (key) {
                var parts = key.split('|');
                ctx.fillStyle = parts[0]; ctx.globalAlpha = +parts[1];
                F.groups[key].forEach(function (c) {
                    var k = Math.max(0, c.k), px = c.x * cw + (reduce ? 0 : c.t.bend * k * k + Math.sin(t * .8 + c.t.ph) * 1.1 * k * k), py = c.y * chh, ch = c.ch;
                    if (!reduce && Math.abs(px - mx) < 48 && Math.abs(py - my) < 36 && (c.x * 31 + c.y * 17 + tick) % 3 === 0) ch = (c.x + c.y + tick) % 2 ? '1' : '0';
                    ctx.fillText(ch, px, py);
                });
            });
            // the bird crosses the sky; touched, it breaks into pixel blocks and pulls itself together
            if (!reduce) {
                bird.x += 1.1 * dt; bird.phase += .09 * dt;
                if (bird.x > W + 120) { bird.x = -120; bird.y = H * (.12 + Math.random() * .2); }
                var by = bird.y + Math.sin(bird.x / 90) * 18, shape = BIRD[(bird.phase | 0) % 2];
                if (Math.hypot(mx - (bird.x + 4.5 * cw), my - by) < 60 && bird.burst <= 0) bird.burst = 1;
                bird.burst = Math.max(0, bird.burst - .018 * dt);
                shape.forEach(function (row, ry) {
                    Array.from(row).forEach(function (ch, rx) {
                        if (ch === ' ') return;
                        var gx = bird.x + rx * cw, gy = by + ry * chh, b = bird.burst;
                        if (b > 0) {
                            var sx = Math.sin(rx * 12.9 + ry * 78.2) * 40 * b, sy = Math.cos(rx * 4.1 + ry * 9.7) * 30 * b;
                            ctx.globalAlpha = .9; ctx.fillStyle = '#f1ecdf';
                            ctx.fillRect(gx + sx, gy + sy, cw * .8, chh * .7);
                        } else {
                            ctx.globalAlpha = .9; ctx.fillStyle = /\d/.test(ch) ? '#e2a54b' : '#d58aa8';
                            ctx.fillText(ch, gx, gy);
                        }
                    });
                });
            }
            // leaves drift, sway, get pushed by the cursor, settle and fade
            for (var j = leaves.length - 1; j >= 0; j--) {
                var l = leaves[j], ldx = l.x - mx, ldy = l.y - my, d2 = ldx * ldx + ldy * ldy;
                if (d2 < 12100) { var dd = Math.sqrt(d2 + 1), f = (1 - dd / 110) * .35; l.vx += ldx / dd * f; l.vy += ldy / dd * f * .6; }
                l.sway += .035 * dt; l.vy = Math.min(.85, l.vy + .016 * dt); l.vx *= .97;
                l.x += (l.vx + Math.sin(l.sway) * .55) * dt; l.y += l.vy * dt;
                if (l.y > H * .9) l.life -= .02 * dt;
                if (l.life <= 0 || l.y > H || l.x < -20 || l.x > W + 20) { leaves.splice(j, 1); continue; }
                ctx.globalAlpha = l.life; ctx.fillStyle = l.c;
                ctx.font = '600 ' + l.fs + 'px ui-monospace, Menlo, monospace';
                ctx.save(); ctx.translate(l.x, l.y); ctx.rotate(Math.sin(l.sway) * .8); ctx.fillText(l.ch, 0, 0); ctx.restore();
            }
            ctx.globalAlpha = 1;
            if (!reduce && visible) raf = window.requestAnimationFrame(frame);
        }
        function redraw() { window.cancelAnimationFrame(raf); last = 0; frame(performance.now()); }
        size(); redraw();
        if (document.fonts && document.fonts.ready) document.fonts.ready.then(function () { if (hero.isConnected) { size(); redraw(); } });
        on(window, 'resize', function () { size(); redraw(); });
        on(hero, 'pointermove', function (e) { var r = hero.getBoundingClientRect(); mx = e.clientX - r.left; my = e.clientY - r.top; if (reduce) redraw(); });
        on(hero, 'pointerleave', function () { mx = my = -1e4; });
        // tap: a gust from where you touched
        on(hero, 'pointerdown', function (e) {
            if (reduce || !F) return;
            var x = e.clientX - hero.getBoundingClientRect().left;
            F.trees.forEach(function (tr) {
                var d = tr.x * cw - x, near = Math.exp(-(d * d) / (2 * 280 * 280));
                tr.vb += Math.sign(d || 1) * near * (tr.L + 1) * 2.4;
                if (tr.kind === 'broad' && near > .25) shed(tr, 6, Math.sign(d || 1) * 2.2 * near);
            });
        });
        var io = new IntersectionObserver(function (en) { visible = en[0].isIntersecting; redraw(); });
        io.observe(hero);
        cleanups.push(function () { io.disconnect(); window.cancelAnimationFrame(raf); visible = false; });
    }

    /* ---------------- random walk: the wheel scrubs through random posts ---------------- */
    function mountWalk() {
        var strip = $('hx-strip'), row = $('hx-strip-row'), pool = $('hx-pool'), dial = $('hx-dial');
        if (!strip || !pool) return;
        var posts = shuffle(Array.from(pool.content.querySelectorAll('a'))).slice(0, 16);
        if (!posts.length) { strip.hidden = true; return; }
        var html = posts.map(function (a) {
            return '<a class="hx-post" href="' + esc(a.getAttribute('href')) + '"><strong>' + esc(a.textContent) + '</strong><small><b>' + esc(a.dataset.cat) + '</b> · ' + esc(a.dataset.date) + '</small></a>';
        }).join('');
        while (posts.length < 6) posts = posts.concat(posts);
        row.innerHTML = html + html + html;
        var base = reduce ? 0 : .5, impulse = 0, x = 0, raf, last = 0;
        var knob = $('hx-dial-knob'), mark = $('hx-dial-mark');
        function show(v) {
            var p = (Math.max(-3, Math.min(3, v)) + 3) / 6 * 100;
            mark.style.left = p + '%'; knob.style.left = p + '%';
            dial.setAttribute('aria-valuenow', v.toFixed(1));
        }
        function loop(now) {
            var dt = Math.min(3, (now - (last || now)) / 16.7) || 1; last = now;
            impulse *= Math.pow(.94, dt);
            var v = Math.max(-3, Math.min(3, base + impulse));
            var w = row.scrollWidth / 3;
            x -= v * 1.6 * dt;
            if (x <= -w) x += w; if (x > 0) x -= w;
            row.style.transform = 'translateX(' + x + 'px)';
            show(v);
            raf = window.requestAnimationFrame(loop);
        }
        raf = window.requestAnimationFrame(loop);
        on(strip, 'wheel', function (e) {
            e.preventDefault();
            var delta = Math.abs(e.deltaY) > Math.abs(e.deltaX) ? e.deltaY : e.deltaX;
            impulse = Math.max(-6, Math.min(6, impulse + delta * .012));
            if (reduce) { x -= delta; }
        }, {passive: false});
        function fromPointer(e) { var r = dial.getBoundingClientRect(); base = Math.max(-3, Math.min(3, ((e.clientX - r.left) / r.width) * 6 - 3)); impulse = 0; }
        on(dial, 'pointerdown', function (e) { dial.setPointerCapture(e.pointerId); fromPointer(e); });
        on(dial, 'pointermove', function (e) { if (dial.hasPointerCapture(e.pointerId)) fromPointer(e); });
        on(dial, 'keydown', function (e) {
            if (e.key === 'ArrowLeft' || e.key === 'ArrowRight') { e.preventDefault(); base = Math.max(-3, Math.min(3, base + (e.key === 'ArrowRight' ? .5 : -.5))); }
        });
        cleanups.push(function () { window.cancelAnimationFrame(raf); });
    }

    /* ---------------- paths: categories as lanes ---------------- */
    var LANE = ['#8fb8d8', '#d8cfb8', '#e2a54b', '#c9583c', '#7fc79a'];
    function scramble(el, text) {
        if (reduce) { el.textContent = text; return; }
        var glyphs = '01/\\^*#%', n = 0, id = window.setInterval(function () {
            n++;
            el.textContent = Array.from(text).map(function (ch, i) { return i < n * text.length / 8 ? ch : glyphs[(Math.random() * glyphs.length) | 0]; }).join('');
            if (n >= 8) { window.clearInterval(id); el.textContent = text; }
        }, 40);
    }
    function mountPaths() {
        var labels = document.querySelectorAll('#hx-labels .hx-label'), lanes = document.querySelectorAll('#hx-floor .hx-lane'), card = $('hx-card');
        if (!labels.length) return;
        var current = -1;
        lanes.forEach(function (lane, i) {
            lane.style.setProperty('--c', LANE[i % LANE.length]);
            var bar = lane.querySelector('i');
            bar.style.setProperty('--d', (4.5 + (i * 1.7) % 4) + 's');
            bar.style.setProperty('--dl', (-i * 1.3) + 's');
        });
        labels.forEach(function (l, i) { l.style.setProperty('--c', LANE[i % LANE.length]); });
        function activate(i) {
            if (i === current) return;
            current = i;
            var l = labels[i], c = LANE[i % LANE.length], name = l.dataset.name;
            labels.forEach(function (x, j) { x.classList.toggle('on', j === i); });
            lanes.forEach(function (x, j) { x.classList.toggle('on', j === i); });
            card.style.setProperty('--c', c);
            card.parentElement.style.setProperty('--c', c);
            card.classList.add('is-swapping');
            window.setTimeout(function () { card.classList.remove('is-swapping'); }, 260);
            scramble($('hx-card-name'), name);
            scramble($('hx-card-desc'), l.dataset.desc || (l.dataset.count + ' 篇公开文章，悬停或点击跑道进入'));
            var tpl = document.querySelector('.hx-latest[data-cat="' + name.replace(/"/g, '\\"') + '"]');
            var list = $('hx-card-list');
            list.innerHTML = tpl ? tpl.innerHTML : '';
            var first = list.querySelector('a');
            $('hx-side-count').textContent = l.dataset.count + ' 篇';
            $('hx-side-date').textContent = first ? first.dataset.date : '—';
            $('hx-card-go').href = l.getAttribute('href');
        }
        labels.forEach(function (l, i) {
            on(l, 'pointerenter', function () { activate(i); });
            on(l, 'focus', function () { activate(i); });
        });
        lanes.forEach(function (lane, i) { on(lane, 'pointerenter', function () { activate(i); }); });
        activate(0);
    }

    function mount() {
        mountBot();
        while (cleanups.length) cleanups.pop()();
        var hero = document.querySelector('#banner.hx-hero');
        if (!hero) return;
        splitGlyphs(hero);
        mountTyper($('hx-code'));
        mountClock(hero);
        mountForest(hero);
        mountWalk();
        mountPaths();
    }
    window.HomeFX = {mount: mount};
})();
