# 官网视觉还原 QA

## Comparison Target

- 参考图：`/var/folders/86/dyvpwsyn2t5_gptz5kjm4d0h0000gn/T/codex-clipboard-2049008c-1d89-4be2-8ea4-cfdb3f823b89.png`
- 桌面首屏：`/Users/lxy/works/salesyyp/.qa/official-warm-hero-final.png`
- 产品能力：`/Users/lxy/works/salesyyp/.qa/official-warm-product.png`
- 场景与动态：`/Users/lxy/works/salesyyp/.qa/official-warm-scenes.png`
- 联系与页脚：`/Users/lxy/works/salesyyp/.qa/official-warm-contact.png`
- 手机端：`/Users/lxy/works/salesyyp/.qa/official-warm-mobile.png`
- 手机菜单：`/Users/lxy/works/salesyyp/.qa/official-warm-mobile-menu.png`
- 合并对照：`/Users/lxy/works/salesyyp/.qa/official-warm-comparison.png`
- 桌面检查视口：1440 × 1000 CSS px
- 手机检查视口：390 × 844 CSS px

## Findings

- 没有遗留 P0、P1 或 P2 级视觉问题。
- 品牌基调与参考图一致：暖白背景、橙棕主色、深咖啡正文、中文宋体标题、轻阴影卡片。
- 首屏结构、双 CTA、功能图标组和设备主视觉均保持参考图的视觉层级。
- 三段产品能力采用交错图文结构，桌面端保持宽幅卡片，手机端顺序折叠为单列。
- 五类经营场景、三条官网动态、品牌介绍与联系模块均完整呈现。
- 客服二维码区域在未配置真实二维码时显示品牌占位图标；后台上传后会直接替换为真实二维码。
- 手机菜单、页面锚点与客服电话链接已实际点击或渲染验证。
- 浏览器控制台最终检查无 error。

## Comparison History

- Pass 1 — P2：首屏主标题在 1440 px 视口下被拆成四行，弱化了参考图的两段式标题节奏。
  - 修正：在中文逗号后设置明确换行，并收敛桌面标题字号。
- Pass 2 — 无遗留 P0、P1、P2 问题。

## Verification

- [x] 桌面首屏视觉检查
- [x] 产品能力与经营场景检查
- [x] 动态、联系区和页脚检查
- [x] 390 px 手机端布局检查
- [x] 手机菜单交互检查
- [x] 浏览器控制台检查
- [x] 参考图与实现合并对照

## Follow-up Polish

- P3：正式上线前上传真实客服微信二维码，并用正式客服电话替换演示数据。
- P3：内容运营可在后台继续替换首屏、三段产品能力和五类场景图片。

final result: passed
