async function bootstrap() {
    const nav = document.createElement("nav");
    nav.innerHTML = `
        <ul>
            <li><a href="/index.html">Home</a></li>
            <li><a href="/dashboard.html">Dashboard</a></li>
            <li><a href="/foodlog.html">Foodlog</a></li>
            <li><a href="/foods.html">Foods</a></li>
            <li><a href="/stats.html">Stats</a></li>
        </ul>
    `;
    render_logout_button(nav);
    document.body.prepend(nav);
}

async function render_logout_button(parent_element = document.body) {
    const logout_button = document.createElement("button");
    logout_button.innerText = "Logout";
    logout_button.addEventListener("click", () => {
        logout();
    });
    parent_element.appendChild(logout_button);
}

bootstrap();
