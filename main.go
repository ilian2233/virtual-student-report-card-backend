package main

import (
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatalf("Error loading .env file \n%e", err)
	}

	db, err := createDatabaseConnection()
	if err != nil {
		log.Fatal(err)
	}

	h := handler{
		secretKet: os.Getenv("SECRET_KEY"),
		db:        db,
	}

	studentHandler := http.NewServeMux()
	studentHandler.HandleFunc("/exams", h.getStudentExams)

	teacherHandler := http.NewServeMux()
	teacherHandler.HandleFunc("/exams", h.postTeacherExams)

	adminHandler := http.NewServeMux()
	adminHandler.HandleFunc("/curriculums", h.curriculums)
	adminHandler.HandleFunc("/courses", h.courses)
	adminHandler.HandleFunc("/exams", h.getExams)
	adminHandler.HandleFunc("/students", h.students)
	adminHandler.HandleFunc("/teachers", h.teachers)

	mainHandler := http.NewServeMux()
	mainHandler.HandleFunc("/login", h.handleLogin)
	mainHandler.Handle("/student", studentHandler)
	mainHandler.Handle("/teacher", teacherHandler)
	mainHandler.Handle("/admin", adminHandler)
	if err = http.ListenAndServe(":8000", mainHandler); err != nil {
		panic(err)
	}

}
