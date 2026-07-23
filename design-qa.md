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
