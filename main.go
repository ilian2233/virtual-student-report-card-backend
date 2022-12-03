package main

import (
	"log"
	"net/http"

	_ "github.com/lib/pq"
)

func main() {
	db, err := createDatabaseConnection()
	if err != nil {
		log.Fatal(err)
	}

	h := handler{
		secretKet: "", //TODO: Load from env file
		db:        db,
	}

	//studentHandler := http.NewServeMux()
	//studentHandler.HandleFunc("/exam")
	//
	//teacherHandler := http.NewServeMux()
	//teacherHandler.HandleFunc("/exam")
	//teacherHandler.HandleFunc("/student")
	//
	//adminHandler := http.NewServeMux()
	//adminHandler.HandleFunc("/curriculum")
	//adminHandler.HandleFunc("/course")
	//adminHandler.HandleFunc("/exam")
	//adminHandler.HandleFunc("/student")
	//adminHandler.HandleFunc("/teacher")

	mainHandler := http.NewServeMux()
	mainHandler.HandleFunc("/login", h.handleLogin)
	//mainHandler.Handle("/student", studentHandler)
	//mainHandler.Handle("/teacher", teacherHandler)
	//mainHandler.Handle("/admin", adminHandler)
	if err = http.ListenAndServe(":8000", mainHandler); err != nil {
		panic(err)
	}

}
