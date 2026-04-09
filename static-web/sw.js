// Replaced at build time by Dockerfile.web via --build-arg BUILD_HASH.
// In local dev this stays as-is ('__CACHE_VERSION__'), which is fine.
const CACHE_VERSION = '__CACHE_VERSION__';
const ASSETS_CACHE = `calorize-assets-${CACHE_VERSION}`;

// All static assets to precache on install.
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

// On install: precache all static assets and activate immediately.
self.addEventListener('install', (event) => {
    event.waitUntil(
        caches.open(ASSETS_CACHE).then((cache) => cache.addAll(PRECACHE_URLS))
    );
    // Take over without waiting for existing tabs to close.
    self.skipWaiting();
});

// On activate: delete any old asset caches from previous versions.
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
//   - API calls (/api/*)  → always network (no caching, fail normally if offline)
//   - Everything else     → cache-first, fall back to network
self.addEventListener('fetch', (event) => {
    const url = new URL(event.request.url);

    // Pass API calls straight through — no caching, no offline interference.
    if (url.pathname.startsWith('/api/')) return;

    event.respondWith(
        caches
            .match(event.request)
            .then((cached) => cached || fetch(event.request))
    );
});
