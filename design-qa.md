**Comparison Target**
- Primary target: latest merchant print-template UI in `/Users/xiaoyi/works/tanban`.
- Reference: `/Users/xiaoyi/works/tanban/baseline-desktop.png`.
- Built result: `/Users/xiaoyi/works/tanban/built-desktop.png`.
- Additional breakpoint evidence: `/Users/xiaoyi/works/tanban/built-mobile.png`.
- Strategy: regenerated the signed-in route baseline with the preserved Playwright storage state and compared at exact matching viewport `1440x1000`.
- Route: `http://localhost:5173/settings/dine-in-print-template`.

**Summary of the Approach Taken**
- Started the merchant web app with live HMR.
- Reused the existing authenticated Playwright storage state from the prior QA session.
- Captured a fresh exact-viewport desktop baseline and matching built screenshot.
- Ran pixel-level image comparison with Pillow.
- Captured a narrow breakpoint screenshot after the final responsive pass.
- Applied one corrective change: removed the remaining fixed `min-width: 860px` from the scrollable master/template region so the master-detail view can collapse on narrow screens.

**QA Findings**
- Screenshot comparison before/after the final responsive adjustment:
  - `1440x1000`: `baseline-desktop.png` vs `built-desktop.png`
  - `390x844`: `baseline-mobile.png` vs `built-mobile.png`
- Wide-screen visual diff: `0` pixels changed (`0.000000%`), max channel delta `0`.
- Responsive behavior:
  - Page shell still collapses to the mobile navigation pattern.
  - Master/detail grid collapses to one column.
  - The bottom/right border-radius mismatch in the selected template row is corrected for narrow layouts.
  - Horizontal scrolling is removed from the master/template work area.
- No additional visual changes were required after the last implementation pass.

**What Visually Matched**
- Page header hierarchy and spacing.
- Ant Design select controls, alert, and settings-card borders.
- Blue selection treatment in the master list.
- Business/printing-template tab treatment.
- Right-side template setting fields, radio/checkbox states, and summary card.
- Overall desktop spacing and container proportions.
- Exact desktop rendering against the fresh baseline (pixel-identical).

**Intentional Differences**
- No intentional desktop difference in the final QA comparison.
- The narrow breakpoint intentionally reorganizes the route for mobile:
  - Mobile navigation replaces the desktop sidebar.
  - Master and detail panels stack vertically.
  - Template rows use a simplified continuous edge treatment.
  - Selectors are allowed to shrink/wrap to the viewport.

**Approximate Improvements**
- The page was intentionally changed from a fixed-width desktop-only composition to a responsive master-detail layout at narrower breakpoints.
- The wide Ant Design `Select` controls now cap at `100%` width, which avoids clipping on mobile.

**Remaining Mismatches / Limitations**
- Live API requests still resolve against configured defaults during the local screenshot session; the visual QA used a preserved authenticated state and validated the rendered page output.
- Browser console still reports the existing React 19/Ant Design compatibility warning; this is unrelated to the design fidelity result.

**Evidence**
- `/Users/xiaoyi/works/tanban/baseline-desktop.png`
- `/Users/xiaoyi/works/tanban/built-desktop.png`
- `/Users/xiaoyi/works/tanban/baseline-mobile.png`
- `/Users/xiaoyi/works/tanban/built-mobile.png`

**Final Result**
- passed

---

# Print Template Design QA

## Evidence

- Source visual truth:
  - `/Users/xiaoyi/works/tanban/artifacts/design-qa/print-template/reference-label.png` (2374 × 816)
  - `/Users/xiaoyi/works/tanban/artifacts/design-qa/print-template/reference-receipt.png` (860 × 1702)
- Browser-rendered implementation:
  - `/Users/xiaoyi/works/tanban/artifacts/design-qa/print-template/label-implementation.png` (1289 × 1875)
  - `/Users/xiaoyi/works/tanban/artifacts/design-qa/print-template/receipt-implementation.png` (1289 × 1886)
- Combined comparisons:
  - `/Users/xiaoyi/works/tanban/artifacts/design-qa/print-template/label-comparison.png` (2400 × 1746)
  - `/Users/xiaoyi/works/tanban/artifacts/design-qa/print-template/receipt-comparison.png` (1720 × 1702)
- Browser: Codex in-app browser, default desktop viewport, device pixel ratio 2.
- State: dine-in print template; merchant receipt and 40 × 30 item-label tabs; Large, Detailed, paper-size, and Custom states exercised.
- Primary interactions tested: copy-role tabs, preset selection, label paper-size selection, field customization, live preview updates, and save-button dirty state.
- Console checked: no uncaught application errors. Existing Ant Design deprecation/React compatibility warnings remain and are unrelated to this change.

