# 官网全站视觉一致性 QA

## Comparison Target

- 视觉基准：首页暖白、橙棕、深咖啡体系。
- 首页基准截图：`/Users/lxy/works/salesyyp/.audit/website-theme-2026-08-02/18-home-desktop-final.png`
- 动态列表：`/Users/lxy/works/salesyyp/.audit/website-theme-2026-08-02/19-news-desktop-final.png`
- 动态详情：`/Users/lxy/works/salesyyp/.audit/website-theme-2026-08-02/20-article-desktop-final.png`
- 版权介绍：`/Users/lxy/works/salesyyp/.audit/website-theme-2026-08-02/21-copyright-desktop-final.png`
- 合并对照：`/Users/lxy/works/salesyyp/.audit/website-theme-2026-08-02/22-theme-comparison.png`
- 手机端：首页、动态列表、动态详情、版权介绍与菜单截图保存在同一审查目录的 `13` 至 `17` 号文件。
- 桌面截图：1129 × 635 px，约 1129 × 635 CSS px，72 dpi，无密度归一化。
- 手机测试：请求 390 × 844 CSS px；应用内浏览器实际内容捕获为 335 × 725 px，72 dpi。所有手机页面在同一浏览器尺寸下比较。
- 状态：首页、动态列表、动态详情、版权介绍的初始首屏；手机菜单包含展开状态。

## Findings

- 最终没有遗留 P0、P1 或 P2 问题。
- 字体与层级：所有页面统一使用宋体风格展示标题、无衬线正文和同一字号层级；移动端标题没有单字掉行。
- 间距与布局：四个公开路由共享相同页头高度、页面边距、内容宽度、圆角、细边框与轻阴影。
- 颜色与视觉令牌：旧版深绿、荧光黄页面已替换为首页的暖白、浅杏、橙棕和深咖啡色。
- 图片质量：页头直接复用商户运营系统的真实 `tanban-icon.png`，未用文字、CSS 图形或近似图标代替；动态默认封面改为现有暖色产品图。
- 文案与内容：导航命名统一为“产品能力 / 适用门店 / 产品动态 / 关于摊伴”，动态列表与详情使用一致的“摊伴动态”表达。
- 响应式与交互：手机菜单可以展开，动态卡片可进入详情页，四个公开路由无横向溢出或内容遮挡。
- 浏览器运行检查：使用新标签依次访问四个路由，最终控制台无 error。

## Comparison History

- Pass 1 — P1：动态列表、动态详情仍使用深绿色头部与荧光黄按钮，版权介绍页仍使用深绿宣传卡，与首页暖色品牌体系明显割裂。
  - 修正：新增共享官网页头、页脚和品牌组件；重做动态列表、详情与版权介绍的页面表面、按钮、卡片和排版令牌。
- Pass 1 — P2：首页左上角仅为文字字标，未使用商户运营系统中的真实图形 Logo。
  - 修正：直接复用 `apps/merchant-web/src/assets/brand/tanban-icon.png`，并在所有公开页面显示同一 Logo 组合。
- Pass 2 — P2：390 px 手机检查中，首页主标题出现“套”单字掉行。
  - 修正：将手机端主标题收敛到 40 px，并微调字距。
- Pass 2 — P2：部分内页在浏览器截图中因透明模糊页头的合成方式出现页头内容间歇性缺失。
  - 修正：内页页头改为不透明暖白背景并关闭背景模糊，保留首页的吸顶效果。
- Pass 3 — 合并对照中无遗留 P0/P1/P2 问题。

## Implementation Checklist

- [x] 首页使用真实摊伴 Logo
- [x] 动态列表统一首页视觉体系
- [x] 动态详情统一首页视觉体系
- [x] 版权介绍统一首页视觉体系
- [x] 桌面端四路由逐页截图检查
- [x] 手机端四路由逐页截图检查
- [x] 手机菜单展开验证
- [x] 动态卡片进入详情页验证
- [x] 浏览器控制台无错误
- [x] 官网构建通过

## Focused Evidence

- Logo、页头、色彩与标题层级在合并对照中尺寸清晰，无需额外局部裁剪。
- 手机标题和菜单另有独立聚焦截图，分别为 `13-home-mobile-final.png` 与 `14-mobile-menu-final.png`。

## Follow-up Polish

- P3：正式上线前为动态文章补充每篇独立的运营封面，避免长期复用产品功能图。

final result: passed
