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

# 快餐订单取餐号卡片视觉验收

- source visual truth path: `/var/folders/86/dyvpwsyn2t5_gptz5kjm4d0h0000gn/T/codex-clipboard-187cd980-1c4c-4191-90bb-93038fb0ca2c.png`
- implementation screenshot path: `/Users/lxy/works/salesyyp/apps/merchant-web/design-qa-artifacts/fast-food-order-pickup-badge-final.png`
- combined comparison path: `/Users/lxy/works/salesyyp/apps/merchant-web/design-qa-artifacts/fast-food-order-pickup-badge-comparison.png`
- implementation URL: `http://127.0.0.1:5174/order-card-qa.html` (temporary local QA entry)
- source pixels: `325 × 430`
- implementation pixels and CSS viewport: `342 × 480`, device density `1`
- density normalization: source scaled proportionally to `480px` height for the side-by-side focused comparison
- state: 已付款、请取餐的快餐自取订单，取餐号 `0003`，码牌 `A03`

## Full-view comparison evidence

The supplied screenshot is the reported before-state. In the combined comparison, the left card uses a large white tile containing only a small number icon and repeats the actual pickup number in the adjacent text block. The revised card on the right uses the same footprint for a warm, high-contrast `取餐号 0003` badge and repurposes the adjacent hierarchy for the placement name and plate code. The rest of the card structure, status tags, products, remark, time, amount, and action remain aligned with the original.

## Focused region comparison evidence

The entire card is readable at the comparison size, so no additional crop was necessary. Browser measurements confirm the placement title has equal `clientWidth` and `scrollWidth` (`136px`), so `收银台右侧` is not truncated. The accessibility tree exposes `取餐号 0003`, and no duplicate pickup number remains in the placement description.

## Required fidelity surfaces

- Fonts and typography: the pickup number uses the established strong card weight; the smaller `取餐号` label preserves hierarchy without competing with the order status.
- Spacing and layout rhythm: the left badge reuses the dining-card `82 × 72px` service marker footprint, while the right service block flexes into the remaining width without overflow.
- Colors and visual tokens: the badge uses the existing warm gold/brown takeout palette and remains distinct from the cyan `请取餐` status.
- Image quality and asset fidelity: no raster assets or replacement icons were introduced; the empty number icon was removed rather than approximated.
- Copy and content: the card now separates `取餐号` from `放餐位置`, displays the assigned plate code, and falls back to `未生成` / `未指定码牌` when data is absent.

## Comparison history

### Initial state — P2

- [P2] The large white service tile appeared empty because it contained only a small `#` icon, while the important pickup number was duplicated in the adjacent area.

Fix: moved the pickup label and number into the service tile, adopted the same numbered-marker pattern as dining orders, and changed the adjacent copy to placement and plate information.

### Final pass

No actionable P0/P1/P2 visual differences remain. The pickup number is immediately scannable, the placement title is not truncated, and the original card controls remain visible.

## Runtime and test checks

- Browser-rendered implementation checked at `342 × 480`.
- Accessibility name `取餐号 0003` and full placement title were verified.
- Browser console has no errors or deprecation warnings from this card.
- The component test covers populated and missing pickup/plate data and verifies the primary `查看并处理` action.
- Merchant-web test suite and production build pass.

final result: passed

# 快餐取餐大屏视觉验收

- source visual truth paths:
  - `/var/folders/86/dyvpwsyn2t5_gptz5kjm4d0h0000gn/T/codex-clipboard-158fdb9a-1012-4db2-bde4-bb55c9fed59e.png`
  - `/var/folders/86/dyvpwsyn2t5_gptz5kjm4d0h0000gn/T/codex-clipboard-9cd2cf09-a895-4780-9851-5983f47e844a.png`
- implementation URL:
  - `http://127.0.0.1:4177/__preview/pickup-display?layout=landscape`
  - `http://127.0.0.1:4177/__preview/pickup-display?layout=portrait`
- implementation screenshots:
  - `/Users/lxy/works/salesyyp/.qa/pickup-display-landscape-1920x1080.jpg`
  - `/Users/lxy/works/salesyyp/.qa/pickup-display-portrait-900x1600.jpg`
- combined comparison evidence:
  - `/Users/lxy/works/salesyyp/.qa/pickup-display-landscape-comparison.jpg`
  - `/Users/lxy/works/salesyyp/.qa/pickup-display-portrait-comparison.jpg`
- source pixels: `2048 × 1365` landscape reference and `844 × 2048` portrait photo
- implementation pixels / CSS viewport / density: `1920 × 1080` at `1920 × 1080`, and `900 × 1600` at `900 × 1600`; browser capture used `deviceScaleFactor: 1`
- state: realistic store name with five preparing and three ready pickup codes

