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

type handler struct {
	secretKet string
	db        interface {
		validateUserLogin(email string, password []byte) bool
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

	var u person
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		respondWithMessage(w, "Invalid body", http.StatusBadRequest)
		return
	}

	if u.email == "" || string(u.password) == "" {
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
	id, err := performChecks([]string{http.MethodGet}, "Student", r)

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
	id, err := performChecks([]string{http.MethodPost}, "Teacher", r)

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

func (h handler) curriculums(w http.ResponseWriter, r *http.Request) {
	_, err := performChecks([]string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodDelete}, "Admin", r)

	switch true {
	case errors.Is(err, errForbiddenMethod):
		respondWithMessage(w, "Only GET,POST,PATCH and DELETE methods are allowed", http.StatusBadRequest)
		return
	case errors.Is(err, errValidatingJWT):
		respondWithMessage(w, "unauthorized", http.StatusForbidden)
		return
	case errors.Is(err, errMissingRole):
		log.Printf("Roles list doesn't contain admin")
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

	switch r.Method {
	case http.MethodGet:
		h.getCurriculums(w)
	case http.MethodPost:
	case http.MethodPatch:
		h.upsertCurriculums(w, r)
	case http.MethodDelete:
		h.deleteCurriculum(w, r)
	default:
		respondWithMessage(w, "method not allowed", 400)
	}
}

func (h handler) getCurriculums(w http.ResponseWriter) {
	curriculums, err := h.db.getAllCurriculums()
	if err != nil {
		log.Printf("Failed to get student exams \n%e", err)
		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	resp, err := json.Marshal(curriculums)
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

func (h handler) upsertCurriculums(w http.ResponseWriter, r *http.Request) {
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

	var c []curriculum
	if err = json.Unmarshal(b, &c); err != nil {
		log.Printf("Couldn't unmarshall curriculums")
		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	if err = h.db.updateCurriculums(c); err != nil {
		log.Printf("Curriculum insert failed with \n%e", err)
		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	respondWithMessage(w, "success", http.StatusOK)
}

func (h handler) deleteCurriculum(w http.ResponseWriter, r *http.Request) {
	curriculumID := r.URL.Query().Get("id")

	if err = h.db.deleteCurriculum(curriculumID); err != nil {
		log.Printf("Curriculum delete failed with \n%e", err)
		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	respondWithMessage(w, "success", http.StatusOK)
}

func (h handler) courses(w http.ResponseWriter, r *http.Request) {
	_, err := performChecks([]string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodDelete}, "Admin", r)

	switch true {
	case errors.Is(err, errForbiddenMethod):
		respondWithMessage(w, "Only GET,POST,PATCH and DELETE methods are allowed", http.StatusBadRequest)
		return
	case errors.Is(err, errValidatingJWT):
		respondWithMessage(w, "unauthorized", http.StatusForbidden)
		return
	case errors.Is(err, errMissingRole):
		log.Printf("Roles list doesn't contain admin")
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

	switch r.Method {
	case http.MethodGet:
		h.getCourses(w)
	case http.MethodPost:
	case http.MethodPatch:
		h.upsertCourses(w, r)
	case http.MethodDelete:
		h.deleteCourse(w, r)
	default:
		respondWithMessage(w, "method not allowed", 400)
	}
}

func (h handler) getCourses(w http.ResponseWriter) {
	curriculums, err := h.db.getAllCourses()
	if err != nil {
		log.Printf("Failed to get courses \n%e", err)
		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	resp, err := json.Marshal(curriculums)
	if err != nil {
		fmt.Printf("Failed to marshall courses \n%e", err)
		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err = w.Write(resp); err != nil {
		fmt.Printf("Failed to write courses \n%e", err)
	}
}

func (h handler) upsertCourses(w http.ResponseWriter, r *http.Request) {
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

	var c []course
	if err = json.Unmarshal(b, &c); err != nil {
		log.Printf("Couldn't unmarshall courses")
		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	if err = h.db.upsertCourses(c); err != nil {
		log.Printf("Course insert failed with \n%e", err)
		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	respondWithMessage(w, "success", http.StatusOK)
}

func (h handler) deleteCourse(w http.ResponseWriter, r *http.Request) {
	courseID := r.URL.Query().Get("id")

	if err = h.db.deleteCourses(courseID); err != nil {
		log.Printf("Course delete failed with \n%e", err)
		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	respondWithMessage(w, "success", http.StatusOK)
}

func (h handler) getExams(w http.ResponseWriter, r *http.Request) {
	_, err := performChecks([]string{http.MethodGet, http.MethodPost}, "Admin", r)

	switch true {
	case errors.Is(err, errForbiddenMethod):
		respondWithMessage(w, "Only GET and POST methods are allowed", http.StatusBadRequest)
		return
	case errors.Is(err, errValidatingJWT):
		respondWithMessage(w, "unauthorized", http.StatusForbidden)
		return
	case errors.Is(err, errMissingRole):
		log.Printf("Roles list doesn't contain admin")
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

	exams, err := h.db.getAllExams()
	if err != nil {
		log.Printf("Failed to get courses \n%e", err)
		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	resp, err := json.Marshal(exams)
	if err != nil {
		fmt.Printf("Failed to marshall courses \n%e", err)
		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err = w.Write(resp); err != nil {
		fmt.Printf("Failed to write courses \n%e", err)
	}
}

func (h handler) students(w http.ResponseWriter, r *http.Request) {
	_, err := performChecks([]string{http.MethodGet, http.MethodPost, http.MethodPatch}, "Admin", r)

	switch true {
	case errors.Is(err, errForbiddenMethod):
		respondWithMessage(w, "Only GET,POST and PATCH methods are allowed", http.StatusBadRequest)
		return
	case errors.Is(err, errValidatingJWT):
		respondWithMessage(w, "unauthorized", http.StatusForbidden)
		return
	case errors.Is(err, errMissingRole):
		log.Printf("Roles list doesn't contain admin")
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

	switch r.Method {
	case http.MethodGet:
		h.getStudents(w)
	case http.MethodPost:
	case http.MethodPatch:
		h.upsertStudents(w, r)
	default:
		respondWithMessage(w, "method not allowed", 400)
	}
}

func (h handler) getStudents(w http.ResponseWriter) {
	students, err := h.db.getAllStudents()
	if err != nil {
		log.Printf("Failed to get students \n%e", err)
		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	resp, err := json.Marshal(students)
	if err != nil {
		fmt.Printf("Failed to marshall students \n%e", err)
		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err = w.Write(resp); err != nil {
		fmt.Printf("Failed to write students \n%e", err)
	}
}

func (h handler) upsertStudents(w http.ResponseWriter, r *http.Request) {
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

	var students []student
	if err = json.Unmarshal(b, &students); err != nil {
		log.Printf("Couldn't unmarshall students")
		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	if err = h.db.upsertStudents(students); err != nil {
		log.Printf("Student insert failed with \n%e", err)
		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	respondWithMessage(w, "success", http.StatusOK)
}

func (h handler) teachers(w http.ResponseWriter, r *http.Request) {
	_, err := performChecks([]string{http.MethodGet, http.MethodPost, http.MethodPatch}, "Admin", r)

	switch true {
	case errors.Is(err, errForbiddenMethod):
		respondWithMessage(w, "Only GET,POST and PATCH methods are allowed", http.StatusBadRequest)
		return
	case errors.Is(err, errValidatingJWT):
		respondWithMessage(w, "unauthorized", http.StatusForbidden)
		return
	case errors.Is(err, errMissingRole):
		log.Printf("Roles list doesn't contain admin")
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

	switch r.Method {
	case http.MethodGet:
		h.getTeachers(w)
	case http.MethodPost:
	case http.MethodPatch:
		h.upsertTeachers(w, r)
	default:
		respondWithMessage(w, "method not allowed", 400)
	}
}

func (h handler) getTeachers(w http.ResponseWriter) {
	teachers, err := h.db.getAllTeachers()
	if err != nil {
		log.Printf("Failed to get teachers \n%e", err)
		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	resp, err := json.Marshal(teachers)
	if err != nil {
		fmt.Printf("Failed to marshall teachers \n%e", err)
		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err = w.Write(resp); err != nil {
		fmt.Printf("Failed to write teachers \n%e", err)
	}
}

func (h handler) upsertTeachers(w http.ResponseWriter, r *http.Request) {
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

	var teachers []teacher
	if err = json.Unmarshal(b, &teachers); err != nil {
		log.Printf("Couldn't unmarshall teachers")
		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	if err = h.db.upsertTeachers(teachers); err != nil {
		log.Printf("Teacher insert failed with \n%e", err)
		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	respondWithMessage(w, "success", http.StatusOK)
}
