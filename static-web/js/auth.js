function check_login() {
    return localStorage.getItem("token") !== null;
}

function set_login(response) {
    localStorage.setItem("token", response.token);
    localStorage.setItem("user_id", response.user_id);
}

function unset_login() {
    localStorage.removeItem("token");
    localStorage.removeItem("user_id");
}

function get_user_id() {
    return localStorage.getItem("user_id");
}

export { check_login, set_login, unset_login, get_user_id };
