**Comparison Target**

- Source visual truth:
  - `/var/folders/86/dyvpwsyn2t5_gptz5kjm4d0h0000gn/T/codex-clipboard-48026c36-fd5c-44ea-a73f-ce99f70134c0.png`
  - `/var/folders/86/dyvpwsyn2t5_gptz5kjm4d0h0000gn/T/codex-clipboard-ee18583c-9c96-42d3-8b27-137e4ebd85c5.png`
- Implementation screenshot: `/Users/lxy/works/salesyyp/design-qa-implementation.png`
- Combined focused comparison: `/Users/lxy/works/salesyyp/design-qa-comparison.png`
- State: merchant decoration preview rendering `COUNT_ACTION`, `PROMO_CAPSULE`, and existing `CLASSIC` cart templates with one store theme.
- Browser viewport: 1265 × 780 CSS px; full-page implementation capture: 1265 × 828 px at device scale factor 1.
- Source images: 1260 × 2736 px at 144 dpi. Focused source cart-bar crops: 1260 × 300 px.
- Density normalization: source and implementation focused regions were independently cropped, then normalized to a common 1250 px width for visual comparison. The references are inspiration for the cart-bar component rather than a full-screen pixel clone, so menu content and phone framing were excluded from fidelity findings.

**Findings**

- No actionable P0, P1, or P2 mismatch remains in the cart-template scope.
- Fonts and typography: the implementation preserves the source hierarchy of quantity/amount as the primary text and the action label as the strongest control. Text remains legible at preview scale, with no unintended truncation.
- Spacing and layout rhythm: both new templates keep a persistent bottom action area, clear icon-to-copy spacing, and a large tap target. The promotional template retains the source capsule silhouette and segmented offer block.
- Colors and visual tokens: the source structure is retained, while color intentionally follows the store decoration tokens instead of copying the screenshots' yellow, orange, or cyan brands. Foreground contrast is sufficient in the tested theme.
- Image quality and asset fidelity: the cart uses the project's existing shopping-cart icon asset/library icon; no emoji, handcrafted SVG, or CSS-drawn replacement was introduced.
- Copy and content: `已点 2 份菜品 / 去下单` and `优惠券 2张可用 / ¥30.00 / 选好了` clearly communicate the two referenced patterns.

**Open Questions**

- None blocking. The screenshots contain different merchant brands and full menu layouts; only their cart-bar patterns were treated as source truth, matching the requested scope.

**Comparison History**

- Pass 1 — P2: the `COUNT_ACTION` editor preview did not show a quantity badge and used the weaker copy `已点 2 件`, making the reference's at-a-glance item count less recognizable.
  - Fix: added a theme-controlled numeric badge to the cart icon and changed the preview copy to `已点 2 份菜品`.
  - Post-fix evidence: `/Users/lxy/works/salesyyp/design-qa-comparison.png` shows the revised count template together with both source cart bars.
- Pass 2 — no actionable P0/P1/P2 findings.

**Implementation Checklist**

- [x] Render three selectable cart templates in the decoration preview.
- [x] Keep template colors driven by store decoration tokens.
- [x] Preserve clear quantity/amount hierarchy and large primary actions.
- [x] Verify the rendered templates in the in-app browser.
- [x] Inspect the browser console; no errors were present. Two pre-existing React Router v7 future-flag warnings remain.
- [x] Verify primary visible states and controls through the browser DOM snapshot: all three templates, cart summaries, and action buttons rendered.

**Full-view and Focused Evidence**

- Full-view evidence: `/Users/lxy/works/salesyyp/design-qa-implementation.png` verifies all three templates in the same decoration-preview page.
- Focused evidence: `/Users/lxy/works/salesyyp/design-qa-comparison.png` places source and implementation cart regions in a single comparison image. Focused comparison was required because the cart controls are too small to judge reliably in the full-page screenshot.

**Follow-up Polish**

- P3: validate the final font rasterization and safe-area spacing once the templates are viewed in WeChat Developer Tools on a representative iPhone device profile.

final result: passed
