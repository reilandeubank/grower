/// <reference types="vitest/config" />
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { VitePWA } from "vite-plugin-pwa";

const backend = "http://localhost:8080";

export default defineConfig(({ mode }) => ({
  plugins: [
    react(),
    VitePWA({
      // The SW is not useful in unit tests and would only slow them down.
      disable: mode === "test",
      registerType: "autoUpdate",
      strategies: "injectManifest",
      srcDir: "src",
      filename: "sw.ts",
      injectRegister: "auto",
      includeAssets: ["favicon.svg", "icons/apple-touch-icon-180.png"],
      manifest: {
        name: "grower",
        short_name: "grower",
        description: "Your plants and what they need today.",
        start_url: "/",
        scope: "/",
        display: "standalone",
        theme_color: "#2e6b3f",
        background_color: "#f4f6f1",
        icons: [
          { src: "/icons/icon-192.png", sizes: "192x192", type: "image/png", purpose: "any" },
          { src: "/icons/icon-512.png", sizes: "512x512", type: "image/png", purpose: "any" },
          { src: "/icons/maskable-512.png", sizes: "512x512", type: "image/png", purpose: "maskable" },
        ],
      },
      // The worker itself (precache, offline caches, push) is src/sw.ts.
      injectManifest: {
        globPatterns: ["**/*.{js,css,html,svg,png,woff2,webmanifest}"],
      },
    }),
  ],
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
  server: {
    proxy: {
      "/api": { target: backend, changeOrigin: false },
      "/healthz": { target: backend },
    },
  },
  test: {
    environment: "jsdom",
    include: ["src/**/*.test.{ts,tsx}"],
    css: { modules: { classNameStrategy: "non-scoped" } },
    restoreMocks: true,
    unstubGlobals: true,
    setupFiles: ["src/test/setup.ts"],
  },
}));
