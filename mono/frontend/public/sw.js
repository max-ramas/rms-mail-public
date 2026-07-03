// Minimal service worker — required for PWA install prompt (beforeinstallprompt).
// Caching is handled by the browser's HTTP cache and Next.js static assets.
self.addEventListener("install", () => {
  self.skipWaiting();
});

self.addEventListener("activate", () => {
  self.clients.claim();
});

// Minimal fetch handler — same-origin only; never proxy cross-origin API calls.
self.addEventListener("fetch", (event) => {
  const url = new URL(event.request.url);
  if (url.origin !== self.location.origin) return;
  event.respondWith(fetch(event.request));
});
