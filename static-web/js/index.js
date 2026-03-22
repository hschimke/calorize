import { check_login } from "./auth.js";

async function main() {
    const app_container = document.getElementById("app");
    app_container.textContent = "";

    if (check_login()) {
        app_container.appendChild(document.createTextNode("Welcome to Calorize, visit the "));
        const link = document.createElement("a");
        link.href = "/dashboard.html";
        link.textContent = "dashboard";
        app_container.appendChild(link);
        app_container.appendChild(document.createTextNode("."));
    } else {
        app_container.appendChild(document.createTextNode("You are not logged in. Please login or register at "));
        const link = document.createElement("a");
        link.href = "/login.html";
        link.textContent = "/login";
        app_container.appendChild(link);
        app_container.appendChild(document.createTextNode("."));
    }
}

main();