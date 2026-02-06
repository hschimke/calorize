import { check_login } from "./auth.js";

async function main() {
    const app_container = document.getElementById("app");
    app_container.innerText = "Hello World";

    if (check_login()) {
        app_container.innerHTML = "Welcome to the Calorize, visit the <a href='/dashboard.html'>dashboard</a>.";
    } else {
        app_container.innerHTML = "You are not logged in. Please login or register at <a href='/login.html'>/login</a>.";
    }
}

main();