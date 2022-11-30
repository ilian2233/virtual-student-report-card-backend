package main

import (
	"log"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func main() {
	// this Pings the database trying to connect
	// use sqlx.Open() for sql.Open() semantics
	_, err := sqlx.Connect("postgres", "user=test_user dbname=testdb sslmode=disable")
	if err != nil {
		log.Fatalln(err)
	}
	log.Println("DB connection successfully")
}
