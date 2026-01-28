const BASE_URL = 'http://localhost:8383';

export async function request(endpoint, options = {}) {
    const url = endpoint.startsWith('http') ? endpoint : `${BASE_URL}${endpoint}`;
    const headers = {
        'Content-Type': 'application/json',
        ...options.headers,
    };

    const config = {
        ...options,
        headers,
    };

    const response = await fetch(url, config);

    if (response.status === 401) {
        // Redirect to login if unauthorized, unless we are already on login/register/index
        const path = window.location.pathname;
        if (!path.endsWith('login.html') && !path.endsWith('register.html') && path !== '/' && !path.endsWith('index.html')) {
            window.location.href = '/login.html';
        }
    }

    if (!response.ok) {
        const text = await response.text();
        throw new Error(text || response.statusText);
    }

    // Attempt to parse JSON, fall back to text if empty or not json
    try {
        return await response.json();
    } catch {
        return null;
    }
}
