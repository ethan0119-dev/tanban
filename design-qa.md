# Design QA

## Source

- Reference screenshot: `/var/folders/86/dyvpwsyn2t5_gptz5kjm4d0h0000gn/T/codex-clipboard-c866aa3f-dce6-4016-b96a-98a88a2f5680.png`
- Navigation reference: `/var/folders/86/dyvpwsyn2t5_gptz5kjm4d0h0000gn/T/codex-clipboard-d49caeb4-4884-4fd2-b0e0-205fd4ce5023.png`

## Implementation evidence

- Home: `/Users/lxy/works/salesyyp/.qa/home-automator.png`
- Menu: `/Users/lxy/works/salesyyp/.qa/menu-automator.png`
- Recharge: `/Users/lxy/works/salesyyp/.qa/recharge-automator.png`
- Coupons: `/Users/lxy/works/salesyyp/.qa/coupons-automator.png`
- Side-by-side comparison: `/Users/lxy/works/salesyyp/.qa/home-reference-vs-implementation.png`
- Viewport: WeChat DevTools iPhone 12/13 mini simulator, 366 x 794 capture.
- State: `manong-coffee`, warm bakery theme, store closed.

## Comparison and findings

1. The reference showed an active but visually empty popup with a stretched close control and undersized CTA. The implementation no longer renders popup records without title, subtitle, or image, so the store content remains usable.
2. Warm bakery colors now reach the menu, recharge, coupon, checkout, order, profile, lottery, and legal surfaces through shared appearance tokens.
3. The menu cart bar is attached directly above the native tab bar instead of floating with large rounded corners.
4. Native tab icons are present in normal and selected states. Four navigation presets can now select coordinated icon assets and colors without previewing shapes that the native WeChat tab bar cannot reproduce.
5. Menu, recharge, and coupon screenshots were reviewed for clipping, typography, spacing, color consistency, disabled states, and bottom safe-area placement; no blocking visual defect remains.

## Iteration history

- Iteration 1: replaced the popup button default styling and added a real close icon.
- Iteration 2: suppressed empty popup content at both API and mini-program layers.
- Iteration 3: introduced shared page appearance loading and replaced page-local green constants with theme tokens.
- Iteration 4: attached the cart bar to the page bottom and added icon-backed navigation presets.
- Iteration 5: removed rounded navigation shapes from the admin preview because the production mini-program currently uses the native tab bar.

## Final result

`passed`

---

# Store Settings Design QA — 2026-07-22

## Source visual truth

- Basic information reference: `/var/folders/86/dyvpwsyn2t5_gptz5kjm4d0h0000gn/T/codex-clipboard-c43c59e2-a17c-441d-a8db-c15832125372.png`
- Store information reference: `/var/folders/86/dyvpwsyn2t5_gptz5kjm4d0h0000gn/T/codex-clipboard-b9e749af-f0a5-4465-bd5d-6822f59b90e6.png`

## Implementation evidence

- Production URL: `https://mysales.666qwe.cn/settings/store`
- Basic information screenshot: `/tmp/salesyyp-store-settings-basic.png`
- Store information screenshot: `/tmp/salesyyp-store-settings-operation.png`
- Responsive screenshot: `/tmp/salesyyp-store-settings-mobile.png`
- Desktop viewport: `1266 x 1050`; production data, manager session, store `manong-coffee-gulou`.
- Responsive viewport: `390 x 844`; collapsed navigation state.

## Full-view comparison evidence

- The implementation keeps the reference's two-tab information architecture and the same primary field groups while using the existing Tanban card, spacing, typography, icon and brown theme tokens.
- Basic information has a clearer status summary before the form; store identity, visibility, logo, contact, region, address, coordinates and announcement follow the reference's task order.
- Store information preserves operating channels, business hours, main products, average spend, environment images and food-safety material, while retaining Tanban's existing weekly schedule, temporary override, reviewed licenses and official ordering entry.

## Focused-region comparison evidence

- Typography: PingFang/system Chinese UI font, 12–16 px form hierarchy, 22–25 px page heading and medium card headings remain consistent with the existing merchant console.
- Spacing/layout: paired desktop fields use a two-column grid, grouped sections use 12–20 px rhythm, and mobile collapses status rows, schedule controls and image grids without horizontal overflow.
- Colors/tokens: brown primary actions, warm neutral surfaces, green open/visible status and amber closed status use existing semantic tokens with sufficient contrast.
- Image quality/assets: the real store logo and reviewed license images are used; environment and safety images are selected from the existing media library rather than placeholders.
- Copy/content: all settings explain their customer-visible effect, unavailable delivery is explicitly disabled, and certificate ownership remains clearly assigned to the platform.

## Findings and accepted constraints

- No actionable P0, P1 or P2 mismatch remains.
- The reference embeds a third-party map. The implementation intentionally uses validated latitude/longitude fields plus a high-resolution external map link, avoiding an unconfigured map SDK and preserving a reliable save path.
- The reference uses a sparse, fixed-width form. The implementation intentionally uses responsive cards to match the current product and keep the same information hierarchy at desktop and narrow widths.

## Interaction and runtime checks

- Tabs, image-library modal open/cancel, responsive layout, production data load and unchanged-value save/readback were tested.
- Save returned `店铺设置已保存并同步到顾客端`.
- Browser console errors and warnings checked: none.

## Comparison history

- Iteration 1: implemented and compared both tabs against the supplied references.
- Iteration 1 result: no actionable P0/P1/P2 issue; no visual fix loop required.

## Final result

`passed`
