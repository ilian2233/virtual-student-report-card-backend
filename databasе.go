package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"
)

//TODO: Update teacher and student insert and update strategies

const (
	dropTables = `
DROP TABLE IF EXISTS exam;
DROP TABLE IF EXISTS course;
DROP TABLE IF EXISTS admin;
DROP TABLE IF EXISTS student;
DROP TABLE IF EXISTS teacher;
DROP TABLE IF EXISTS person;`

	schema = `
CREATE TABLE IF NOT EXISTS person (
    email TEXT NOT NULL PRIMARY KEY UNIQUE CHECK (email ~ '^[A-Za-z0-9._%-]+@[A-Za-z0-9.-]+[.][A-Za-z]+$'),
    name TEXT NOT NULL CHECK (name <> ''),
    phone TEXT,
    password TEXT NOT NULL CHECK (password <> '')
);

CREATE TABLE IF NOT EXISTS admin (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    person_id TEXT REFERENCES person(email) NOT NULL,
    active BOOLEAN DEFAULT TRUE
);

CREATE TABLE IF NOT EXISTS student (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    person_id TEXT REFERENCES person(email) NOT NULL
);

CREATE TABLE IF NOT EXISTS teacher (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    person_id TEXT REFERENCES person(email) NOT NULL,
    active BOOLEAN DEFAULT TRUE
);

-- Given subject of study e.g. math
CREATE TABLE IF NOT EXISTS course (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    teacher_id UUID REFERENCES teacher(id) NOT NULL,
    name TEXT NOT NULL CHECK (name <> ''),
    number_of_seats INT DEFAULT 50 CHECK (number_of_seats > 0),
    deleted BOOL DEFAULT FALSE,
    UNIQUE(name, teacher_id)
);

CREATE TABLE IF NOT EXISTS exam (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    course_id UUID REFERENCES course(id) NOT NULL,
    student_id UUID REFERENCES student(id) NOT NULL,
    points INT CHECK (points > 0),
  	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted BOOL DEFAULT FALSE
);`

	addExampleData = `
INSERT INTO person(name, phone, email, password) VALUES 
    ('ivan', '0881234563', 'test@test.com', '$2a$10$hk6NfxXSkNUzEd7fiqGyxOGUjimPR/jtdmRf0yrbB/eKfh5HwKe4q'),
    ('ivan1', '0881234564', 'test1@test.com', '$2a$10$hk6NfxXSkNUzEd7fiqGyxOGUjimPR/jtdmRf0yrbB/eKfh5HwKe4q'),
    ('ivan2', '0881234565', 'test2@test.com', '$2a$10$hk6NfxXSkNUzEd7fiqGyxOGUjimPR/jtdmRf0yrbB/eKfh5HwKe4q');

INSERT INTO admin(person_id) VALUES 
    ((SELECT email FROM person WHERE name='ivan'));

INSERT INTO student(person_id) VALUES 
    ((SELECT email FROM person WHERE name='ivan1'));

INSERT INTO teacher(person_id) VALUES 
    ((SELECT email FROM person WHERE name='ivan2'));

INSERT INTO course(teacher_id, name) VALUES 
    ((SELECT id FROM teacher LIMIT 1), 'Math'), 
    ((SELECT id FROM teacher LIMIT 1), 'Programming Basics');

INSERT INTO exam(course_id, student_id, points) VALUES 
    ((SELECT id FROM course WHERE name='Math' LIMIT 1),(SELECT id FROM student LIMIT 1), 56);
`
)

type dbConnection struct {
	db *sqlx.DB
}

func createDatabaseConnection() (dbConnection, error) {
	connString := fmt.Sprintf("user=%s dbname=%s sslmode=disable", os.Getenv("DB_USER"), os.Getenv("DB_NAME"))
	db, err := sqlx.Connect("postgres", connString)
	if err != nil {
		return dbConnection{}, err
	}
	log.Println("DB connection successfully")

	db.MustExec(dropTables)
	log.Println("DB drop old tables")

	db.MustExec(schema)
	log.Println("DB schema created successfully")

	db.MustExec(addExampleData)
	log.Println("DB populated with example data")

	return dbConnection{
		db: db,
	}, nil
}

func (conn dbConnection) validateUserLogin(email string, password []byte) bool {
	var u User
	if err := conn.db.Get(&u, "SELECT email, password FROM person WHERE email=$1", email); err != nil {
		log.Printf("Failed to query db,\n %e", err)
		return false
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), password); err != nil {
		log.Printf("Password did not match \n%e", err)
		return false
	}

	return true
}

