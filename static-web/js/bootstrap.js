import { check_login, unset_login } from './auth.js';
import { api } from './api.js';
import { getLocalDateString } from './utils.js';

async function bootstrap() {
    const isLoggedIn = check_login();
    const nav = document.createElement("nav");

    let navLinkSource = [];
    if (isLoggedIn) {
        navLinkSource = [
            { href: "/dashboard.html", text: "Dashboard" },
            { href: "/foodlog.html", text: "Foodlog" },
            { href: "/food-ui.html", text: "Foods" },
            { href: "/stat-ui.html", text: "Stats" },
            { href: "/account.html", text: "Account" },
        ];
    } else {
        navLinkSource = [
            { href: "/index.html", text: "Home" },
            { href: "/login.html", text: "Login/Register" },
        ];
    }

    const navContent = document.createElement("div");
    navContent.className = "nav-content";

    const logo = document.createElement("div");
    logo.className = "logo";
    logo.textContent = "Calorize";

    // Hamburger toggle (visible only on mobile via CSS)
    const toggle = document.createElement("button");
    toggle.className = "nav-toggle";
    toggle.setAttribute("aria-label", "Toggle navigation");
    toggle.textContent = "☰";
    toggle.addEventListener("click", () => {
        nav.classList.toggle("nav-open");
    });

    const ul = document.createElement("ul");
    navLinkSource.forEach(link => {
        const li = document.createElement("li");
        const a = document.createElement("a");
        a.href = link.href;
        a.textContent = link.text;
        // Close mobile menu on link tap
        a.addEventListener("click", () => {
            nav.classList.remove("nav-open");
        });
        li.appendChild(a);
        ul.appendChild(li);
    });

    const authAction = document.createElement("div");
    authAction.id = "auth-action";

    navContent.appendChild(logo);
    navContent.appendChild(toggle);
    navContent.appendChild(ul);
    navContent.appendChild(authAction);
    nav.appendChild(navContent);

    document.body.prepend(nav);

    // Highlight the active nav link
    const currentPath = window.location.pathname;
    nav.querySelectorAll('a').forEach(a => {
        if (a.getAttribute('href') === currentPath) {
            a.classList.add('active');
        }
    });

    if (isLoggedIn) {
        render_logout_button(authAction);

        // Validate that the cookie-based session is still active.
        // If the server returns 401, clear the stale localStorage token.
        try {
            await api.getStats('day', getLocalDateString());
        } catch (e) {
            if (e.message && e.message.includes('401')) {
                unset_login();
                window.location.href = '/login.html';
                return;
            }
        }
    }
}

async function render_logout_button(parent_element) {
    const logout_button = document.createElement("button");
    logout_button.textContent = "Logout";
    logout_button.className = "logout-btn";
    logout_button.addEventListener("click", () => {
        logout();
    });
    parent_element.appendChild(logout_button);
}

async function logout() {
    try {
        await api.logout();
    } catch (e) {
        console.warn("Logout failed on server, clearing local state anyway", e);
    }
    unset_login();
    window.location.href = '/index.html';
}

bootstrap();
