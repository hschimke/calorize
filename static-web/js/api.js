import Config from './config.js';
import Logger from './logger.js';

export async function request(endpoint, options = {}) {
    // Simple ID for tracing
    const requestId = Math.random().toString(36).substring(7);
    const url = endpoint.startsWith('http') ? endpoint : `${Config.API_BASE_URL}${endpoint}`;

    Logger.debug(`[REQ:${requestId}] Starting request`, { url, method: options.method || 'GET', options });

    const headers = {
        'Content-Type': 'application/json',
        ...options.headers,
    };

    const config = {
        ...options,
        headers,
    };

    try {
        const response = await fetch(url, config);

        Logger.debug(`[REQ:${requestId}] Response received`, { status: response.status, statusText: response.statusText });

        if (response.status === 401) {
            Logger.warn(`[REQ:${requestId}] 401 Unauthorized - Checking for redirect`);
            // Redirect to login if unauthorized, unless we are already on login/register/index
            const path = window.location.pathname;
            if (!path.endsWith('login.html') && !path.endsWith('register.html') && path !== '/' && !path.endsWith('index.html')) {
                window.location.href = '/login.html';
            }
        }

        if (!response.ok) {
            const text = await response.text();
            Logger.error(`[REQ:${requestId}] Request failed`, { status: response.status, error: text });
            throw new Error(text || response.statusText);
        }

        // Attempt to parse JSON, fall back to text if empty or not json
        try {
            const data = await response.json();
            Logger.debug(`[REQ:${requestId}] JSON parsed successfully`, data);
            return data;
        } catch (e) {
            Logger.debug(`[REQ:${requestId}] Response was not JSON or empty`, e);
            return null;
        }
    } catch (error) {
        Logger.error(`[REQ:${requestId}] Network or parsing error`, error);
        throw error;
    }
}