## Required Fidelity Surfaces

- Fonts and typography: passed. The admin UI keeps the product typography; physical-paper previews use a thermal/monospace treatment. Pickup code, role abbreviation, product name, and end marker have distinct emphasis.
- Spacing and layout rhythm: passed. Desktop uses a 9/15 preview-editor split, preset cards use a consistent two-column grid, and the 40 × 30 preview preserves its physical 4:3 ratio.
- Colors and visual tokens: passed. The implementation uses the existing tan/brown product palette and semantic Ant Design states instead of introducing a competing visual system.
- Image quality and asset fidelity: passed. The references contain no reusable product imagery or custom illustration assets; supplied screenshots were used only as visual truth.
- Copy and content: passed. The UI exposes paper size, font size, preset choice, label sequence, role abbreviation, end marker, feed lines, and existing custom content.

## Comparison History

### Iteration 1

- P1: the initial legacy label state selected “Large” while still showing the order number and omitting the bottom order type.
  - Fix: aligned new Large defaults with the reference structure (order type shown, order number hidden) and classified saved pre-preset layouts as Custom.
  - Post-fix evidence: the label preview shows `数量：1/2`, a large product name, specifications, attributes, remark, time, and `堂食`.
- P2: at a normal desktop width the preview and editor stacked vertically, reducing the usefulness of live configuration.
  - Fix: moved the split breakpoint from `xxl` to `xl`.
  - Post-fix evidence: browser bounding boxes are 487px and 811px at the tested viewport.

## Final Findings

No actionable P0/P1/P2 differences remain.

Accepted differences:

- The reference configuration screen is visually sparse; the implementation preserves the product’s existing page heading, business-scene switch, copy-role tabs, loading warning, and save workflow.
- The receipt reference is a physical photo while the implementation preview is a clean simulation. The actual printer output now uses the matching chip-level markup for enlarged centered pickup code, bold role abbreviation, explicit end marker, and trailing feed lines.

## Follow-up Polish

- P3: replace legacy Ant Design `bordered` and `InputNumber addonAfter` usages during a broader component-library cleanup.

final result: passed

---

# 点单页门店信息与推荐商品 Design QA

## Evidence

- Source visual truth: `/var/folders/9g/8zg58z0923n47byvvxydh46r0000gn/T/codex-clipboard-e3f8ff6a-564f-4083-a5ec-ebdec59b4409.png`
- Implementation screenshot: `/Users/xiaoyi/works/tanban/artifacts/design-qa/menu/implementation-real-data-20260723.png`
- Full WeChat DevTools screenshot: `/Users/xiaoyi/works/tanban/artifacts/design-qa/menu/menu-real-data-20260723.png`
- Combined comparison: `/Users/xiaoyi/works/tanban/artifacts/design-qa/menu/comparison-real-data-20260723.png`
- Store information interaction screenshot: `/var/folders/9g/8zg58z0923n47byvvxydh46r0000gn/T/com.openai.sky.CUAService/微信开发者工具 Screenshot 2026-07-23 at 6.39.04 PM.jpeg`
- Viewport: WeChat DevTools iPhone 12/13 Pro simulator at 93%
- Source pixels: 1260 × 2736; normalized to 278 × 604
- Implementation pixels: full DevTools screenshot 960 × 768; simulator crop 280 × 604
- Density normalization: both comparison images were normalized to 604 px height before side-by-side review
- State: self-pickup selected, three recommendation cards, first category selected, empty cart

## Full-view comparison evidence

The implementation preserves the source hierarchy: store identity and fulfillment selector, announcement with a right-aligned “更多” entry, three recommendation cards spanning the catalog width, category navigation beside the product list, and persistent cart/tab bars. The implementation intentionally retains the merchant decoration colors, current product photography, native WeChat navigation, and the system’s existing ordering modes.

## Focused interaction evidence

The “更多” control was activated in WeChat DevTools. It opens the bottom store-information sheet with announcement, business hours, phone/address details, copy-address, navigation when coordinates exist, and contact actions. No separate focused crop was needed because the complete sheet is readable in the interaction screenshot.

## Required fidelity surfaces

- Fonts and typography: existing PingFang/SF system stack, weight hierarchy, truncation, and small-label sizing remain consistent with the app design system.
- Spacing and layout rhythm: source hierarchy is matched; compact spacing is retained so the recommendation strip and first product remain above the cart bar.
- Colors and visual tokens: all controls use the published decoration variables rather than hard-coded reference colors.
- Image quality and assets: source product images and existing icon assets are used; no placeholder drawings or synthetic icons were introduced.
- Copy and content: store name, announcement, ordering modes, recommendation tag, prices, categories, and product details come from live catalog/store data.

