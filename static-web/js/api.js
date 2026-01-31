/**
 * API Client for Calorize
 * Handles all server communication including Authentication (WebAuthn), Foods, Logs, and Stats.
 */
export class API {
    constructor(baseUrl = '') {
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

    async register(username) {
        // 1. Begin Registration
        // Note: The server expects query param for username in begin
        const options = await this.request(`/auth/register/begin?username=${encodeURIComponent(username)}`, 'POST');

        // Decode challenge and user.id
        options.challenge = this.base64URLToBuffer(options.publicKey.challenge);
        options.user.id = this.base64URLToBuffer(options.publicKey.user.id);
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
        options.challenge = this.base64URLToBuffer(options.publicKey.challenge);
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
        return await this.request(`/auth/login/finish?username=${encodeURIComponent(username)}`, 'POST', assertionForServer);
    }

    async logout() {
        return await this.request('/auth/logout', 'POST');
    }

    // --- Foods ---

    async getFoods() {
        return await this.request('/foods');
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
        if (date) {
            url += `?date=${date}`;
        }
        return await this.request(url);
    }

    async createLog(logData) {
        return await this.request('/logs', 'POST', logData);
    }

    async deleteLog(id) {
        return await this.request(`/logs/${id}`, 'DELETE');
    }

    // --- Stats ---

    async getStats(period, date) {
        const params = new URLSearchParams({ period, date });
        return await this.request(`/stats?${params.toString()}`);
    }
}

// Export singleton instance
export const api = new API();
