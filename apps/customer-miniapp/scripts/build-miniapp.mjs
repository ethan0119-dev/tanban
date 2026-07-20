import { execFileSync } from "node:child_process";
import { cpSync, existsSync, mkdirSync, readdirSync, rmSync, statSync } from "node:fs";
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

for (const requiredFile of ["app.js", "app.json", "pages/home/index.js", "pages/home/index.wxml"]) {
  if (!existsSync(path.join(outputRoot, requiredFile))) {
    throw new Error(`mini-program build is missing ${requiredFile}`);
  }
}

console.log(`mini-program build ready: ${outputRoot}`);
