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

test("server renders the Tanban official website", async () => {
  const response = await render();
  assert.equal(response.status, 200);
  assert.match(response.headers.get("content-type") ?? "", /^text\/html\b/i);

  const html = await response.text();
  assert.match(html, /摊伴/);
  assert.match(html, /TANBAN/);
  assert.match(html, /小店，也值得拥有一套/);
  assert.match(html, /顾客扫码点单/);
  assert.match(html, /店员平板收银/);
  assert.match(html, /tanban-icon-/);
  assert.doesNotMatch(html, /Your site is taking shape|codex-preview/i);
});

test("server renders the warm news list and article routes", async () => {
  const listResponse = await render("/news");
  assert.equal(listResponse.status, 200);
  const listHtml = await listResponse.text();
  assert.match(listHtml, /摊伴动态/);
  assert.match(listHtml, /tanban-icon-/);

  const articleResponse = await render("/news/tanban-official-website");
  assert.equal(articleResponse.status, 200);
  const articleHtml = await articleResponse.text();
  assert.match(articleHtml, /摊伴官网与内容管理能力进入开发阶段/);
  assert.match(articleHtml, /返回摊伴动态/);
});

test("server renders the mobile copyright advertisement page", async () => {
  const response = await render("/copyright?brand=%E6%91%8A%E4%BC%B4%E9%A4%90%E9%A5%AE%E7%B3%BB%E7%BB%9F");
  assert.equal(response.status, 200);
  assert.match(response.headers.get("content-type") ?? "", /^text\/html\b/i);
  const html = await response.text();
  assert.match(html, /版权说明|一套系统，连接经营全流程/);
  assert.match(html, /扫码联系摊伴/);
});
