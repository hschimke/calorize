package auth

import "net/http"

const UserIDContextKey = "user_id"

// ### Auth
// - POST /auth/register/begin
// - POST /auth/register/finish
// - POST /auth/login/begin
// - POST /auth/login/finish

func RegisterAuthPaths(mux *http.ServeMux) {
	mux.HandleFunc("/auth/register/begin", registerBeginHandler)
	mux.HandleFunc("/auth/register/finish", registerFinishHandler)
	mux.HandleFunc("/auth/login/begin", loginBeginHandler)
	mux.HandleFunc("/auth/login/finish", loginFinishHandler)
}

func registerBeginHandler(w http.ResponseWriter, r *http.Request)  {}
func registerFinishHandler(w http.ResponseWriter, r *http.Request) {}
func loginBeginHandler(w http.ResponseWriter, r *http.Request)     {}
func loginFinishHandler(w http.ResponseWriter, r *http.Request)    {}
