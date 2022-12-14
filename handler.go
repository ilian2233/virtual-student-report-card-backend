package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

type handler struct {
	secretKet string
	db        interface {
		validateUserLogin(email string, password []byte) bool
		getUserRoles(email string) []string
		getStudentExams(email string) ([]StudentExam, error)
		insertExam(email string, e InputExam) error
		getTeacherCourseNames(email string) ([]string, error)
		getStudentEmails() ([]string, error)
		delete(table, uuid string) error
		getAllCourses() ([]Course, error)
		insertCourse(Course) error
		updateCourse(Course) error
		getAllExams() ([]exam, error)
		getAllStudents() ([]Student, error)
		insertStudent(Student) error
		updateStudent(Student) error
		getAllTeachers() ([]Teacher, error)
		insertTeacher(Teacher) error
		updateTeacher(Teacher) error
	}
}

func setupHandler(db dbConnection) *http.ServeMux {
	h := handler{
		secretKet: os.Getenv("SECRET_KEY"),
		db:        db,
	}

	mainHandler := http.NewServeMux()
	mainHandler.HandleFunc("/login", corsHandler(h.handleLogin))
	mainHandler.HandleFunc("/student/exams", corsHandler(h.getStudentExams))
	mainHandler.HandleFunc("/teacher/exams", corsHandler(h.postTeacherExams))
	mainHandler.HandleFunc("/teacher/courses", corsHandler(h.getTeacherCourses))
	mainHandler.HandleFunc("/teacher/students", corsHandler(h.getStudentEmails))
	mainHandler.HandleFunc("/admin/courses", corsHandler(h.courses))
	mainHandler.HandleFunc("/admin/exams", corsHandler(h.getExams))
	mainHandler.HandleFunc("/admin/students", corsHandler(h.students))
	mainHandler.HandleFunc("/admin/teachers", corsHandler(h.teachers))

	return mainHandler
}

func (h handler) handleLogin(w http.ResponseWriter, r *http.Request) {

	type User struct {
		Email    string
		Password string
	}

	if r.Method != http.MethodPost {
		respondWithMessage(w, "Only POST method is allowed", http.StatusBadRequest)
		return
	}

	var u User
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		respondWithMessage(w, "Invalid body", http.StatusBadRequest)
		return
	}

	if u.Email == "" || u.Password == "" {
		respondWithMessage(w, "Email or password is empty", http.StatusForbidden)
		return
	}

	if !h.db.validateUserLogin(u.Email, []byte(u.Password)) {
		respondWithMessage(w, "Incorrect email or password", http.StatusForbidden)
		return
	}

	token := jwt.New(jwt.SigningMethodHS256)
	claims := token.Claims.(jwt.MapClaims)

	claims["roles"] = h.db.getUserRoles(u.Email)
	claims["email"] = u.Email
	claims["exp"] = time.Now().Add(time.Minute * 43830).Unix()

	tokenString, err := token.SignedString([]byte(h.secretKet))
	if err != nil {
		log.Printf("Failed generating token, \n%e", err)
		respondWithMessage(w, "Something went wrong", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Authorization", tokenString)
	w.WriteHeader(200)
}

type Roles []interface{}

func (r Roles) contains(s string) bool {
	for _, v := range r {
		if v == s {
			return true
		}
	}
	return false
}

func (h handler) getStudentExams(w http.ResponseWriter, r *http.Request) {
	email, err := performChecks([]string{http.MethodGet}, "Student", r)

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

	exams, err := h.db.getStudentExams(email)
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
	email, err := performChecks([]string{http.MethodPost}, "Teacher", r)

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

	var e InputExam
	if err = json.Unmarshal(b, &e); err != nil {
		log.Printf("Couldn't unmarshall exams")
		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	if err = h.db.insertExam(email, e); err != nil {
		log.Printf("Exams insert failed with \n%e", err)
		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	respondWithMessage(w, "success", http.StatusOK)
}

func (h handler) getTeacherCourses(w http.ResponseWriter, r *http.Request) {
	email, err := performChecks([]string{http.MethodGet}, "Teacher", r)

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

	courses, err := h.db.getTeacherCourseNames(email)
	if err != nil {
		log.Printf("Failed to get student courses \n%e", err)
		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	resp, err := json.Marshal(courses)
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

func (h handler) getStudentEmails(w http.ResponseWriter, r *http.Request) {
	_, err := performChecks([]string{http.MethodGet}, "Teacher", r)

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

	courses, err := h.db.getStudentEmails()
	if err != nil {
		log.Printf("Failed to get student courses \n%e", err)
		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	resp, err := json.Marshal(courses)
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
		h.upsertCourses(w, r, true)
	case http.MethodPatch:
		h.upsertCourses(w, r, false)
	case http.MethodDelete:
		h.deleteCourse(w, r)
	default:
		respondWithMessage(w, "method not allowed", 400)
	}
}

func (h handler) getCourses(w http.ResponseWriter) {
	courses, err := h.db.getAllCourses()
	if err != nil {
		log.Printf("Failed to get courses \n%e", err)
		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	resp, err := json.Marshal(courses)
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

func (h handler) upsertCourses(w http.ResponseWriter, r *http.Request, insert bool) {
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

	var c Course
	if err = json.Unmarshal(b, &c); err != nil {
		log.Printf("Couldn't unmarshall courses")
		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	if insert {
		err = h.db.insertCourse(c)
	} else {
		err = h.db.updateCourse(c)
	}

	if err != nil {
		log.Printf("Course insert failed with \n%e", err)
		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	respondWithMessage(w, "success", http.StatusOK)
}

func (h handler) deleteCourse(w http.ResponseWriter, r *http.Request) {
	courseID := r.URL.Query().Get("id")

	if err := h.db.delete("course", courseID); err != nil {
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
		h.upsertStudents(w, r, true)
	case http.MethodPatch:
		h.upsertStudents(w, r, false)
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

func (h handler) upsertStudents(w http.ResponseWriter, r *http.Request, insert bool) {
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

	var s Student
	if err = json.Unmarshal(b, &s); err != nil {
		log.Printf("Couldn't unmarshall students")
		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	if insert {
		err = h.db.insertStudent(s)
	} else {
		err = h.db.updateStudent(s)
	}

	if err != nil {
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
		h.upsertTeachers(w, r, true)
	case http.MethodPatch:
		h.upsertTeachers(w, r, false)
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

func (h handler) upsertTeachers(w http.ResponseWriter, r *http.Request, insert bool) {
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

	var t Teacher
	if err = json.Unmarshal(b, &t); err != nil {
		log.Printf("Couldn't unmarshall teachers")
		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	if insert {
		err = h.db.insertTeacher(t)
	} else {
		err = h.db.updateTeacher(t)
	}

	if err != nil {
		log.Printf("Teacher insert failed with \n%e", err)
		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	respondWithMessage(w, "success", http.StatusOK)
}
