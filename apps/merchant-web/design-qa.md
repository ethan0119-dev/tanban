# 收银台视觉验收

- source visual truth path: `/Users/lxy/.codex/generated_images/019f94f0-6e76-7510-9f31-772e23fdb57e/call_KkBvOyIaPv06fV7RixyjVQUL.png`
- implementation URL: `http://127.0.0.1:4175/__preview/cashier`
- implementation screenshot path: `/Users/lxy/works/salesyyp/apps/merchant-web/design-qa-artifacts/design-qa-cashier-final.png`
- full-view comparison path: `/Users/lxy/works/salesyyp/apps/merchant-web/design-qa-artifacts/design-qa-cashier-comparison-final.png`
- focused comparison path: `/Users/lxy/works/salesyyp/apps/merchant-web/design-qa-artifacts/design-qa-cashier-focused-right.png`
- tablet evidence path: `/Users/lxy/works/salesyyp/apps/merchant-web/design-qa-artifacts/design-qa-cashier-tablet.png`
- viewport: `1487 × 1058` CSS px, `deviceScaleFactor: 1`
- source pixels: `1487 × 1058`
- implementation pixels: `1487 × 1058`
- density normalization: none required; source and implementation were compared at identical dimensions and 1× density
- state: preview fixture, 堂食点单, B03 selected, PAY_AFTER unpaid bill with two additions

## Full-view comparison evidence

The final full-view comparison confirms the selected option-two structure: 108px dark task rail, 78px header, 58.6%/41.4% table-and-operation split, four-column table grid, 87px business summary footer, and the same above-the-fold density. The semantic table state intentionally reads “待结账” instead of the mock’s “就餐中” because the implemented product requirement explicitly needs an unsettled state.

## Focused comparison evidence

The focused right-panel comparison checks readable order metadata, addition separation, item quantity/price alignment, member discount, stacked total, primary action hierarchy, and secondary table operations. A focused comparison was necessary because these dense receipt details are too small to judge reliably from the full-view composite alone.

## Required fidelity surfaces

- Fonts and typography: existing merchant-web Inter/PingFang/Microsoft YaHei stack retained; heading, table number, receipt item, amount, and small metadata weights and sizes follow the source hierarchy without clipping.
- Spacing and layout rhythm: header, rail, main split, table-card grid, operation panel, action grid, and footer align with the source. Final table/action regions have no hidden overflow at the desktop target.
- Colors and visual tokens: dark charcoal rail, burnt-orange primary mode, green/blue/orange/red operational states, white receipt surface, and warm neutral borders map to the source palette.
- Image quality and asset fidelity: the source contains no required product photography or illustration. Visible UI glyphs use the existing Ant Design icon library; no emoji, placeholder art, handcrafted SVG, or CSS-drawn product asset is used.
- Copy and content: “堂食点单 / 带走点单 / 新开桌 / 加菜 / 修改人数 / 打印客户联 / 结账 / 转台 / 并台 / 退菜” match the required cashier workflow. “待结账” is an intentional product correction.

## Comparison history

### Pass 1 — blocked

Evidence: `design-qa-artifacts/design-qa-cashier-comparison-pass1.png`

- [P2] Main board was about 31px too wide and the operation panel too narrow.
- [P2] Only 11 tables were visible, leaving the third row incomplete.
- [P2] Table elapsed time and amount collided on active cards.
- [P2] Rail item rhythm and footer height did not match the source.
- [P2] Order panel lacked addition separators and was too vertically sparse.

Fixes: changed the workspace split to 58.6%/41.4%, added the twelfth preview table, separated card amount geometry, matched rail section heights, changed the footer to 87px, and added addition dividers.

### Pass 2 — blocked

Evidence: `design-qa-artifacts/design-qa-cashier-comparison-pass2.png`

- [P2] Four area tabs produced a horizontal scrollbar.
- [P2] The total remained on one horizontal line instead of the source’s stacked label and amount.
- [P2] Primary operation icons and receipt text were visibly undersized.

Fixes: switched area tabs to equal grid tracks, stacked the total label/amount with the settlement state on the right, and increased receipt/action typography and icon sizes.

### Responsive pass — blocked, then fixed

Evidence after fix: `design-qa-artifacts/design-qa-cashier-tablet.png`

- [P2] At `1024 × 768`, receipt content pushed persistent cashier actions below the viewport.

Fix: made the receipt panel the flexible scroll region at tablet widths while keeping add, print, settle, transfer, merge, and return actions visible. Final metrics: body `scrollWidth === clientWidth`; primary actions end at y=609, secondary actions at y=669, footer begins at y=681.

### Final pass

Evidence: `design-qa-artifacts/design-qa-cashier-comparison-final.png` and `design-qa-artifacts/design-qa-cashier-focused-right.png`

No actionable P0/P1/P2 visual differences remain. Residual icon-shape differences are P3 and are expected because the implementation uses the product’s supported icon library. The “待结账” copy difference is intentional.

## Interaction and runtime checks

- Switched between 堂食点单 and 带走点单; three realistic takeout cards rendered.
- Opened the 加菜 drawer.
- Added a simple product directly.
- Opened a configured product, selected “中辣”, and verified the configured cart line.
- Checked browser logs. No application exception or failed resource was present; the existing Ant Design v5/React 19 compatibility warning appears when opening a modal.
- Verified `1024 × 768` tablet layout with no horizontal body overflow and persistent cashier actions visible.

## Findings

No actionable P0/P1/P2 findings remain.

## Follow-up polish

- [P3] A dedicated chair/table glyph could replace the closest available Ant Design table icon if the product later adds a custom icon set.
- [P3] Upgrade Ant Design to a React 19-native release to remove the existing compatibility warning.

final result: passed
