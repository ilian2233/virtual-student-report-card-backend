package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
)

type user struct {
	email    string
	password string
}

type handler struct {
	secretKet string
	db        interface {
		validateUserLogin(email, password string) bool
		getUserUUIDByEmail(email string) (string, error)
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

	id, err := h.db.getUserUUIDByEmail(u.email)
	if err != nil {
		log.Printf("Failed extracting uuid by email, \n%e", err)
		respondWithMessage(w, "Something went wrong", 500)
		return
	}

	token := jwt.New(jwt.SigningMethodHS256)
	claims := token.Claims.(jwt.MapClaims)

	claims["roles"] = h.db.getUserRoles(id)
	claims["uuid"] = id
	claims["exp"] = time.Now().Add(time.Minute * 30).Unix()

	tokenString, err := token.SignedString(h.secretKet)
	if err != nil {
		log.Printf("Failed generating token, \n%e", err)
		respondWithMessage(w, "Something went wrong", 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Authorization", tokenString)
	w.WriteHeader(200)
}

func (h handler) getStudentExams(w http.ResponseWriter, r *http.Request) {

	token, err := validateToken(r.Header)
	if err != nil {
		respondWithMessage(w, "unauthorized", 403)
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		log.Printf("Couldn't parse claims")
		respondWithMessage(w, "something went wrong", 500)
		return
	}

	id := claims["uuid"].(string)
	if _, err = uuid.Parse(id); err != nil {
		log.Printf("Couldn't parse uuid")
		respondWithMessage(w, "something went wrong", 500)
		return
	}

	exams, err := h.db.getStudentExams(id)
	if err != nil {
		log.Printf("Failed to get student exams \n%e", err)
		respondWithMessage(w, "something went wrong", 500)
		return
	}

	resp, err := json.Marshal(exams)
	if err != nil {
		fmt.Printf("Failed to marshall exams \n%e", err)
		respondWithMessage(w, "something went wrong", 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err = w.Write(resp); err != nil {
		fmt.Printf("Failed to write exams \n%e", err)
	}
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

func validateToken(reqHeader http.Header) (*jwt.Token, error) {

	if reqHeader["Authorization"] == nil {
		return nil, fmt.Errorf("can not find token in header")
	}

	token, err := jwt.Parse(reqHeader["Authorization"][0], func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("there was an error in parsing")
		}
		return os.Getenv("SECRET_KEY"), nil
	})

	if token.Valid {
		return token, nil
	} else if errors.Is(err, jwt.ErrTokenMalformed) {
		return nil, fmt.Errorf("that's not even a token")
	} else if errors.Is(err, jwt.ErrTokenExpired) || errors.Is(err, jwt.ErrTokenNotValidYet) { // Token is either expired or not active yet
		return nil, fmt.Errorf("token is either expired or not active yet")
	} else {
		return nil, fmt.Errorf("couldn't handle this token \n%e", err)
	}
}
