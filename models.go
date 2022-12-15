package main

type User struct {
	Email    string
	Password string
}

type Student struct {
	Id    string
	Name  string
	Phone string
	Email string
}

type Teacher struct {
	Id    string
	Name  string
	Phone string
	Email string
}

type Course struct {
	Id            string
	TeacherId     string
	Name          string
	NumberOfSeats int
}

type exam struct {
	id        string
	courseID  string
	studentID string
	points    int
}

type StudentExam struct {
	StudentName string
	CourseName  string
	Points      int
}

type InputExam struct {
	StudentEmail string
	CourseName   string
	Points       int
}
