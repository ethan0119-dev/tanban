import { defineConfig, globalIgnores } from "eslint/config";
import nextVitals from "eslint-config-next/core-web-vitals";
import nextTs from "eslint-config-next/typescript";

const eslintConfig = defineConfig([
  ...nextVitals,
  ...nextTs,
  // Override default ignores of eslint-config-next.
  globalIgnores([
    // Default ignores of eslint-config-next:
    ".next/**",
    "out/**",
    "build/**",
    "dist/**",
    "**/dist/**",
    ".vinext/**",
    ".wrangler/**",
    "next-env.d.ts",
    "**/*.tsbuildinfo",
  ]),
  {
    // All three clients load their initial remote state in effects. React's
    // experimental rule treats these intentional async loaders as cascading
    // synchronous state updates, even though the updates happen after I/O.
    rules: { "react-hooks/set-state-in-effect": "off" },
  },
]);

export default eslintConfig;
