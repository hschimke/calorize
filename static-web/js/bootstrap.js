import { check_login, unset_login } from './auth.js';
import { api } from './api.js';

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
        ];
    } else {
        navLinkSource = [
            { href: "/index.html", text: "Home" },
            { href: "/login.html", text: "Login/Register" },
        ];
    }

    nav.innerHTML = `
        <div class="nav-content">
            <div class="logo">Calorize</div>
            <ul>
                ${navLinkSource.map(link => `<li><a href="${link.href}">${link.text}</a></li>`).join('')}
            </ul>
            <div id="auth-action"></div>
        </div>
    `;

    document.body.prepend(nav);

    if (isLoggedIn) {
        render_logout_button(nav.querySelector('#auth-action'));
    }

    add_styles();
}

async function render_logout_button(parent_element) {
    const logout_button = document.createElement("button");
    logout_button.innerText = "Logout";
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

function add_styles() {
    const style = document.createElement('style');
    style.textContent = `
        body {
            margin: 0;
            font-family: sans-serif;
            color: #333;
        }
        nav {
            background-color: #333;
            color: white;
            padding: 1rem;
            margin-bottom: 2rem;
        }
        .nav-content {
            max-width: 800px;
            margin: 0 auto;
            display: flex;
            align-items: center;
            justify-content: space-between;
        }
        .logo {
            font-weight: bold;
            font-size: 1.2rem;
        }
        nav ul {
            list-style: none;
            padding: 0;
            margin: 0;
            display: flex;
            gap: 20px;
        }
        nav a {
            color: #fff;
            text-decoration: none;
            opacity: 0.8;
            transition: opacity 0.2s;
        }
        nav a:hover {
            opacity: 1;
        }
        .logout-btn {
            background: rgba(255, 255, 255, 0.1);
            border: 1px solid rgba(255, 255, 255, 0.2);
            color: white;
            padding: 5px 10px;
            cursor: pointer;
            border-radius: 4px;
        }
        .logout-btn:hover {
            background: rgba(255, 255, 255, 0.2);
        }
    `;
    document.head.appendChild(style);
}

bootstrap();
