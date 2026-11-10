# Third-Party Notices

## hexo-theme-fluid

The public blog frontend is adapted from the visual structure of
[`hexo-theme-fluid`](./hexo-theme-fluid):

- upstream: https://github.com/fluid-dev/hexo-theme-fluid
- license: GNU General Public License v3.0
- local license copy: `hexo-theme-fluid/LICENSE`

The following asset is copied from the local Fluid source and renamed:

- `static/user/img/fluid-banner.png`
  - source: `hexo-theme-fluid/source/img/default.png`

The Go templates and `static/user/css/fluid.css` are adaptations for this
project's Go data model and routing rather than a Hexo runtime integration.

## LXGW WenKai

The public theme can load LXGW WenKai from the
[`lxgw-wenkai-webfont`](https://github.com/chawyehsu/lxgw-wenkai-webfont)
package:

- package version: `1.7.0`
- typeface license: SIL Open Font License 1.1
- webfont package license: MIT

## Source Han Serif

This repository includes HYPNOS Motto Serif, a ten-character WOFF2 subset
of Adobe's [Source Han Serif CN SemiBold 2.003](https://github.com/adobe-fonts/source-han-serif/blob/2.003R/SubsetOTF/CN/SourceHanSerifCN-SemiBold.otf).
The subset keeps the original glyph outlines and uses a new family name.
The font is distributed under the [SIL Open Font License 1.1](refs_blog/payathon-2026/blog/assets/fonts/OFL.txt).
If the headline changes, regenerate the subset to cover its new characters.

## Default banner

`static/user/img/blog-banner.jpg` is based on the Unsplash photo
["Body of water surrounding with trees"](https://unsplash.com/photos/body-of-water-surrounding-with-trees-_LuLiJc1cdo)
by [Kalen Emsley](https://unsplash.com/@kalenemsley).
