package main

type student struct {
	id       string
	name     string
	phone    string
	email    string
	password []byte
}

type teacher struct {
	id       string
	name     string
	phone    string
	email    string
	password []byte
}

type curriculum struct {
	id             string
	name           string
	requiredPoints int
}

type course struct {
	id            string
	teacherId     string
	curriculumId  string
	name          string
	numberOfSeats int
}

type exam struct {
	id        string
	courseID  string
	studentID string
	points    int
}

type studentExam struct {
	id         string
	courseName string
	points     int
}
