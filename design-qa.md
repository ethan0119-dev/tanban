# Design QA — hotspot decoration and structured thermal printing

Date: 2026-07-20

## Reference sources

- Decoration editor: `/var/folders/86/dyvpwsyn2t5_gptz5kjm4d0h0000gn/T/codex-clipboard-c66c6a93-1c76-47b4-bdef-924ac42dd6aa.png` (1917 × 1169)
- Print template editor: `/var/folders/86/dyvpwsyn2t5_gptz5kjm4d0h0000gn/T/codex-clipboard-64b8097c-49b1-4d85-8d14-1352c84cfed6.png` (1809 × 1139)

## Production implementation evidence

- Hotspot editor focused state: `artifacts/design-qa/decoration-hotspot-production.png` (1265 × 712)
- Merchant receipt, 58 mm: `artifacts/design-qa/print-template-merchant-58mm-full-production.png` (1265 × 2388 full page)
- Customer receipt, 80 mm preview: `artifacts/design-qa/print-template-customer-80mm-preview-production.png` (1265 × 712)
- Printer paper-width and copy-role routing: `artifacts/design-qa/printer-device-routing-production.png` (1265 × 712)
- Normalized side-by-side decoration comparison: `artifacts/design-qa/comparison-decoration-hotspot.png`
- Normalized side-by-side printing comparison: `artifacts/design-qa/comparison-structured-print.png`

The reference and implementation captures are desktop states. For visual comparison, both sides were normalized into equal-size desktop panels without changing their aspect ratio. The production interaction captures use the in-app browser's 1265 × 712 viewport.

## Interaction and functional checks

### Decoration hotspot

- Opened the production merchant decoration editor.
- Added the `单图热区` module.
- Uploaded a 1731 × 909 PNG through the authenticated merchant upload API and confirmed its public HTTPS media URL renders in the editor and mini-program preview.
- Entered draw mode and dragged a rectangle on the image. The editor produced percentage coordinates (`x`, `y`, `width`, `height`) and synchronized the overlay into the phone preview.
- Opened the action selector and confirmed the safe action set: no action, menu, orders, profile, and phone call.
- Changed the hotspot action from menu to orders and confirmed the preview state changed to `OPEN_ORDERS`.
- No draft was published during QA.
- Browser console errors: none.

### Thermal print templates

- Opened the production dine-in template editor.
- Verified the four isolated copy roles: merchant, customer, kitchen, and per-item label.
- Verified merchant receipt structure: store, scene, pickup number, order/table/time, item table, specifications/add-ons, totals, payment provider, notes, and footer.
- Verified customer receipt adds customer data and order QR placeholder fields.
- Verified kitchen receipt emphasizes products/specifications/notes and defaults to hiding prices.
- Verified item label splits to product-level cup/food labels.
- Switched customer receipt from 58 mm to 80 mm and confirmed preview width and dirty-state marker update.
- Triggered reload with unsaved changes and confirmed the role-specific discard warning appears before state loss.
- Opened printer configuration and confirmed physical 58/80 mm width plus allowed copy-role routing are explicit device settings.
- Browser console errors: none.

## Iteration history

1. Initial functional implementation added image hotspots and structured ticket roles.
2. Visual QA found that replacing a hotspot image could retain stale coordinates; image changes now confirm and clear old hotspots.
3. Workflow QA found unsaved changes could be lost across receipt roles/scenes; dirty state is now tracked per role with guarded reload/scene switching.
4. Release audit found payment/refund transactions could be coupled to printing; immutable `print_outbox` facts now decouple financial state from print delivery.
5. Release audit found multi-printer role duplication and width mismatch risks; devices now route explicit copy roles and rendering always follows physical paper width.
6. Production deployment exposed a MySQL 5.7 trigger privilege restriction; migration compatibility was changed to nullable legacy roles plus a generated unique key, requiring no elevated database privilege.
7. Final production QA confirmed the live pages, interactions, health checks, schema, logs, and browser console.

final result: passed
