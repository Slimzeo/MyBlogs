# MyBlogs

处理这个仓库的博客预览、文章导入、访问故障、推送或上线时，使用[项目技能 hypnos-blog](.agents/skills/hypnos-blog/SKILL.md)。其中的脚本和操作说明随当前分支一起更新。

## 页面头图

修改导航、正文或项目卡片时，保留页面原有的照片头图。局部草图只用于用户点名的区域。Projects 沿用后台的 `theme_page_banner`，轮播放在头图下方。用户明确要求改头图时再调整它。

验收 Projects 时，从不带片段的 `/projects` 打开桌面和手机首屏，确认配置的照片已加载且可见，再点头图的向下箭头进入轮播。

项目卡片在后台「项目卡片」中维护。封面直接引用项目 GitHub 仓库当前的官方 Logo，由访客浏览器加载。Seekino 展示带本项目 Logo 的「平台体验」和带 GitHub 标志的「GitHub」，catendar 只展示「GitHub」。简介和关联文章均可留空。等作者发表技术文章后，再在后台关联到卡片。
