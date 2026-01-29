import { request } from './api.js';
import { bufferToBase64URL, base64URLToBuffer } from './utils.js';
import Logger from './logger.js';

export async function register(username, email) {
    try {
        Logger.info('Starting registration flow', { username, email });
        // 1. Begin Registration
        const startOpts = await request('/auth/register/begin', {
            method: 'POST',
            body: JSON.stringify({ username, email })
        });

        // Decode challenge and user.id
        startOpts.publicKey.challenge = base64URLToBuffer(startOpts.publicKey.challenge);
        startOpts.publicKey.user.id = base64URLToBuffer(startOpts.publicKey.user.id);

        Logger.debug('Prompting for credentials creation');
        // 2. Create Credentials (browser prompt)
        const credential = await navigator.credentials.create({ publicKey: startOpts.publicKey });

        // 3. Finish Registration
        const attestationResponse = {
            id: credential.id,
            rawId: bufferToBase64URL(credential.rawId),
            type: credential.type,
            response: {
                attestationObject: bufferToBase64URL(credential.response.attestationObject),
                clientDataJSON: bufferToBase64URL(credential.response.clientDataJSON),
            },
        };

        Logger.debug('Finishing registration');
        const response = await request('/auth/register/finish', {
            method: 'POST',
            body: JSON.stringify(attestationResponse)
        });

        Logger.info('Registration successful', response);
        return response; // Contains user_id (future)
    } catch (err) {
        Logger.error('Registration failed:', err);
        throw err;
    }
}

export async function login(username) {
    try {
        Logger.info('Starting login flow', { username });
        // 1. Begin Login
        const startOpts = await request(`/auth/login/begin?username=${encodeURIComponent(username)}`, {
            method: 'POST'
        });

        // Decode challenge and allowCredentials ids
        startOpts.publicKey.challenge = base64URLToBuffer(startOpts.publicKey.challenge);
        if (startOpts.publicKey.allowCredentials) {
            startOpts.publicKey.allowCredentials = startOpts.publicKey.allowCredentials.map(c => ({
                ...c,
                id: base64URLToBuffer(c.id)
            }));
        }

        Logger.debug('Prompting for assertion');
        // 2. Get Assertion (browser prompt)
        const assertion = await navigator.credentials.get({ publicKey: startOpts.publicKey });

        // 3. Finish Login
        const assertionResponse = {
            id: assertion.id,
            rawId: bufferToBase64URL(assertion.rawId),
            type: assertion.type,
            response: {
                authenticatorData: bufferToBase64URL(assertion.response.authenticatorData),
                clientDataJSON: bufferToBase64URL(assertion.response.clientDataJSON),
                signature: bufferToBase64URL(assertion.response.signature),
                userHandle: assertion.response.userHandle ? bufferToBase64URL(assertion.response.userHandle) : null,
            },
        };

        Logger.debug('Finishing login');
        const response = await request('/auth/login/finish', {
            method: 'POST',
            body: JSON.stringify(assertionResponse)
        });

        Logger.info('Login successful', response);
        return response; // Contains user_id (future)
    } catch (err) {
        Logger.error('Login failed:', err);
        throw err;
    }
}
