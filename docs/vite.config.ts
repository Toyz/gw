import { defineConfig } from "vite";

// gw ships as a GitHub Pages project site at https://toyz.github.io/gw/, so the
// build must be served from the "/gw/" base path.
export default defineConfig({
  base: "/gw/",
  esbuild: {
    jsx: "automatic",
    jsxImportSource: "@toyz/loom",
    target: "es2022",
    keepNames: true,
  },
});
