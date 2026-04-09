// Replaced at build time by Dockerfile.web via --build-arg BUILD_HASH.
// In local dev this stays as-is ('__CACHE_VERSION__'), which is fine.
const CACHE_VERSION = '__CACHE_VERSION__';
const ASSETS_CACHE = `calorize-assets-${CACHE_VERSION}`;

// All static assets to precache on install.
// NOTE: On first registration this triggers ~30 parallel network requests.
// This is standard PWA behaviour and is acceptable for a private app; all
// requests are small and happen in the background after the page has loaded.
const PRECACHE_URLS = [
    '/',
    '/index.html',
    '/login.html',
    '/dashboard.html',
    '/food-ui.html',
    '/foodlog.html',
    '/stat-ui.html',
    '/account.html',
    '/css/main.css',
    '/js/bootstrap.js',
    '/js/api.js',
    '/js/auth.js',
    '/js/utils.js',
    '/js/ui.js',
    '/js/dashboard.js',
    '/js/food-ui.js',
    '/js/food-search.js',
    '/js/foodlog.js',
    '/js/stat-ui.js',
    '/js/charts.js',
    '/js/account.js',
    '/js/index.js',
    '/js/login.js',
    '/manifest.json',
    '/icons/icon-192.png',
    '/icons/icon-512.png',
];

// On install: precache all static assets, then activate immediately.
// skipWaiting() is chained inside waitUntil so the SW only moves to the
// activate phase after cache.addAll() has fully resolved. Calling it outside
// the promise would let activation race ahead of a complete cache population.
self.addEventListener('install', (event) => {
    event.waitUntil(
        caches
            .open(ASSETS_CACHE)
            .then((cache) => cache.addAll(PRECACHE_URLS))
            .then(() => self.skipWaiting())
    );
});

// On activate: delete any old asset caches from previous versions.
// Trade-off: deleting caches before new content is served means a tab that
// opens in the narrow window between activate and the first cache-hit will
// fall through to the network. This is acceptable — the app requires
// connectivity and avoids serving stale assets from a previous version.
self.addEventListener('activate', (event) => {
    event.waitUntil(
        caches.keys().then((keys) =>
            Promise.all(
                keys
                    .filter((k) => k !== ASSETS_CACHE)
                    .map((k) => caches.delete(k))
            )
        )
    );
    // Claim all open tabs immediately so updated assets take effect.
    self.clients.claim();
});

// Fetch strategy:
//   - API calls (/api/*)  → always network (no caching, no offline interference)
//   - Everything else     → cache-first, fall back to network
//
// If an asset is not in the cache AND the user is offline, fetch() will reject
// and the browser shows its default network-error page. This is intentional —
// this PWA does not support offline data access.
self.addEventListener('fetch', (event) => {
    const url = new URL(event.request.url);

    // Pass API calls straight through — returning without calling
    // event.respondWith() hands control back to the browser for a normal
    // network request, ensuring auth tokens are never cached.
    if (url.pathname.startsWith('/api/')) return;

    event.respondWith(
        caches
            .match(event.request)
            .then((cached) => cached || fetch(event.request))
    );
});
