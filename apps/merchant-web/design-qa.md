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

# 收银台 iPad 操作区与按钮功能验收

- source visual truth path: `/Users/lxy/Downloads/IMG_0003.PNG`
- normalized source path: `/Users/lxy/works/salesyyp/.qa/cashier-ipad-source-content.jpg`
- implementation screenshot path: `/Users/lxy/works/salesyyp/.qa/cashier-ipad-final.jpg`
- combined comparison path: `/Users/lxy/works/salesyyp/.qa/cashier-ipad-comparison.jpg`
- implementation URL: `http://127.0.0.1:4175/__preview/cashier`
- viewport: `1180 × 688` CSS px, `deviceScaleFactor: 1`
- source pixels: `2360 × 1640`; cropped app content from y=`264` to `1640`, then normalized from `2360 × 1376` to `1180 × 688`
- implementation pixels: `1180 × 688`
- state: iPad Air landscape browser content area,堂食订单 B03 待结账

## Source interpretation

The supplied screenshot is the reported failure state rather than a pixel-identical target. It establishes the existing visual language and demonstrates the P1 issue: the order receipt consumes the operation column and pushes every action below the visible iPad viewport. The intended after-state preserves that language while making the four high-frequency actions persistent and collecting low-frequency actions into one visible entry.

## Full-view comparison evidence

The combined comparison places the normalized source on the left and the implementation on the right. The implementation preserves the two-column cashier hierarchy, dark rail, store header, table board, receipt styling, and bottom metrics. In the same `1180 × 688` content viewport it now keeps 加菜、修改人数、打印客户联、结账 and 更多操作 fully visible without body scrolling.

## Focused interaction evidence

A separate crop was not needed because the full-height comparison renders the complete action dock at readable size. Browser interaction verified:

- 修改人数 opens the editor and exposes 保存人数.
- 结账 opens both 现金收款 and 系统外支付.
- 更多操作 exposes 转台、并台、退菜; 并台 opens its confirmation workflow.
- 交接班 opens the handover summary and confirmation workflow.
- Operational alert cards filter the table grid, and the overdue count no longer fabricates a minimum value.

## Required fidelity surfaces

- Fonts and typography: retained the existing Inter / PingFang SC / Microsoft YaHei stack, hierarchy, and touch-sized action labels; compact mode only reduces action labels to 15–17px.
- Spacing and layout rhythm: receipt is now the flexible scroll region; the action dock and summary remain fixed. At the target viewport all four primary buttons end above y=`595` and 更多操作 ends above y=`642`.
- Colors and visual tokens: reused the existing cashier orange, green, red, line, and muted tokens. No new competing palette was introduced.
- Image quality and asset fidelity: no reference raster asset was replaced or approximated; existing Ant Design icons remain sharp at device density.
- Copy and content: retained the product’s established cashier wording and added only the explicit “更多操作” and handover explanations required by the responsive workflow.

## Comparison history

### Pass 1 — blocked

- [P1] The source failure state hides every order action below the iPad viewport.
- [P2] Transfer, merge, and return could not be retained alongside the four primary actions at this height without crowding.

Fixes: converted the receipt panel to the only scrollable flexible region, introduced a fixed action dock, reduced action height for short tablet viewports, and grouped low-frequency operations under 更多操作.

### Final pass

No actionable P0/P1/P2 visual differences remain for the requested responsive behavior. All persistent action controls fit inside the target content viewport and remain touch-sized.

## Runtime and test checks

- Browser-rendered implementation checked at `1180 × 688`; all primary buttons and 更多操作 are visible.
- Primary dialog and menu interactions were exercised in the browser.
- No application exception or failed resource was recorded. Existing React Router future notices and the repository’s Ant Design v5 / React 19 compatibility notice remain dependency-level P3 follow-up.
- Merchant web production build passes.
- Merchant web test suite passes: 18 files, 65 tests.
- API cashier tests pass, including access to the audited handover endpoint.

## Findings

No actionable P0/P1/P2 findings remain.

## Follow-up polish

- [P3] Upgrade Ant Design to a React 19-native release in a separate dependency change to remove the existing compatibility notice.

final result: passed

# 员工与角色权限悬停视觉验收

- source visual truth path: `/var/folders/86/dyvpwsyn2t5_gptz5kjm4d0h0000gn/T/codex-clipboard-1ba73867-8d4c-4549-a53b-224796a04743.png`
- implementation URL: `http://127.0.0.1:4176/`
- default implementation screenshot path: `/Users/lxy/works/salesyyp/.qa/staff-role-preview-desktop.png`
- hover implementation screenshot path: `/Users/lxy/works/salesyyp/.qa/staff-role-preview-hover.png`
- comparison path: `/Users/lxy/works/salesyyp/.qa/staff-role-comparison.png`
- tablet evidence path: `/Users/lxy/works/salesyyp/.qa/staff-role-preview-1024.png`
- desktop viewport: `1600 × 900` CSS px, `deviceScaleFactor: 1`
- source pixels: `1920 × 920`
- implementation pixels: `1600 × 900`
- state: realistic staff fixture with owner, manager, and staff role counts

## Source interpretation

The supplied screenshot is an annotated before-state rather than a pixel-identical target. Its red boxes define the content to remove from the three role cards. The required after-state therefore preserves the existing card, icon, role name, and staff count while replacing the inline permission tags with a compact information icon.

## Combined comparison evidence

The side-by-side comparison contains the annotated source and the default implementation in one input. It confirms that the marked tag blocks are absent, the cards remain aligned, and the page gains substantially more vertical space without changing the employee table below.

## Hover and focus evidence

The manager information icon was opened with a real mouse-hover sequence after moving the pointer away from the trigger. The popup contains the full manager permission set and wraps within a bounded width. The same control is keyboard-focusable and uses the role-specific accessible label `查看{角色}权限`.

## Responsive evidence

At `1024 × 768`, the role cards follow the existing two-column breakpoint and place the third card on a second row. The permission trigger remains adjacent to its role label with no clipping. The employee table keeps its existing horizontal-scroll behavior and was intentionally left unchanged.

## Runtime and test checks

- Browser runtime was reloaded after replacing deprecated card props; no new application exception or Ant Design deprecation warning was recorded.
- The component test verifies that role permission labels are not rendered inline and become available after hovering the information icon.
- The merchant-web production build passes.

## Findings

No actionable P0/P1/P2 visual differences remain.

final result: passed
