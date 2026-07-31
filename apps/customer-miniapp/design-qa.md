# Member Price Component Design QA

- Source visual truth: `design-qa-artifacts/member-price-reference-crop.png`
- Implementation screenshots:
  - `design-qa-artifacts/member-price-implementation-screen.png`
  - `design-qa-artifacts/member-price-list-screen.png`
  - `design-qa-artifacts/member-price-recommended-fixed-screen.png`
- Combined comparison: `design-qa-artifacts/member-price-visual-comparison.png`
- Real-device regression comparison: `design-qa-artifacts/member-price-overlap-fix-comparison.png`
- State: active member discount, recommended card, product list, SKU picker with selected and unselected SKUs, configuration summary
- Device: WeChat DevTools, iPhone 12/13 Pro simulator at 94%
- Capture viewport: 960 × 768 px DevTools window
- Source crop: 820 × 1080 px
- Implementation crops: 290 × 660 px
- Density normalization: the three panels were proportionally resized to 740 px height for the combined comparison; this is a component-style comparison because the supplied source and Tanban implementation use different full-screen layouts.

## Full-view comparison evidence

The combined comparison confirms that the implementation preserves the selected source pattern: a dark membership label and a warm-gold discounted amount share one frame, while the original amount remains visually secondary. Tanban intentionally uses its existing coffee-brown and warm-gold palette instead of the source's black and pale yellow.

The recommended card uses a stacked compact variant and a two-row footer: the member-price frame owns the upper row, while the original price and action button are independently anchored below it. The product list uses the Chinese `会员价` label, and the constrained SKU choices use the shorter `VIP` label. The larger configuration summary restores the full Chinese label.

## Focused region comparison evidence

The source crop and the two simulator crops are already focused on the price component. The following surfaces were checked:

- Fonts and typography: discounted amounts retain the strongest weight; membership labels remain readable at compact sizes; original prices are smaller and struck through.
- Spacing and layout rhythm: label and amount share one frame without gaps; compact variants fit recommendation and SKU controls; no price overlaps the action button.
- Colors and visual tokens: dark coffee `#3f302b`, warm gold `#f0dda6`, and gold label text `#f5dfa5` match Tanban's established palette and retain clear contrast.
- Image quality and asset fidelity: no new raster or icon assets were required; existing product images remain unchanged.
- Copy and content: `VIP` is used where width is constrained, while `会员价` is used in the product row and total-price summary.

## Interaction and runtime checks

- Opened the menu page from the tab bar.
- Opened the discounted product's SKU picker.
- Checked selected and unselected SKU price states.
- Switched to the product category containing the member-priced product.
- Confirmed DevTools reported 0 build/runtime errors. Four non-blocking warnings remained for a separate cleanup pass.

## Findings

No actionable P0, P1, or P2 visual differences remain.

The first uploaded build exposed one P1 issue on a real iPhone: the recommended-card footer still used a single flex row, so the fixed-width `选择` button reduced the member-price component's width and clipped `¥10.8`. The footer now reserves 88 rpx for two layers and positions the member-price frame separately from the action button. The post-fix simulator capture confirms the full amount is visible with clear horizontal and vertical separation.

The original-price strike-through is intentionally retained even though the source image uses a plain secondary price; it better communicates the discount relationship and matches Tanban's existing behavior.

## Comparison history

- Initial simulator implementation: passed the focused component comparison.
- Real-device feedback: found a P1 overlap/clipping issue in the narrow recommended card.
- Correction: replaced the shared flex row with a two-layer footer and added a layout-regression assertion.
- Post-fix simulator capture: passed; `VIP ¥10.8`, `¥12`, and `选择` are all fully visible and do not overlap.

final result: passed
