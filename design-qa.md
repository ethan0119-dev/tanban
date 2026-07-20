# Tanban product media visual QA

- QA date: 2026-07-20
- Production merchant console: https://mysales.666qwe.cn/products
- Production media library: https://mysales.666qwe.cn/media-library
- Production API readiness: https://tbapi.666qwe.cn/readyz

## Reference inputs

- Brand source: `/Users/lxy/Library/Containers/com.tencent.xinWeChat/Data/Documents/xwechat_files/smarti_0fb6/temp/RWTemp/2026-07/0f1a5203bae72a6efcd9fa62a0396b14/e1c6d050b0b655403870d540c3611447.png`
- Product operation reference: `/var/folders/86/dyvpwsyn2t5_gptz5kjm4d0h0000gn/T/codex-clipboard-38202558-8d5c-4214-9a3f-3077c7a89a70.png`
- Image selector reference: `/var/folders/86/dyvpwsyn2t5_gptz5kjm4d0h0000gn/T/codex-clipboard-db140908-7d46-4625-8cda-d10196c2b9e5.png`

## Evidence

- `artifacts/product-media-qa.png`
- `artifacts/product-media-qa-wide.png`
- `artifacts/product-table-qa.png`
- `artifacts/media-library-qa.png`
- `artifacts/product-image-picker-qa.png`

## Comparison result

- The cleaned transparent Tanban logo is readable on the dark merchant sidebar and uses the warm copper accent shared by the management UI.
- Product rows expose the requested edit, shelf, recommend, statistics, copy, delete, sold-out, restock, display-channel and batch-operation controls.
- Product editing supports one primary image and up to three gallery images through the shared merchant image selector.
- The image selector follows the reference information architecture: groups, search, upload, asset grid, selected count and confirmation actions.
- The same media library selector is reused by products, store decoration and hotspots, popup advertising and store logo settings.
- The mini-program product detail renders the primary image and gallery as a swiper.
- The Tanban visual theme deliberately differs from the blue reference system while retaining its task structure and interaction density.
- At narrower desktop widths the product table scrolls horizontally and pins the operation column; this is an acceptable admin-table behavior.
- The production media library currently has sparse content because QA preserved real tenant data and avoided destructive or artificial production mutations.

## Functional and accessibility checks

- Product list, product editor, media library, image selector and product statistics were exercised in the production browser.
- Product statistics returned paid-order gross metrics and displayed the documented `PAID_ORDER_GROSS_BEFORE_REFUNDS` scope.
- Dialogs expose named controls, keyboard-close behavior and visible focus/selection states.
- Production browser console contained no errors or warnings during the checked flows.
- API readiness, merchant site and platform site returned healthy responses after deployment.
- Frontend tests/build/typecheck, Go tests, lint, formatting and OpenAPI structural validation passed.

final result: passed
