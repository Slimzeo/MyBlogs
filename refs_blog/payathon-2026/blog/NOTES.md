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
to write `blog/index.html`, `assets/brand.svg` and `assets/agent-topbar.js`.
The Go handler reads the generated files on request.

Bindings:
- Branding uses the HYPNOS letter outlines, including O. The hero retains the native scene and its noninteractive depth target. The quote and typed Python introduction are from the original blog theme.
- The elapsed clock starts at the server's first recorded successful clone (MyBlogs/.git/logs/HEAD), 2026-07-20 00:01:52 UTC+8. It uses the original vector digits.
- Desktop and mobile share the original blog destinations for Blogs, Notes, Archives, Friends and About. The Go handler supplies module settings. Search opens with the button or Cmd/Ctrl+K and uses /search/:keyword.
- The Agent-friendly badge and animated reading bubble use the locally bundled ReleaseTopBar, glyphs, and styles. The generator switches the copy command to the blog archive and removes the payment-product menu action. Compact screens offer the same command on tap.
- Registration actions: showcase; track creation actions: category URL; product introductions: topics; gallery more link: archives.
- Partner strip: tags as bold links in the original slots.
- Rewards: published article count and category ranks 1–5; amounts: top-post hits with category-count fallback.
- Track panels: ranked category names/descriptions, latest-post links in delivery/capability lists.
- Gallery: latest nine visible posts via the original works response shape; title, excerpt, category badge, hits, comment count, local cover or original placeholder, article links. Original card creation and animation modules are retained.
- Calendar: four months of WritingDay values, original phase colors, month/count date slots.
- Benefits carousel: most-read post category, headline and article actions.
- Partner wall: friend names/URLs, new-tab noopener links; organizer: HYPN0S cloud/about.
- Footer: categories and blog navigation; site title/description and the theme's attribution link; article/comment totals and latest display time.

Network isolation:
- Before original scripts run, fetch requests to other origins return inert JSON. `/assets/works.json` returns the current request's blog data without a network request.
- External XMLHttpRequest URLs are redirected to an inert data URL; external sendBeacon calls are discarded; external Image.src assignments receive a local data placeholder.
- A home-only Content-Security-Policy blocks all remaining external scripts, image/setAttribute/SVG/CSS beacons, connections, fonts, workers and frames, including telemetry, monitoring, session/login and gallery endpoints on alipay.com, alipay.net, alipayobjects.com, aipay hosts and act-protocol.com.
- The device-id storage key is removed and its getItem/setItem operations are intercepted.
- Capture-phase registration/login handlers use blog destinations before the original campaign/account handlers. Login hrefs and benefit button copy are restored after module initialization.

Limitations for lead review:
- External article cover URLs use the original placeholder to satisfy offline operation; they are not downloaded.
- Empty category slots are hidden. Tag and friend lists clone their existing item components when more slots are needed.
