package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

type user struct {
	email    string
	password string
}

type handler struct {
	secretKet string
	db        interface {
		validateUserLogin(email, password string) bool
		getUserUUIDByEmail(email string) string
		getUserRoles(uuid string) []string
	}
}

func (h handler) handleLogin(w http.ResponseWriter, r *http.Request) {

	if r.Method != "POST" {
		respondWithMessage(w, "Only POST methods is allowed", 400)
		return
	}

	var u user
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		respondWithMessage(w, "Invalid body", 400)
		return
	}

	if u.email == "" || u.password == "" {
		respondWithMessage(w, "Email or password is empty", 403)
		return
	}

	if !h.db.validateUserLogin(u.email, u.password) {
		respondWithMessage(w, "Incorrect email or password", 403)
		return
	}

	uuid := h.db.getUserUUIDByEmail(u.email)
	token := jwt.New(jwt.SigningMethodHS256)
	claims := token.Claims.(jwt.MapClaims)

	claims["roles"] = h.db.getUserRoles(uuid)
	claims["uuid"] = uuid
	claims["exp"] = time.Now().Add(time.Minute * 30).Unix()

	tokenString, err := token.SignedString(h.secretKet)
	if err != nil {
		respondWithMessage(w, "Something went wrong", 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Authorization", tokenString)
	w.WriteHeader(200)
}

func respondWithMessage(w http.ResponseWriter, messageTxt string, statusCode int) {
	type messageResponse struct {
		Message string `json:"message"`
	}

	mess := &messageResponse{Message: messageTxt}

	resp, _ := json.Marshal(mess)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_, _ = w.Write(resp)
}
