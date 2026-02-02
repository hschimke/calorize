function check_login() {
    return localStorage.getItem("token") !== null;
}

function set_login(response) {
    localStorage.setItem("token", response.token);
}

function unset_login() {
    localStorage.removeItem("token");
}

export { check_login, set_login, unset_login };
