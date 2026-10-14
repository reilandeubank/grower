/// <reference types="vitest/config" />
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { VitePWA } from "vite-plugin-pwa";
import { API_CACHE, MEDIA_CACHE } from "./src/lib/cacheNames";

const backend = "http://localhost:8080";

export default defineConfig(({ mode }) => ({
  plugins: [
    react(),
    VitePWA({
      // The SW is not useful in unit tests and would only slow them down.
      disable: mode === "test",
      registerType: "autoUpdate",
      strategies: "generateSW",
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
      workbox: {
        globPatterns: ["**/*.{js,css,html,svg,png,woff2,webmanifest}"],
        navigateFallback: "/index.html",
        navigateFallbackDenylist: [/^\/api\//, /^\/healthz/],
        cleanupOutdatedCaches: true,
        // Read-only offline: the last answers to the reads the screens make, and photo
        // thumbnails. Saves (non-GET) and the export always go to the network. The matchers are
        // copied into the service worker as source, so they can't use variables from this file.
        runtimeCaching: [
          {
            urlPattern: ({ url, sameOrigin }) =>
              sameOrigin &&
              /^\/api\/v1\/(me|today|locations|plants(\/[^/]+(\/(tasks|events|photos))?)?)$/.test(url.pathname),
            handler: "NetworkFirst",
            method: "GET",
            options: { cacheName: API_CACHE, networkTimeoutSeconds: 3, cacheableResponse: { statuses: [200] } },
          },
          {
            urlPattern: ({ url, sameOrigin }) => sameOrigin && /^\/media\/photos\/[^/]+\/(small|medium)$/.test(url.pathname),
            handler: "CacheFirst",
            method: "GET",
            options: {
              cacheName: MEDIA_CACHE,
              expiration: { maxEntries: 300, maxAgeSeconds: 30 * 24 * 60 * 60 },
              cacheableResponse: { statuses: [200] },
            },
          },
        ],
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
