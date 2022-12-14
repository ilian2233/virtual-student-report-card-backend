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
		getStudentExams(email string) ([]Exam, error)
		insertExam(email string, e Exam) error
		getTeacherCourseNames(email string) ([]string, error)
		getStudentFacultyNumbers() ([]string, error)
		delete(table, uuid string) error
		getAllCourses() ([]Course, error)
		insertCourse(Course) error
		updateCourse(Course) error
		getAllStudents() ([]Student, error)
		insertStudent(Student) error
		updateStudent(Student) error
		getTeacherEmails() ([]string, error)
		insertTeacher(Teacher) error
		updateTeacher(Teacher) error
		getUsers(role string) (any, error)
		getTeacherExams() ([]Exam, error)
		archiveUser(email, role string) error
		resendPassword(email string) error
		changePassword(email, oldPassword, NewPassword string) error
		createPassword(code, password string) error
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
	mainHandler.HandleFunc("/teacher/exams", corsHandler(h.teacherExams))
	mainHandler.HandleFunc("/teacher/courses", corsHandler(h.getTeacherCourses))
	mainHandler.HandleFunc("/teacher/students", corsHandler(h.getStudentFacultyNumbers))
	mainHandler.HandleFunc("/admin/courses", corsHandler(h.courses))
	//mainHandler.HandleFunc("/admin/exams", corsHandler(h.getExams))
	mainHandler.HandleFunc("/admin/students", corsHandler(h.students))
	mainHandler.HandleFunc("/admin/teachers", corsHandler(h.teachers))
	mainHandler.HandleFunc("/admin/users", corsHandler(h.users))
	mainHandler.HandleFunc("/forgotten-password", corsHandler(h.forgottenPassword))
	mainHandler.HandleFunc("/change-password", corsHandler(h.changePassword))
	mainHandler.HandleFunc("/createPassword", corsHandler(h.createPassword))

	return mainHandler
}

