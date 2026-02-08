import { check_login, unset_login } from './auth.js';
import { api } from './api.js';

async function bootstrap() {
    const isLoggedIn = check_login();
    const nav = document.createElement("nav");

    let navLinks = '';
    if (isLoggedIn) {
        navLinks = `
            <li><a href="/dashboard.html">Dashboard</a></li>
            <li><a href="/foodlog.html">Foodlog</a></li>
            <li><a href="/foods.html">Foods</a></li>
            <li><a href="/stats.html">Stats</a></li>
        `;
    } else {
        navLinks = `
            <li><a href="/index.html">Home</a></li>
            <li><a href="/login.html">Login/Register</a></li>
        `;
    }

    nav.innerHTML = `
        <div class="nav-content">
            <div class="logo">Calorize</div>
            <ul>
                ${navLinks}
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
