"use client";

import { useEffect } from "react";

export function ServiceWorkerRegistration() {
  useEffect(() => {
    if (typeof window === "undefined" || !("serviceWorker" in navigator)) {
      return;
    }

    // In dev, the service worker must not intercept requests: cached
    // /_next/ chunks would keep serving stale code across rebuilds.
    // Unregister any previously installed worker and drop its caches.
    if (process.env.NODE_ENV !== "production") {
      const cleanup = async () => {
        const registrations = await navigator.serviceWorker.getRegistrations();
        await Promise.all(registrations.map((r) => r.unregister()));
        if ("caches" in window) {
          const keys = await caches.keys();
          await Promise.all(keys.map((key) => caches.delete(key)));
        }
      };
      cleanup().catch(() => {});
      return;
    }

    const register = async () => {
      try {
        await navigator.serviceWorker.register("/sw.js", { scope: "/" });
      } catch (error) {
        console.error("Failed to register service worker", error);
      }
    };

    register();
  }, []);

  return null;
}
