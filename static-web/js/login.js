import { api } from "./api.js";
import { set_login } from "./auth.js";

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
    console.log(username);
    const response = await api.login(username);
    set_login(response);
}

async function register(username, email) {
    console.log(username, email);
    const response = await api.register(username, email);
    set_login(response);
}

main();