func (conn dbConnection) getUserRoles(uuid string) (roles []string) {
	dest := ""
	if err := conn.db.Get(&dest, "SELECT id FROM admin WHERE person_id=$1", uuid); err == nil {
		roles = append(roles, "Admin")
	}

	if err := conn.db.Get(&dest, "SELECT id FROM student WHERE person_id=$1", uuid); err == nil {
		roles = append(roles, "Student")
	}

	if err := conn.db.Get(&dest, "SELECT id FROM teacher WHERE person_id=$1", uuid); err == nil {
		roles = append(roles, "Teacher")
	}

	return roles
}

func (conn dbConnection) getStudentExams(studentEmail string) (exams []Exam, err error) {
	id, err := conn.getStudentIdFromEmail(studentEmail)
	if err != nil {
		return exams, err
	}

	if err = conn.db.Select(&exams, "SELECT p.name as StudentName, c.name as CourseName, points FROM exam e JOIN course c ON c.id = e.course_id JOIN student s ON s.id = e.student_id JOIN person p ON p.email = s.person_id WHERE student_id=$1 AND c.deleted=FALSE", id); err != nil {
		return exams, err
	}
	return exams, nil
}

func (conn dbConnection) getStudentIdFromEmail(email string) (id string, err error) {
	if err = conn.db.Get(&id, "SELECT id FROM student WHERE person_id=$1", email); err != nil {
		return "", err
	}
	return id, nil
}

func (conn dbConnection) insertExam(teacherEmail string, e Exam) error {
	courses, err := conn.getTeacherCourses(teacherEmail)
	if err != nil {
		return err
	}

	if !courseList(courses).contains(e.CourseName) {
		return fmt.Errorf("course not led by that teacher")
	}

	teacherId, err := conn.getTeacherIdFromEmail(teacherEmail)
	if err != nil {
		return err
	}

	var courseID string
	if err = conn.db.Get(&courseID, "SELECT id FROM course WHERE name = $1 AND teacher_id = $2", e.CourseName, teacherId); err != nil {
		return err
	}

	var studentID string
	if err = conn.db.Get(&studentID, "SELECT id FROM student WHERE person_id = $1", e.StudentEmail); err != nil {
		return err
	}

	if _, err = conn.db.Exec("INSERT INTO exam(course_id, student_id, points) VALUES ($1, $2, $3)", courseID, studentID, e.Points); err != nil {
		return err
	}
	return nil
}

type courseList []Course

func (c courseList) contains(name string) bool {
	for _, v := range c {
		if v.Name == name {
			return true
		}
	}
	return false
}

func (conn dbConnection) getTeacherCourses(email string) ([]Course, error) {
	id, err := conn.getTeacherIdFromEmail(email)
	if err != nil {
		return nil, err
	}

	var courses []Course
	if err = conn.db.Select(&courses, "SELECT id, teacher_id as TeacherId, name, number_of_seats as NumberOfSeats FROM course WHERE teacher_id = $1 AND deleted=FALSE", id); err != nil {
		log.Printf("Failed to get teacher courses")
		return nil, err
	}

	return courses, nil
}

func (conn dbConnection) getTeacherCourseNames(email string) ([]string, error) {
	courses, err := conn.getTeacherCourses(email)
	if err != nil {
		return nil, err
	}

	var result []string
	for _, v := range courses {
		result = append(result, v.Name)
	}

	return result, nil
}

func (conn dbConnection) getStudentEmails() ([]string, error) {
	students, err := conn.getAllStudents()
	if err != nil {
		return nil, err
	}

	var result []string
	for _, v := range students {
		result = append(result, v.Email)
	}

	return result, nil
}

func (conn dbConnection) getTeacherIdFromEmail(email string) (id string, err error) {
	if err = conn.db.Get(&id, "SELECT id FROM teacher WHERE person_id=$1", email); err != nil {
		return "", err
	}
	return id, nil
}

func (conn dbConnection) getAllCourses() (courses []Course, err error) {
	if err = conn.db.Select(&courses, "SELECT id, teacher_id as TeacherId, name, number_of_seats as NumberOfSeats FROM course WHERE deleted=FALSE"); err != nil {
		log.Printf("Failed to get courses")
		return nil, err
	}
	return courses, nil
}

