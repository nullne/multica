const CACHE_NAME = "multica-shell-v2";
const APP_SHELL_ASSETS = [
  "/offline",
  "/manifest.webmanifest",
  "/favicon.svg",
  "/apple-icon",
  "/pwa-icons/192",
  "/pwa-icons/512",
];
const STATIC_DESTINATIONS = new Set(["style", "script", "image", "font"]);

function isCacheableStaticAsset(request, url) {
  if (request.method !== "GET") {
    return false;
  }

  if (url.origin !== self.location.origin) {
    return false;
  }

  if (request.mode === "navigate") {
    return false;
  }

  if (url.pathname.startsWith("/api/")) {
    return false;
  }

  if (url.pathname.startsWith("/auth/")) {
    return false;
  }

  if (url.pathname.startsWith("/_next/data/")) {
    return false;
  }

  if (STATIC_DESTINATIONS.has(request.destination)) {
    return true;
  }

  return (
    url.pathname.startsWith("/_next/static/") ||
    url.pathname.startsWith("/pwa-icons/") ||
    url.pathname === "/manifest.webmanifest" ||
    url.pathname === "/favicon.svg" ||
    url.pathname === "/apple-icon"
  );
}

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
      fetch(request).catch(async () => caches.match("/offline")),
    );
    return;
  }

  if (!isCacheableStaticAsset(request, url)) {
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
