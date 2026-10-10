import { fileURLToPath } from "node:url";

import { defineConfig } from "vitest/config";

// 让测试能解析源码里的 "@/..." 路径别名（与 tsconfig paths 一致）。
export default defineConfig({
  resolve: {
    alias: {
      "@": fileURLToPath(new URL(".", import.meta.url)),
    },
  },
  test: {
    environment: "node",
  },
});
