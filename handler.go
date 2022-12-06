package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

type user struct {
	email    string
	password string
}

type studentExam struct {
	courseName string
	points     int
}

type teacherExam struct {
	courseID  string
	studentID string
	points    int
}

type handler struct {
	secretKet string
	db        interface {
		validateUserLogin(email, password string) bool
		getUserUUIDByEmail(email string) (string, error)
		getUserRoles(uuid string) []string
		getStudentExams(uuid string) ([]studentExam, error)
		insertExams(teacherUUID string, exams []teacherExam) (teacherExam, error)
	}
}

func (h handler) handleLogin(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		respondWithMessage(w, "Only POST method is allowed", http.StatusBadRequest)
		return
	}

	var u user
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		respondWithMessage(w, "Invalid body", http.StatusBadRequest)
		return
	}

	if u.email == "" || u.password == "" {
		respondWithMessage(w, "Email or password is empty", http.StatusForbidden)
		return
	}

	if !h.db.validateUserLogin(u.email, u.password) {
		respondWithMessage(w, "Incorrect email or password", http.StatusForbidden)
		return
	}

	id, err := h.db.getUserUUIDByEmail(u.email)
	if err != nil {
		log.Printf("Failed extracting uuid by email, \n%e", err)
		respondWithMessage(w, "Something went wrong", http.StatusInternalServerError)
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
		respondWithMessage(w, "Something went wrong", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Authorization", tokenString)
	w.WriteHeader(200)
}

type roles []string

func (r roles) contains(s string) bool {
	for _, v := range r {
		if v == s {
			return true
		}
	}
	return false
}

func (h handler) getStudentExams(w http.ResponseWriter, r *http.Request) {

	id, err := performChecks(http.MethodGet, "Student", r)

	switch true {
	case errors.Is(err, errForbiddenMethod):
		respondWithMessage(w, "Only GET method is allowed", http.StatusBadRequest)
		return
	case errors.Is(err, errValidatingJWT):
		respondWithMessage(w, "unauthorized", http.StatusForbidden)
		return
	case errors.Is(err, errMissingRole):
		log.Printf("Roles list doesn't contain student")
		respondWithMessage(w, "unauthorized", http.StatusForbidden)
		return
	case errors.Is(err, jwt.ErrTokenInvalidClaims):
		log.Printf("Couldn't parse claims")
		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
		return
	case errors.Is(err, jwt.ErrTokenInvalidId):
		log.Printf("Couldn't parse uuid")
		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	exams, err := h.db.getStudentExams(id)
	if err != nil {
		log.Printf("Failed to get student exams \n%e", err)
		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	resp, err := json.Marshal(exams)
	if err != nil {
		fmt.Printf("Failed to marshall exams \n%e", err)
		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err = w.Write(resp); err != nil {
		fmt.Printf("Failed to write exams \n%e", err)
	}
}

func (h handler) postTeacherExams(w http.ResponseWriter, r *http.Request) {
	id, err := performChecks(http.MethodPost, "Teacher", r)

	switch true {
	case errors.Is(err, errForbiddenMethod):
		respondWithMessage(w, "Only POST method is allowed", http.StatusBadRequest)
		return
	case errors.Is(err, errValidatingJWT):
		respondWithMessage(w, "unauthorized", http.StatusForbidden)
		return
	case errors.Is(err, errMissingRole):
		log.Printf("Roles list doesn't contain teacher")
		respondWithMessage(w, "unauthorized", http.StatusForbidden)
		return
	case errors.Is(err, jwt.ErrTokenInvalidClaims):
		log.Printf("Couldn't parse claims")
		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
		return
	case errors.Is(err, jwt.ErrTokenInvalidId):
		log.Printf("Couldn't parse uuid")
		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	b, err := io.ReadAll(r.Body)
	if err != nil {
		msg := "failed to read request body"
		log.Printf(msg)
		respondWithMessage(w, msg, http.StatusBadRequest)
		return
	}

	if len(b) == 0 {
		log.Printf("request body must not be empty")
		respondWithMessage(w, "content must be provided in request body", http.StatusBadRequest)
		return
	}

	var exams []teacherExam
	if err = json.Unmarshal(b, &exams); err != nil {
		log.Printf("Couldn't unmarshall exams")
		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	failedExam, err := h.db.insertExams(id, exams)
	var emptyExam teacherExam
	if failedExam != emptyExam {
		resp, err := json.Marshal(failedExam)
		if err != nil {
			fmt.Printf("Failed to marshall exam \n%e", err)
			respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		if _, err = w.Write(resp); err != nil {
			fmt.Printf("Failed to write exams \n%e", err)
		}
	} else if err != nil {
		log.Printf("Exams insert failed with \n%e", err)
		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	respondWithMessage(w, "success", http.StatusOK)
}