## Comparison history

1. Initial runtime capture: recommendation strip was absent.
   - Finding: P1 — the production public catalog response omitted the `recommended` property, so the configured recommendation state could not reach the mini-program.
   - Evidence: `GET /api/v1/public/stores/manong-coffee-gulou/catalog` returned products without `recommended`.
2. Controlled visual-state capture: the same live products were projected into the already-implemented recommendation state only for visual inspection.
   - Fix verified: the recommendation strip renders in the correct position and the catalog remains usable below it.
   - The generated preview override was removed by rebuilding the mini-program.
3. Production deployment and real-data recapture:
   - Release `20260723T105116Z-1372378` passed the server backup, API build, readiness, frontend build, Nginx preflight, and activation stages.
   - The public catalog now returns `recommended: true` for all three configured products.
   - The real-data WeChat DevTools capture shows all three products in the top recommendation strip.
   - Mini-program version `1.0.20260723.2` uploaded successfully and replaced the prior experience version.

## Findings

No actionable P0/P1/P2 differences remain.

Accepted content difference:

- The test store has not configured its phone, address, coordinates, or business hours, so the “更多” sheet correctly renders the corresponding “暂未配置”/fallback copy. Once the merchant saves those fields, the same sheet exposes phone, address, and navigation actions without another mini-program release.

## Implementation checklist

- [x] Deploy the API version that includes `recommended` in public catalog products.
- [x] Confirm the configured products return `"recommended": true`.
- [x] Rebuild/upload the mini-program and repeat the real-data screenshot check.

final result: passed

---

# 点单页按钮、购物车与字体 Design QA

## Evidence

- Source visual truth: `/var/folders/9g/8zg58z0923n47byvvxydh46r0000gn/T/codex-clipboard-cbf20573-62bb-4491-86aa-5037f1815135.png`
- Final menu screenshot: `/Users/xiaoyi/works/tanban/artifacts/design-qa/menu/menu-typography-buttons-final-20260723.jpeg`
- Final simulator crop: `/Users/xiaoyi/works/tanban/artifacts/design-qa/menu/implementation-crop-20260723.png`
- Cart sheet crop: `/Users/xiaoyi/works/tanban/artifacts/design-qa/menu/cart-sheet-crop-final-20260723.png`
- Same-size comparison: `/Users/xiaoyi/works/tanban/artifacts/design-qa/menu/reference-vs-final-20260723.png`
- Source pixels: 882 × 1926; normalized to 280 × 604
- Implementation pixels: full DevTools screenshot 960 × 768; simulator crop 280 × 604
- Viewport: WeChat DevTools iPhone 12/13 Pro simulator
- Runtime debugger: 0 errors

## Required Fidelity Surfaces

- Fonts and typography: passed. Global caption/body/subtitle/title/display tokens were raised to 24/30/34/40/48rpx. Core customer pages now use 500–600 weights for most headings and controls; amounts retain 700 for checkout recognition.
- Spacing and layout rhythm: passed. Recommendation prices and actions have visible separation, text actions are compact pills, and no-option actions are 48–56rpx circular buttons rather than full-width controls.
- Colors and visual tokens: passed. New controls continue to inherit merchant decoration variables for primary, surface, muted, border, radius, and shadow values.
- Image quality and assets: passed. Existing catalog photography and icon assets are preserved.
- Copy and content: passed. Product names, prices, specification summaries, quantities, and cart totals are supplied by the current catalog/cart state.

## Interaction Verification

1. Opened the cart by activating the cart quantity/summary area.
2. Verified the sheet lists the exact cart line with its configuration summary and line total.
3. Increased `拿铁` from 1 to 2; the sheet count and bottom total changed from `¥16` to `¥32`.
4. Decreased it from 2 to 1, then from 1 to 0.
5. Verified quantity 0 removes the line, closes the now-empty sheet, and resets the cart summary to `0 / ¥0 / 已选 0 件`.

## Findings

No actionable P0/P1/P2 differences remain.

Accepted differences:

- The source screenshot contains the runtime vConsole overlay and a selected specification row. The final capture intentionally uses an empty-cart state after the deletion-to-zero verification.
- The reference did not define a cart-sheet visual. The added sheet follows the same decoration tokens, typography scale, radius, and control language as the point-of-sale screen.

final result: passed
