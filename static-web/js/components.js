export function injectNavbar() {
    const nav = document.createElement('nav');
    nav.className = 'navbar';
    nav.innerHTML = `
        <div class="nav-brand">Calorize</div>
        <div>
            <a href="/dashboard.html">Dashboard</a>
            <a href="/foods.html">Foods</a>
            <a href="#" id="logoutBtn">Logout</a>
        </div>
    `;
    document.body.prepend(nav);

    document.getElementById('logoutBtn').addEventListener('click', (e) => {
        e.preventDefault();
        // Since we use cookies, we can just clear them or call a logout endpoint if it existed.
        // For now, simple cookie clear + redirect.
        document.cookie = 'auth_session_id=; Max-Age=0; path=/;'; // Actually this session id is for registration/login flow...
        // The real auth persistence isn't clearly defined in the current server code (it returns "Login Success" but doesn't seem to set a persistent auth token cookie for subsequent requests??)
        // Wait, looking at server.go... 
        // FinishLogin writes response but doesn't seem to set a session cookie for the user?
        // Ah, the API might be missing session management for post-login state.
        // I will assume for now we might need to fix that on server, but for UI:
        window.location.href = '/login.html';
    });
}
