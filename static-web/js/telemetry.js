(function() {
  const seenErrors = new Set();

  function getBuild() {
    return document.querySelector('meta[name="calorize-build"]')?.content || 'unknown';
  }

  function sendEvent(kind, message, location) {
    const key = `${kind}:${message}:${location}`;
    if (seenErrors.has(key)) return;
    seenErrors.add(key);

    const payload = JSON.stringify({
      events: [{
        kind: kind,
        message: String(message || 'unknown error'),
        location: location || '',
        route: window.location.pathname,
        build: getBuild(),
        ts: Date.now()
      }]
    });

    if (navigator.sendBeacon) {
      let sent = false;
      try {
        const blob = new Blob([payload], { type: 'application/json' });
        sent = navigator.sendBeacon('/api/v1/client-events', blob);
      } catch (e) {
        sent = false;
      }
      if (!sent) {
        fetch('/api/v1/client-events', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: payload,
          keepalive: true
        }).catch(() => {});
      }
    } else {
      fetch('/api/v1/client-events', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: payload,
        keepalive: true
      }).catch(() => {});
    }
  }

  window.addEventListener('error', function(e) {
    const msg = e.message || (e.error && e.error.message) || 'Script error';
    const loc = e.filename ? `${e.filename}:${e.lineno}:${e.colno || 0}` : '';
    sendEvent('js_error', msg, loc);
  });

  window.addEventListener('unhandledrejection', function(e) {
    const reason = e.reason;
    const msg = (reason && (reason.message || reason.stack)) || String(reason || 'Unhandled rejection');
    sendEvent('unhandled_rejection', msg, '');
  });
})();
