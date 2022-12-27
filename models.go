package main

type User struct {
	Email    string
	Password string
}

type person struct {
	Name  string
	Email string
	Phone string
}

type Student struct {
	FacultyNumber string
	Name          string
	Phone         string
	Email         string
}

type Teacher struct {
	Name  string
	Phone string
	Email string
}

type Course struct {
	Id            string
	TeacherId     string
	TeacherName   string
	Name          string
	NumberOfSeats int
}

type exam struct {
	id        string
	courseID  string
	studentID string
	points    int
}

type Exam struct {
	StudentName          string
	StudentFacultyNumber string
	CourseName           string
	Points               int
}
