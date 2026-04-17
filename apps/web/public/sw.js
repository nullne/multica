const CACHE_NAME = "multica-shell-v1";
const APP_SHELL_ASSETS = [
  "/",
  "/issues",
  "/offline",
  "/manifest.webmanifest",
  "/favicon.svg",
  "/apple-icon",
  "/pwa-icons/192",
  "/pwa-icons/512",
];

self.addEventListener("install", (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => cache.addAll(APP_SHELL_ASSETS)),
  );
  self.skipWaiting();
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches.keys().then((keys) =>
      Promise.all(
        keys
          .filter((key) => key !== CACHE_NAME)
          .map((key) => caches.delete(key)),
      ),
    ),
  );
  self.clients.claim();
});

self.addEventListener("fetch", (event) => {
  const { request } = event;

  if (request.method !== "GET") {
    return;
  }

  const url = new URL(request.url);

  if (request.mode === "navigate") {
    event.respondWith(
      fetch(request).catch(async () => {
        return (
          (await caches.match(request)) ||
          (await caches.match("/offline"))
        );
      }),
    );
    return;
  }

  if (url.origin !== self.location.origin) {
    return;
  }

  event.respondWith(
    caches.match(request).then(async (cachedResponse) => {
      if (cachedResponse) {
        return cachedResponse;
      }

      const response = await fetch(request);
      if (response.ok && request.destination !== "document") {
        const cache = await caches.open(CACHE_NAME);
        cache.put(request, response.clone());
      }
      return response;
    }).catch(async () => caches.match("/offline")),
  );
});
