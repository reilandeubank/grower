/// <reference lib="webworker" />
// The service worker (vite-plugin-pwa, injectManifest): the precached app shell, read-only
// offline caches for the API and photos, and the daily reminder's notifications and actions.
import { ExpirationPlugin } from "workbox-expiration";
import {
  cleanupOutdatedCaches,
  createHandlerBoundToURL,
  precacheAndRoute,
  type PrecacheEntry,
} from "workbox-precaching";
import { NavigationRoute, registerRoute } from "workbox-routing";
import { CacheFirst, NetworkFirst } from "workbox-strategies";
import { API_CACHE, MEDIA_CACHE } from "./lib/cacheNames";
import {
  appPath,
  LATER_ACTION,
  remindLater,
  toNotification,
} from "./lib/notification";

declare const self: ServiceWorkerGlobalScope & {
  __WB_MANIFEST: (PrecacheEntry | string)[];
};

// Updates apply at once (registerType "autoUpdate"): take over open pages without waiting.
void self.skipWaiting();
self.addEventListener("activate", (event) =>
  event.waitUntil(self.clients.claim()),
);

precacheAndRoute(self.__WB_MANIFEST);
cleanupOutdatedCaches();
registerRoute(
  new NavigationRoute(createHandlerBoundToURL("/index.html"), {
    denylist: [/^\/api\//, /^\/healthz/],
  }),
);

// Read-only offline: the last answers to the reads the screens make, and photo thumbnails.
// Saves (non-GET) and the export always go to the network. Logout deletes both caches.
const only200 = {
  cacheWillUpdate: async ({ response }: { response: Response }) =>
    response.status === 200 ? response : null,
};
registerRoute(
  ({ url, sameOrigin }) =>
    sameOrigin &&
    /^\/api\/v1\/(me|today|locations|plants(\/[^/]+(\/(tasks|events|photos))?)?)$/.test(
      url.pathname,
    ),
  new NetworkFirst({
    cacheName: API_CACHE,
    networkTimeoutSeconds: 3,
    plugins: [only200],
  }),
  "GET",
);
registerRoute(
  ({ url, sameOrigin }) =>
    sameOrigin && /^\/media\/photos\/[^/]+\/(small|medium)$/.test(url.pathname),
  new CacheFirst({
    cacheName: MEDIA_CACHE,
    plugins: [
      only200,
      new ExpirationPlugin({
        maxEntries: 300,
        maxAgeSeconds: 30 * 24 * 60 * 60,
      }),
    ],
  }),
  "GET",
);

self.addEventListener("push", (event) => {
  const { title, options } = toNotification(event.data?.text());
  event.waitUntil(self.registration.showNotification(title, options));
});

// "Remind me at" postpones the reminder without opening grower; a failure says why.
// Any other tap opens the notification's page in an open grower window, or a new one.
self.addEventListener("notificationclick", (event) => {
  event.notification.close();
  const data = event.notification.data as {
    url?: unknown;
    date?: unknown;
  } | null;
  if (event.action === LATER_ACTION && typeof data?.date === "string") {
    event.waitUntil(
      remindLater(data.date).then(
        (failed) =>
          failed &&
          self.registration.showNotification(failed.title, failed.options),
      ),
    );
    return;
  }
  const url = new URL(appPath(data?.url), self.location.origin).href;
  event.waitUntil(
    (async () => {
      const windows = await self.clients.matchAll({
        type: "window",
        includeUncontrolled: true,
      });
      const open = windows.find(
        (w) => new URL(w.url).origin === self.location.origin,
      );
      if (!open) {
        await self.clients.openWindow(url);
        return;
      }
      const focused = await open.focus();
      await focused.navigate(url).catch(() => self.clients.openWindow(url));
    })(),
  );
});
