import { api } from "./api.js";
import { set_login } from "./auth.js";
import { showToast } from "./ui.js";

function main() {
    const login_form = document.getElementById("login-form");
    const register_form = document.getElementById("register-form");

    login_form.addEventListener("submit", (e) => {
        e.preventDefault();
        const username = login_form.username.value;
        login(username);
    });

    register_form.addEventListener("submit", (e) => {
        e.preventDefault();
        const username = register_form.username.value;
        const email = register_form.email.value;
        register(username, email);
    });
}

async function login(username) {
    try {
        const response = await api.login(username);
        set_login(response);
        window.location.href = '/dashboard.html';
    } catch (e) {
        showToast("Login failed: " + e.message, 'error');
    }
}

async function register(username, email) {
    try {
        const response = await api.register(username, email);
        set_login(response);
        window.location.href = '/dashboard.html';
    } catch (e) {
        showToast("Registration failed: " + e.message, 'error');
    }
}

main();