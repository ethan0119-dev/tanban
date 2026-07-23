import assert from "node:assert/strict";
import test from "node:test";

async function render(pathname = "/") {
  const workerUrl = new URL("../dist/server/index.js", import.meta.url);
  workerUrl.searchParams.set("test", `${process.pid}-${Date.now()}`);
  const { default: worker } = await import(workerUrl.href);

  return worker.fetch(
    new Request(`http://localhost${pathname}`, { headers: { accept: "text/html" } }),
    { ASSETS: { fetch: async () => new Response("Not found", { status: 404 }) } },
    { waitUntil() {}, passThroughOnException() {} },
  );
}

test("server renders the Tanban product prototype", async () => {
  const response = await render();
  assert.equal(response.status, 200);
  assert.match(response.headers.get("content-type") ?? "", /^text\/html\b/i);

  const html = await response.text();
  assert.match(html, /摊伴/);
  assert.match(html, /TANBAN/);
  assert.match(html, /平台管理端/);
  assert.match(html, /商户运营端/);
  assert.match(html, /顾客点单端/);
  assert.doesNotMatch(html, /Your site is taking shape|codex-preview/i);
});

test("server renders the mobile copyright advertisement page", async () => {
  const response = await render("/copyright?brand=%E6%91%8A%E4%BC%B4%E9%A4%90%E9%A5%AE%E7%B3%BB%E7%BB%9F");
  assert.equal(response.status, 200);
  assert.match(response.headers.get("content-type") ?? "", /^text\/html\b/i);
  const html = await response.text();
  assert.match(html, /版权说明|一套系统，连接经营全流程/);
  assert.match(html, /扫码联系摊伴/);
});
