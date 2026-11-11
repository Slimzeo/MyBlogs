# Local reference home

To restore this version in a new checkout, place the saved `payathon-2026.zip`
in the repository root temporarily, then run the following commands from the root.
The imported reference files also work offline. If `refs_blog/payathon-2026/dist`
is already present, just run the generator.

```sh
echo "fcd2362c67df7b3a6cadf32ad2c5766fc8035ebeb161c2ef1c60678b9c56c9db  payathon-2026.zip" | shasum -a 256 -c - &&
  unzip -oq payathon-2026.zip 'payathon-2026/dist/*' -d refs_blog &&
  python3 -I refs_blog/payathon-2026/blog/transform.py
```

The generator uses the pinned reference and the blog theme's Python introduction
to write `blog/index.html` (intro and feed), `blog/journal.html` (later pages), `assets/brand.svg`, `assets/agent-topbar.js` and the
local scene modules in `assets/generated/`.
The Go handler reads the generated files on request.

Bindings:
- The HYPNOS Cloud wordmark links to the homepage. The chat bot has its own button beside it. The header and centered motto use the Fluid/Bootstrap system sans-serif stack. The opening wordmark uses the letter outlines. The tide canvas is composited at 48% opacity, with .012 gray ink at rest and .18 beneath lit crests. In the hero, visible body contours push the tide glyphs aside using the same spring as the pointer. The Python introduction comes from the blog theme.
- The elapsed clock starts at the server's first recorded successful clone (MyBlogs/.git/logs/HEAD), 2026-07-20 00:01:52 UTC+8. Its vector digits, units and date label render together at half scale.
- Desktop and mobile share Home, Projects, Archives, Categories and About. Projects opens the author's project descriptions and repository links. One Go topbar template provides the links, current tab and optional access-key control for all public layouts. The same theme switch uses the saved Fluid reading preference. Search opens with the button or Cmd/Ctrl+K and uses /search/:keyword.
- The animated chat bot uses the locally bundled ReleaseTopBar, glyphs, and styles. Its reading action copies the public blog address and instructions for an AI reader. Each English navigation item has a Chinese description, also shown in the compact menu. The generator switches the copy command to the journal feed and removes the payment-product menu action. Compact screens offer the same command on tap.
- The homepage is a chronological journal: an electronic résumé through diary excerpts, reading notes and work logs. Keep the animated entrance, followed by the Fluid article index on a single board. Its down arrow targets the index, and article titles reuse the opening scene’s local glyph interaction. Blogs (/blogs) skips the intro and opens the same chronology. Categories (/categories) opens the column index. Tags (/tags) opens the cross-column topic index. Column pages can filter by tag. Old /notes links retain their selection. Each article belongs to one column and may have multiple tags. Do not add profile or project showcases, featured articles, a section picker or a second animated ending.
- `templates/theme/journal.html` and `static/user/css/journal.css` define the feed shared by the generated and fallback home pages. Go sends only the authorized page of entries as HTML, newest display date first. Page size stays consistent; subsequent pages go straight to writing. Navigation settings take effect in the server-rendered links.
- The AI import path remains the existing draft-only blogctl/API workflow. Only published entries are listed publicly.

Network isolation:
- Before original scripts run, fetch requests to other origins return inert JSON.
- External XMLHttpRequest URLs are redirected to an inert data URL; external sendBeacon calls are discarded; external Image.src assignments receive a local data placeholder.
- A home-only Content-Security-Policy blocks all remaining external scripts, image/setAttribute/SVG/CSS beacons, connections, fonts, workers and frames, including telemetry, monitoring, session/login and gallery endpoints on alipay.com, alipay.net, alipayobjects.com, aipay hosts and act-protocol.com.
- The device-id storage key is removed and its getItem/setItem operations are intercepted.