## Source interpretation

The two screenshots are reference systems rather than a pixel-identical brand target. Their shared visual truth is a large store identity, a clear preparing/ready split, very large pickup codes, and a layout that remains legible on both horizontal and vertical public displays. The implementation intentionally uses the existing Tanban warm-brown brand shell and a green ready state instead of copying the competitors' neutral gray or photographic header.

## Full-view comparison

- Landscape keeps the source's two equal operational columns and strengthens long-distance scanning with bordered number cards, a high-contrast dark surround, and a dedicated store/time header.
- Portrait changes the reference photo's narrow side-by-side columns into stacked preparing and ready regions. This is intentional: each pickup code remains materially larger at a 9:16 viewing distance.
- Both target viewports have `scrollWidth === innerWidth` and `scrollHeight === innerHeight`; no region or persistent footer overflows.

## Required fidelity surfaces

- Fonts and typography: system Chinese sans-serif with 900-weight pickup numbers; store, state, helper copy, and metadata use distinct optical scales. Four-character pickup codes remain unwrapped.
- Spacing and layout: consistent outer shell gaps, large rounded panels, aligned headings, and bounded number grids. Landscape uses two columns; portrait uses two rows.
- Colors and tokens: warm near-black shell and cream panel map to Tanban's merchant brand. Ready uses a semantic green surface and text with strong contrast.
- Image quality and assets: the screen does not require decorative raster imagery. A real store logo from the API is rendered when configured; operational icons come from the existing Ant Design icon library.
- Copy and content: store name, business date, live time, active count, update state, preparing instructions, and pickup instructions remain independently understandable on a public screen.
- Accessibility and resilience: semantic section labels, button label, reduced-motion handling, no text overlap, and no viewport overflow at both tested aspect ratios.

## Interaction and runtime evidence

- The landscape and portrait query modes were both opened in the browser and rendered the expected layout.
- The fullscreen control is visible and keyboard-addressable. The automation browser declined fullscreen permission; the component showed its supported fallback instruction instead of failing.
- Polling, visibility refresh, rotating pagination, reconnect copy, empty-state copy, and newly-ready highlighting are implemented.
- Browser console contains no application error; only the repository's existing React Router v7 future notices were recorded.

## Focused comparison

A separate crop was not necessary because the combined comparisons keep the store header, state headings, icons, and complete four-character pickup codes legible. The original-size implementation screenshots were also inspected for border, shadow, line-height, and icon alignment.

## Comparison history

### Pass 1

No actionable P0/P1/P2 difference was found. The implementation preserves the reference information hierarchy while deliberately adapting the portrait layout for larger pickup codes.

## Findings

No actionable P0/P1/P2 findings remain.

## Follow-up polish

- [P3] After the first real shop trial, measure the longest comfortable viewing distance and tune the maximum number-card page size if a display is smaller than 32 inches.

final result: passed

# 堂食结算状态、点餐设置与收银台合计区视觉验收

- source visual truth paths:
  - `/var/folders/86/dyvpwsyn2t5_gptz5kjm4d0h0000gn/T/codex-clipboard-e6a376cd-377f-4f95-8d4d-8707b65280e6.png`
  - `/var/folders/86/dyvpwsyn2t5_gptz5kjm4d0h0000gn/T/codex-clipboard-f075a92f-f6bc-4fd8-a3fb-bdced654459c.png`
  - `/var/folders/86/dyvpwsyn2t5_gptz5kjm4d0h0000gn/T/codex-clipboard-f4fd8ce5-a1c4-4a92-90f3-fe1a1e401715.png`
  - `/Users/lxy/Downloads/IMG_0003.PNG`（收银台既有 iPad 失败状态）
- implementation URLs:
  - `http://127.0.0.1:4177/__preview/operation-settings`
  - `http://127.0.0.1:4177/__preview/cashier`
- implementation screenshots:
  - `/Users/lxy/works/salesyyp/.qa/ordering-flow-settings-pay-before-1440x1000.jpg`
  - `/Users/lxy/works/salesyyp/.qa/ordering-flow-settings-pay-after-1440x1000.jpg`
  - `/Users/lxy/works/salesyyp/.qa/ordering-flow-cashier-1180x688.jpg`
- combined comparison paths:
  - `/Users/lxy/works/salesyyp/.qa/ordering-flow-settings-comparison.jpg`
  - `/Users/lxy/works/salesyyp/.qa/ordering-flow-settings-pay-before-focused.jpg`
  - `/Users/lxy/works/salesyyp/.qa/ordering-flow-settings-pay-after-focused.jpg`
  - `/Users/lxy/works/salesyyp/.qa/ordering-flow-cashier-comparison.jpg`