func (h handler) handleLogin(w http.ResponseWriter, r *http.Request) {
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

	resp, err := json.Marshal(map[string]string{
		"Token": tokenString,
	})
	if err != nil {
		fmt.Printf("Failed to marshall response \n%e", err)
		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err = w.Write(resp); err != nil {
		fmt.Printf("Failed to write response \n%e", err)
	}
}

func (h handler) getStudentExams(w http.ResponseWriter, r *http.Request) {
	email, err := h.performChecks([]string{http.MethodGet}, "Student", r)

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

func (h handler) teacherExams(w http.ResponseWriter, r *http.Request) {
	email, err := h.performChecks([]string{http.MethodPost, http.MethodGet}, "Teacher", r)

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

	switch r.Method {
	case http.MethodGet:
		h.getExams(w)
	case http.MethodPost:
		h.insertExam(w, r, email)
	default:
		respondWithMessage(w, "method not allowed", 400)
	}
}

func (h handler) getTeacherCourses(w http.ResponseWriter, r *http.Request) {
	email, err := h.performChecks([]string{http.MethodGet}, "Teacher", r)

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

func (h handler) getStudentFacultyNumbers(w http.ResponseWriter, r *http.Request) {
	_, err := h.performChecks([]string{http.MethodGet}, "Teacher", r)

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

	courses, err := h.db.getStudentFacultyNumbers()
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
	_, err := h.performChecks([]string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodDelete}, "Admin", r)

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
		respondWithMessage(w, "something went wrong", http.StatusBadRequest)
		return
	}

	respondWithMessage(w, "success", http.StatusOK)
}

func (h handler) deleteCourse(w http.ResponseWriter, r *http.Request) {
	courseName := r.URL.Query().Get("CourseName")

	if err := h.db.delete("course", courseName); err != nil {
		log.Printf("Course delete failed with \n%e", err)
		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	respondWithMessage(w, "success", http.StatusOK)
}

//func (h handler) getExams(w http.ResponseWriter, r *http.Request) {
//	_, err := performChecks([]string{http.MethodGet, http.MethodPost}, "Admin", r)
//
//	switch true {
//	case errors.Is(err, errForbiddenMethod):
//		respondWithMessage(w, "Only GET and POST methods are allowed", http.StatusBadRequest)
//		return
//	case errors.Is(err, errValidatingJWT):
//		respondWithMessage(w, "unauthorized", http.StatusForbidden)
//		return
//	case errors.Is(err, errMissingRole):
//		log.Printf("Roles list doesn't contain admin")
//		respondWithMessage(w, "unauthorized", http.StatusForbidden)
//		return
//	case errors.Is(err, jwt.ErrTokenInvalidClaims):
//		log.Printf("Couldn't parse claims")
//		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
//		return
//	case errors.Is(err, jwt.ErrTokenInvalidId):
//		log.Printf("Couldn't parse uuid")
//		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
//		return
//	}
//
//	exams, err := h.db.getAllExams()
//	if err != nil {
//		log.Printf("Failed to get courses \n%e", err)
//		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
//		return
//	}
//
//	resp, err := json.Marshal(exams)
//	if err != nil {
//		fmt.Printf("Failed to marshall courses \n%e", err)
//		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
//		return
//	}
//
//	w.Header().Set("Content-Type", "application/json")
//	if _, err = w.Write(resp); err != nil {
//		fmt.Printf("Failed to write courses \n%e", err)
//	}
//}

func (h handler) students(w http.ResponseWriter, r *http.Request) {
	_, err := h.performChecks([]string{http.MethodGet, http.MethodPost, http.MethodPatch}, "Admin", r)

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
		respondWithMessage(w, "something went wrong", http.StatusBadRequest)
		return
	}

	respondWithMessage(w, "success", http.StatusOK)
}

func (h handler) teachers(w http.ResponseWriter, r *http.Request) {
	_, err := h.performChecks([]string{http.MethodGet, http.MethodPost, http.MethodPatch}, "Admin", r)

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
	teacherEmails, err := h.db.getTeacherEmails()
	if err != nil {
		log.Printf("Failed to get teacherEmails \n%e", err)
		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	resp, err := json.Marshal(teacherEmails)
	if err != nil {
		fmt.Printf("Failed to marshall teacherEmails \n%e", err)
		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err = w.Write(resp); err != nil {
		fmt.Printf("Failed to write teacherEmails \n%e", err)
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
		respondWithMessage(w, "something went wrong", http.StatusBadRequest)
		return
	}

	respondWithMessage(w, "success", http.StatusOK)
}

func (h handler) getUserData(w http.ResponseWriter, r *http.Request) {
	role := r.URL.Query().Get("role")

	users, err := h.db.getUsers(role)
	if err != nil {
		log.Printf("Failed to get users \n%e", err)
		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	resp, err := json.Marshal(users)
	if err != nil {
		fmt.Printf("Failed to marshall users \n%e", err)
		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err = w.Write(resp); err != nil {
		fmt.Printf("Failed to write users \n%e", err)
	}
}

func (h handler) insertExam(w http.ResponseWriter, r *http.Request, email string) {
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

	var e Exam
	if err = json.Unmarshal(b, &e); err != nil {
		log.Printf("Couldn't unmarshall exams")
		respondWithMessage(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	if err = h.db.insertExam(email, e); err != nil {
		log.Printf("Exams insert failed with \n%e", err)
		respondWithMessage(w, "something went wrong", http.StatusBadRequest)
		return
	}

	respondWithMessage(w, "success", http.StatusOK)
}

func (h handler) getExams(w http.ResponseWriter) {
	exams, err := h.db.getTeacherExams()
	if err != nil {
		log.Printf("Failed to get exams \n%e", err)
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

func (h handler) users(w http.ResponseWriter, r *http.Request) {
	_, err := h.performChecks([]string{http.MethodGet, http.MethodDelete}, "Admin", r)

	switch true {
	case errors.Is(err, errForbiddenMethod):
		respondWithMessage(w, "Only GET and DELETE methods are allowed", http.StatusBadRequest)
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
		h.getUserData(w, r)
	case http.MethodDelete:
		h.archiveUser(w, r)
	default:
		respondWithMessage(w, "method not allowed", 400)
	}
}

func (h handler) archiveUser(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	role := r.URL.Query().Get("role")

	if email == "" {
		log.Printf("email must not be empty")
		respondWithMessage(w, "content must be provided in request body", http.StatusBadRequest)
		return
	}

	if err := h.db.archiveUser(email, role); err != nil {
		log.Printf("Exams insert failed with \n%e", err)
		respondWithMessage(w, "something went wrong", http.StatusBadRequest)
		return
	}

	respondWithMessage(w, "success", http.StatusOK)
}

func (h handler) forgottenPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondWithMessage(w, "Only POST method is allowed", http.StatusBadRequest)
		return
	}

	email := r.URL.Query().Get("email")

	if err := h.db.resendPassword(email); err != nil {
		log.Printf("Failed to resend password with \n%e", err)
		respondWithMessage(w, "something went wrong", http.StatusBadRequest)
		return
	}

	respondWithMessage(w, "success", http.StatusOK)
}

func (h handler) changePassword(w http.ResponseWriter, r *http.Request) {
	email, err := h.performChecksWithoutRoles([]string{http.MethodPost}, r)

	switch true {
	case errors.Is(err, errForbiddenMethod):
		respondWithMessage(w, "Only POST methods are allowed", http.StatusBadRequest)
		return
	case errors.Is(err, errValidatingJWT):
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

	var passwords struct {
		OldPassword string
		NewPassword string
	}
	byteValue, _ := io.ReadAll(r.Body)
	if err = json.Unmarshal(byteValue, &passwords); err != nil {
		log.Printf("Failed to unmarshal password with \n%e", err)
		respondWithMessage(w, "something went wrong", http.StatusBadRequest)
		return
	}

	if err = h.db.changePassword(email, passwords.OldPassword, passwords.NewPassword); err != nil {
		log.Printf("Failed to change password with \n%e", err)
		respondWithMessage(w, "something went wrong", http.StatusBadRequest)
		return
	}

	respondWithMessage(w, "success", http.StatusOK)
}

func (h handler) createPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondWithMessage(w, "Only POST method is allowed", http.StatusBadRequest)
		return
	}

	var Body struct {
		Code     string
		Password string
	}
	byteValue, _ := io.ReadAll(r.Body)
	if err := json.Unmarshal(byteValue, &Body); err != nil {
		log.Printf("Failed to unmarshal password with \n%e", err)
		respondWithMessage(w, "something went wrong", http.StatusBadRequest)
		return
	}

	if err := h.db.createPassword(Body.Code, Body.Password); err != nil {
		log.Printf("Failed to change password with \n%e", err)
		respondWithMessage(w, "something went wrong", http.StatusBadRequest)
		return
	}

	respondWithMessage(w, "success", http.StatusOK)
}
