package main

import (
	"log"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func main() {
	db, err := sqlx.Connect("postgres", "user=test_user dbname=testdb sslmode=disable")
	if err != nil {
		log.Fatalln(err)
	}
	log.Println("DB connection successfully")

	db.MustExec(schema)
	log.Println("DB schema created successfully")

	db.MustExec(addExampleData)
	log.Println("DB populated with example data")
}
