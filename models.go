package main

type Student struct {
	id       string
	name     string
	phone    string
	Email    string
	Password string
}

type Teacher struct {
	id       string
	name     string
	phone    string
	Email    string
	Password string
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

type StudentExam struct {
	Id          string
	StudentName string
	CourseName  string
	Points      int
}