func (conn dbConnection) insertCourse(c Course) error {
	if _, err := conn.db.Exec("INSERT INTO course(teacher_id, name, number_of_seats) VALUES ($1, $2, $3)", c.TeacherId, c.Name, c.NumberOfSeats); err != nil {
		return err
	}
	return nil
}

func (conn dbConnection) updateCourse(c Course) error {
	if _, err := conn.db.Exec("UPDATE course SET teacher_id=$1, name=$2, number_of_seats=$3 WHERE id=$4", c.TeacherId, c.Name, c.NumberOfSeats, c.Id); err != nil {
		return err
	}
	return nil
}

func (conn dbConnection) getAllExams() (exams []exam, err error) {
	if err = conn.db.Select(&exams, "SELECT id, course_id as courseID, student_id as studentID, points FROM exam WHERE deleted=FALSE"); err != nil {
		log.Printf("Failed to get exams")
		return nil, err
	}
	return exams, nil
}

func (conn dbConnection) getAllStudents() (students []Student, err error) {
	if err = conn.db.Select(&students, "SELECT id as Id, name as Name, phone as Phone, email as Email FROM student JOIN person p on p.email = student.person_id"); err != nil {
		log.Printf("Failed to get students")
		return nil, err
	}
	return students, nil
}

func (conn dbConnection) insertStudent(s Student) error {
	tx, err := conn.db.Begin()

	defer func(tx *sql.Tx) {
		_ = tx.Rollback()
	}(tx)

	if err != nil {
		return err
	}

	if err = insertPerson(tx, s); err != nil {
		return err
	}

	if _, err = tx.Exec("INSERT INTO student(person_id) VALUES ($1)", s.Email); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (conn dbConnection) updateStudent(s Student) error {
	tx, err := conn.db.Begin()

	defer func(tx *sql.Tx) {
		_ = tx.Rollback()
	}(tx)

	if err != nil {
		return err
	}

	if err = insertPerson(tx, s); err != nil {
		return err
	}

	if _, err = tx.Exec("UPDATE student SET person_id=$1 WHERE id=$2", s.Email, s.Id); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (conn dbConnection) getAllTeachers() (teachers []Teacher, err error) {
	if err = conn.db.Select(&teachers, "SELECT id as Id, name as Name, phone as Phone, email as Email FROM teacher JOIN person p on p.email = teacher.person_id"); err != nil {
		log.Printf("Failed to get teachers")
		return nil, err
	}
	return teachers, nil
}

func (conn dbConnection) getTeacherEmails() ([]string, error) {
	teachers, err := conn.getAllTeachers()
	if err != nil {
		return nil, err
	}

	var result []string
	for _, v := range teachers {
		result = append(result, v.Email)
	}

	return result, nil
}

func (conn dbConnection) insertTeacher(t Teacher) error {
	tx, err := conn.db.Begin()

	defer func(tx *sql.Tx) {
		_ = tx.Rollback()
	}(tx)

	if err != nil {
		return err
	}

	if err = insertPerson(tx, Student(t)); err != nil {
		return err
	}

	if _, err = tx.Exec("INSERT INTO teacher(person_id) VALUES ($1)", t.Email); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (conn dbConnection) updateTeacher(t Teacher) error {
	tx, err := conn.db.Begin()

	defer func(tx *sql.Tx) {
		_ = tx.Rollback()
	}(tx)

	if err != nil {
		return err
	}

	if err = insertPerson(tx, Student(t)); err != nil {
		return err
	}

	if _, err = tx.Exec("UPDATE teacher SET person_id=$1 WHERE id=$2", t.Email, t.Id); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (conn dbConnection) delete(table, uuid string) error {
	if _, err := conn.db.Exec("UPDATE $1 SET deleted=TRUE WHERE id=$2", table, uuid); err != nil {
		log.Printf("Failed to delete %s with id %s", table, uuid)
		return err
	}
	return nil
}

func insertPerson(tx *sql.Tx, s Student) error {

	defaultPass := []byte(os.Getenv("DEFAULT_PASS"))

	hashedPassword, err := bcrypt.GenerateFromPassword(defaultPass, bcrypt.DefaultCost)
	if err != nil {
		log.Println(err)
	}

	if _, err = tx.Exec("INSERT INTO person(name, email, phone, password) VALUES ($1, $2, $3, $4)", s.Name, s.Email, s.Phone, hashedPassword); err != nil {
		return err
	}
	return nil
}
