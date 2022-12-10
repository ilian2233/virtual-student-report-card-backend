package main

type person struct {
	uuid     string
	email    string
	password []byte
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

type curriculum struct {
	id             string
	name           string
	requiredPoints int
}
