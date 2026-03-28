/**
 * API Client for Calorize
 * Handles all server communication including Authentication (WebAuthn), Foods, Logs, and Stats.
 */
export class API {
    constructor(baseUrl = '/api/v1') {
        this.baseUrl = baseUrl;
    }

    /**
     * Generic request helper
     * @param {string} endpoint 
     * @param {string} method 
     * @param {object} data 
     */
    async request(endpoint, method = 'GET', data = null) {
        const url = `${this.baseUrl}${endpoint}`;
        const options = {
            method,
            headers: {
                'Content-Type': 'application/json',
            },
        };

        if (data) {
            options.body = JSON.stringify(data);
        }

        const response = await fetch(url, options);

        if (!response.ok) {
            const errorText = await response.text();
            throw new Error(`API Error (${response.status}): ${errorText}`);
        }

        // Return null for 204 No Content, otherwise parse JSON
        if (response.status === 204) {
            return null;
        }

        // Check if content-length is 0
        const contentLength = response.headers.get("Content-Length");
        if (contentLength === "0") {
            return null;
        }

        try {
            return await response.json();
        } catch (e) {
            console.warn("Response was not JSON", e);
            return null;
        }
    }

    // --- Authentication (WebAuthn) ---

    /**
     * Helper to encode ArrayBuffer to Base64URL string
     */
    bufferToBase64URL(buffer) {
        const bytes = new Uint8Array(buffer);
        let string = '';
        for (let i = 0; i < bytes.byteLength; i++) {
            string += String.fromCharCode(bytes[i]);
        }
        return btoa(string)
            .replace(/\+/g, '-')
            .replace(/\//g, '_')
            .replace(/=/g, '');
    }

    /**
     * Helper to decode Base64URL string to ArrayBuffer
     */
    base64URLToBuffer(base64URL) {
        const base64 = base64URL.replace(/-/g, '+').replace(/_/g, '/');
        const padLen = (4 - (base64.length % 4)) % 4;
        const padded = base64.padEnd(base64.length + padLen, '=');
        const binary = atob(padded);
        const bytes = new Uint8Array(binary.length);
        for (let i = 0; i < binary.length; i++) {
            bytes[i] = binary.charCodeAt(i);
        }
        return bytes.buffer;
    }

    async register(username, user_email) {
        // 1. Begin Registration
        // Note: The server expects query param for username in begin
        const options = await this.request(`/auth/register/begin?username=${encodeURIComponent(username)}&user_email=${encodeURIComponent(user_email)}`, 'POST');

        // Decode challenge and user.id
        options.publicKey.challenge = this.base64URLToBuffer(options.publicKey.challenge);
        options.publicKey.user.id = this.base64URLToBuffer(options.publicKey.user.id);

        if (options.publicKey.excludeCredentials) {
            for (let cred of options.publicKey.excludeCredentials) {
                cred.id = this.base64URLToBuffer(cred.id);
            }
        }

        // 2. Create Credential
        const credential = await navigator.credentials.create({
            publicKey: options.publicKey
        });

        // Encode response for server
        const credentialForServer = {
            id: credential.id,
            rawId: this.bufferToBase64URL(credential.rawId),
            response: {
                attestationObject: this.bufferToBase64URL(credential.response.attestationObject),
                clientDataJSON: this.bufferToBase64URL(credential.response.clientDataJSON),
            },
            type: credential.type,
        };

        // 3. Finish Registration
        return await this.request(`/auth/register/finish?username=${encodeURIComponent(username)}`, 'POST', credentialForServer);
    }

    async login(username) {
        // 1. Begin Login
        const options = await this.request(`/auth/login/begin?username=${encodeURIComponent(username)}`, 'POST');

        // Decode challenge
        options.publicKey.challenge = this.base64URLToBuffer(options.publicKey.challenge);
        if (options.publicKey.allowCredentials) {
            for (let cred of options.publicKey.allowCredentials) {
                cred.id = this.base64URLToBuffer(cred.id);
            }
        }

        // 2. Get Credential
        const assertion = await navigator.credentials.get({
            publicKey: options.publicKey
        });

        // Encode response for server
        const assertionForServer = {
            id: assertion.id,
            rawId: this.bufferToBase64URL(assertion.rawId),
            response: {
                authenticatorData: this.bufferToBase64URL(assertion.response.authenticatorData),
                clientDataJSON: this.bufferToBase64URL(assertion.response.clientDataJSON),
                signature: this.bufferToBase64URL(assertion.response.signature),
                userHandle: assertion.response.userHandle ? this.bufferToBase64URL(assertion.response.userHandle) : null,
            },
            type: assertion.type,
        };

        // 3. Finish Login
        return await this.request('/auth/login/finish', 'POST', assertionForServer);
    }

    async logout() {
        return await this.request('/auth/logout', 'POST');
    }

    // --- Foods ---

    async getFoods(params = {}) {
        const qs = new URLSearchParams(params).toString();
        return await this.request('/foods' + (qs ? '?' + qs : ''));
    }

    async createFood(foodData) {
        return await this.request('/foods', 'POST', foodData);
    }

    async getFood(id) {
        return await this.request(`/foods/${id}`);
    }

    async updateFood(id, foodData) {
        return await this.request(`/foods/${id}`, 'PUT', foodData);
    }

    async deleteFood(id) {
        return await this.request(`/foods/${id}`, 'DELETE');
    }

    // --- Logs ---

    async getLogs(date) {
        let url = '/logs';
        let params = new URLSearchParams({ tz_offset: new Date().getTimezoneOffset() });
        if (date) {
            params.append('date', date);
        }
        url += `?${params.toString()}`;
        return await this.request(url);
    }

    /**
     * Create a log entry.
     * @param {Object} logData - Log entry data.
     * @param {string} [logData.food_id] - UUID of the food (optional if calories provided).
     * @param {number} [logData.calories] - Quick add calories (required if food_id is null).
     * @param {number} logData.amount - Amount (multiplier).
     * @param {string} logData.meal_tag - Meal tag (e.g. 'breakfast').
     */
    async createLog(logData) {
        return await this.request('/logs', 'POST', logData);
    }

    async deleteLog(id) {
        return await this.request(`/logs/${id}`, 'DELETE');
    }

    // --- Account / Passkeys ---

    async getPasskeys() {
        return await this.request('/account/passkeys');
    }

    async addPasskey() {
        // 1. Begin registration
        const options = await this.request('/account/passkeys/begin', 'POST');

        options.publicKey.challenge = this.base64URLToBuffer(options.publicKey.challenge);
        options.publicKey.user.id = this.base64URLToBuffer(options.publicKey.user.id);
        if (options.publicKey.excludeCredentials) {
            for (let cred of options.publicKey.excludeCredentials) {
                cred.id = this.base64URLToBuffer(cred.id);
            }
        }

        // 2. Create credential via browser
        const credential = await navigator.credentials.create({ publicKey: options.publicKey });

        // 3. Finish registration
        const credentialForServer = {
            id: credential.id,
            rawId: this.bufferToBase64URL(credential.rawId),
            response: {
                attestationObject: this.bufferToBase64URL(credential.response.attestationObject),
                clientDataJSON: this.bufferToBase64URL(credential.response.clientDataJSON),
            },
            type: credential.type,
        };
        return await this.request('/account/passkeys/finish', 'POST', credentialForServer);
    }

    async deletePasskey(id) {
        return await this.request(`/account/passkeys/${encodeURIComponent(id)}`, 'DELETE');
    }

    async renamePasskey(id, name) {
        return await this.request(`/account/passkeys/${encodeURIComponent(id)}`, 'PATCH', { name });
    }

    // --- Stats ---

    async getStats(period, date) {
        const params = new URLSearchParams({ period, tz_offset: new Date().getTimezoneOffset() });
        if (date) {
            params.append('date', date);
        }
        return await this.request(`/stats?${params.toString()}`);
    }

    async getStatsBreakdown(period, date) {
        const params = new URLSearchParams({ period, tz_offset: new Date().getTimezoneOffset() });
        if (date) {
            params.append('date', date);
        }
        return await this.request(`/stats/breakdown?${params.toString()}`);
    }
}

// Export singleton instance
export const api = new API();
