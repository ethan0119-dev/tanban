import { execFileSync } from "node:child_process";
import { cpSync, existsSync, mkdirSync, readFileSync, readdirSync, rmSync, statSync, writeFileSync } from "node:fs";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import path from "node:path";

const packageRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const sourceRoot = path.join(packageRoot, "miniprogram");
const outputRoot = path.join(packageRoot, "dist", "miniprogram");
const require = createRequire(import.meta.url);
const compiler = require.resolve("typescript/bin/tsc");

if (!outputRoot.startsWith(`${packageRoot}${path.sep}dist${path.sep}`)) {
  throw new Error("refusing to clean an unexpected mini-program output path");
}
rmSync(outputRoot, { recursive: true, force: true });
mkdirSync(outputRoot, { recursive: true });

execFileSync(process.execPath, [compiler, "-p", path.join(packageRoot, "tsconfig.build.json")], {
  cwd: packageRoot,
  stdio: "inherit",
});

function copyAssets(sourceDirectory, outputDirectory) {
  for (const name of readdirSync(sourceDirectory)) {
    const source = path.join(sourceDirectory, name);
    const output = path.join(outputDirectory, name);
    if (statSync(source).isDirectory()) {
      mkdirSync(output, { recursive: true });
      copyAssets(source, output);
      continue;
    }
    if (name.endsWith(".ts")) continue;
    cpSync(source, output);
  }
}

copyAssets(sourceRoot, outputRoot);

const publicProjectConfig = JSON.parse(readFileSync(path.join(packageRoot, "project.config.json"), "utf8"));
const appID = String(process.env.TB_MINIAPP_APP_ID || publicProjectConfig.appid || "").trim();
const channelKey = String(process.env.TB_MINIAPP_CHANNEL_KEY || "tanban-public").trim();
const defaultStoreCode = String(process.env.TB_MINIAPP_DEFAULT_STORE_CODE || "manong-coffee-gulou").trim();
if (!/^wx[a-zA-Z0-9]{16}$/.test(appID)) throw new Error("TB_MINIAPP_APP_ID must be a valid WeChat mini-program AppID");
if (!/^[a-zA-Z0-9_-]{3,64}$/.test(channelKey)) throw new Error("TB_MINIAPP_CHANNEL_KEY contains unsupported characters");
if (!/^[a-zA-Z0-9_-]{2,64}$/.test(defaultStoreCode)) throw new Error("TB_MINIAPP_DEFAULT_STORE_CODE contains unsupported characters");

const compiledEnvPath = path.join(outputRoot, "config", "env.js");
const compiledEnv = readFileSync(compiledEnvPath, "utf8")
  .replace('channelKey: "tanban-public"', `channelKey: ${JSON.stringify(channelKey)}`)
  .replace('defaultStoreCode: "manong-coffee-gulou"', `defaultStoreCode: ${JSON.stringify(defaultStoreCode)}`);
if (!compiledEnv.includes(`channelKey: ${JSON.stringify(channelKey)}`) || !compiledEnv.includes(`defaultStoreCode: ${JSON.stringify(defaultStoreCode)}`)) {
  throw new Error("mini-program channel build markers were not found");
}
writeFileSync(compiledEnvPath, compiledEnv);

writeFileSync(path.join(packageRoot, "dist", "project.config.json"), JSON.stringify({
  ...publicProjectConfig,
  appid: appID,
  projectname: String(process.env.TB_MINIAPP_PROJECT_NAME || publicProjectConfig.projectname || "tanban-customer-miniapp"),
  miniprogramRoot: "miniprogram/",
  srcMiniprogramRoot: "../miniprogram/",
}, null, 2));

for (const requiredFile of ["app.js", "app.json", "pages/home/index.js", "pages/home/index.wxml"]) {
  if (!existsSync(path.join(outputRoot, requiredFile))) {
    throw new Error(`mini-program build is missing ${requiredFile}`);
  }
}

console.log(`mini-program build ready: ${outputRoot} (${appID}, channel ${channelKey})`);