- viewports:
  - settings desktop: `1440 × 1000` CSS px, `deviceScaleFactor: 1`
  - settings tablet check: `1024 × 768` CSS px, `deviceScaleFactor: 1`
  - cashier iPad content: `1180 × 688` CSS px, `deviceScaleFactor: 1`
- source pixel dimensions: `986 × 224`, `789 × 177`, `783 × 170`; the cashier source was normalized from its browser-content crop to `1180 × 688`
- implementation pixel dimensions: settings captures `1418 × 985`; cashier capture `1180 × 688`
- state: pay-before settings, pay-after settings, pay-after online payment disabled, and cashier B03 pay-after bill with a fixed total region

## Source interpretation

The three supplied setting screenshots are functional references, not a request to replace the merchant console’s existing Ant Design visual language. Their source truth is the behavior and hierarchy: unpaid-order timeout, table-scan landing choice, dine-in settlement mode, clear-table policy, and pay-after online-payment switch. The cashier source is a reported responsive failure state; the implementation must keep the current product language while preserving the receipt total and actions inside the iPad content viewport.

## Full-view comparison evidence

The combined settings comparison places all supplied controls and both rendered implementation modes in one image. The implementation includes every referenced capability and makes mode-dependent controls explicit: pay-before shows the clear-table policy, while pay-after shows the customer online-payment switch. The full cashier comparison places the normalized prior iPad state above the revised implementation; the revised receipt total is pinned immediately above the action dock and all primary actions remain visible.

## Focused comparison evidence

The two focused settings comparisons place each source mode and the matching rendered settings card in one image at readable scale. The implementation intentionally renames the ambiguous source option “定时清台” to “完成订单后清台” and explains its exact lifecycle. A separate cashier crop was not required because the full `1180 × 688` comparison renders the receipt total and complete action dock at readable size.

## Primary interactions tested

- Switched堂食结算模式 from先结账后用餐 to先用餐后结账; the clear-table control was replaced by the online-payment switch.
- Disabled顾客线上支付 and verified the switch state changed to off.
- Saved the preview form and received the validation-success message.
- Verified the table board renders “已结账” instead of “待点单” for paid pay-before orders.
- Opened交接班 and verified its lightweight-record limitation, daily overview, remark field, and exit action are visible.
- At `1180 × 688`, measured the scrollable receipt detail, fixed total, and action dock as separate non-overlapping regions.
- At `1024 × 768`, verified the settings page has no horizontal overflow.

## Required fidelity surfaces

- Fonts and typography: retained the existing PingFang SC / Microsoft YaHei / system stack and the merchant console hierarchy; mode descriptions remain readable at desktop and tablet widths.
- Spacing and layout rhythm: settings use the existing card, divider, row, and control rhythm. The cashier receipt details are the only scrolling region; the total and action dock are fixed below it.
- Colors and visual tokens: reused existing merchant brown, semantic green/orange/red states, Ant Design alerts, borders, and switch tokens. The new “已结账” table state uses the established green family.
- Image quality and asset fidelity: this flow introduces no raster illustration or replacement asset; existing Ant Design icons remain vector-sharp at the tested density.
- Copy and content: the implementation preserves the reference capabilities while clarifying scope:堂食模式 does not change带走 orders, clear-table policy affects occupancy rather than kitchen production, and轻量交接 does not claim cash-drawer reconciliation.
- Accessibility and interaction states: settings use labelled radio groups and switches; active, checked, and disabled states are exposed in the browser accessibility tree. Primary cashier controls remain visible and touch-sized.

## Comparison history

### Pass 1 — blocked

- [P1] A paid pay-before table was mapped to the legacy “待点单” state, contradicting the paid order shown in当前操作.
- [P1] The reference settings did not exist as executable merchant controls.
- [P2] The receipt total lived inside the scrolling order panel rather than a fixed bottom region.
- [P2] The existing handover copy implied more complete shift archiving than the current audit-only implementation provides.

Fixes: introduced explicit `PENDING_PAYMENT`, `SETTLED`, `DINING`, and `UNSETTLED` table states; added the three referenced mode-dependent controls and server enforcement; pinned the receipt total below the scrollable item list; and corrected the handover explanation.

### Final pass

No actionable P0/P1/P2 visual or interaction findings remain for the requested flows.

## Runtime and test checks

- Browser-rendered implementation checked at all three viewports above.
- No application exception, failed resource, or layout overflow was observed.
- Existing React Router future notices and Ant Design v5 / React 19 compatibility and deprecation warnings remain dependency-level P3 follow-up.
- API full Go test suite passes.
- Merchant web production build and all 65 tests pass.
- Customer miniapp typecheck and all 41 tests pass.

## Follow-up polish

- [P3] Upgrade Ant Design and migrate deprecated `bordered` / `addonAfter` props in a separate dependency-focused change.

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
