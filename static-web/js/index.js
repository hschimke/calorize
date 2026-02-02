import { check_login } from "./auth.js";

async function main() {
    const app_container = document.getElementById("app");
    app_container.innerText = "Hello World";

    if (check_login()) {
        app_container.innerText = "Welcome to the Calorize dashboard.";
    } else {
        app_container.innerText = "You are not logged in.";
    }
}

main();