package main

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"net/smtp"
	"os"

	"github.com/dchest/uniuri"
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
    person_id TEXT UNIQUE REFERENCES person(email) NOT NULL,
    active BOOLEAN DEFAULT TRUE
);

CREATE TABLE IF NOT EXISTS student (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	faculty_number TEXT UNIQUE NOT NULL CHECK ( faculty_number ~ '^\d{8}$'),
    person_id TEXT UNIQUE REFERENCES person(email) NOT NULL,
    active BOOLEAN DEFAULT TRUE
);

CREATE TABLE IF NOT EXISTS teacher (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    person_id TEXT UNIQUE REFERENCES person(email) NOT NULL,
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
    student_faculty_number TEXT REFERENCES student(faculty_number) NOT NULL,
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

INSERT INTO student(faculty_number, person_id) VALUES 
    ('12312312',(SELECT email FROM person WHERE name='ivan1'));

INSERT INTO teacher(person_id) VALUES 
    ((SELECT email FROM person WHERE name='ivan2'));

INSERT INTO course(teacher_id, name) VALUES 
    ((SELECT id FROM teacher LIMIT 1), 'Math'), 
    ((SELECT id FROM teacher LIMIT 1), 'Programming Basics');

INSERT INTO exam(course_id, student_faculty_number, points) VALUES 
    ((SELECT id FROM course WHERE name='Math' LIMIT 1),(SELECT faculty_number FROM student LIMIT 1), 56);
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
	var studentFacultyNumber string
	if err = conn.db.Get(&studentFacultyNumber, "SELECT faculty_number FROM student WHERE person_id=$1", studentEmail); err != nil {
		return exams, err
	}

	if err = conn.db.Select(&exams, "SELECT p.name as StudentName, c.name as CourseName, points FROM exam e JOIN course c ON c.id = e.course_id JOIN student s ON s.faculty_number = e.student_faculty_number JOIN person p ON p.email = s.person_id WHERE faculty_number=$1 AND c.deleted=FALSE", studentFacultyNumber); err != nil {
		return exams, err
	}
	return exams, nil
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

	if _, err = conn.db.Exec("INSERT INTO exam(course_id, student_faculty_number, points) VALUES ($1, $2, $3)", courseID, e.StudentFacultyNumber, e.Points); err != nil {
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

func (conn dbConnection) getStudentFacultyNumbers() ([]string, error) {
	students, err := conn.getAllStudents()
	if err != nil {
		return nil, err
	}

	var result []string
	for _, v := range students {
		result = append(result, v.FacultyNumber)
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
	if err = conn.db.Select(&courses, "SELECT c.id, teacher_id as TeacherId, c.name, number_of_seats as NumberOfSeats, p.name as TeacherName FROM course c JOIN teacher t on t.id = c.teacher_id JOIN person p on p.email = t.person_id WHERE deleted=FALSE"); err != nil {
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

func (conn dbConnection) getAllStudents() (students []Student, err error) {
	if err = conn.db.Select(&students, "SELECT name as Name, phone as Phone, email as Email, faculty_number as FacultyNumber FROM student JOIN person p on p.email = student.person_id WHERE student.active=TRUE"); err != nil {
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

	if err = insertPerson(tx, person{s.Name, s.Email, s.Phone}); err != nil {
		return err
	}

	if _, err = tx.Exec("INSERT INTO student(faculty_number, person_id) VALUES ($1,$2)", generateFacultyNumber(), s.Email); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}

func generateFacultyNumber() string {
	return fmt.Sprint(10000000 + rand.Intn(99999999-10000000))
}

func (conn dbConnection) updateStudent(s Student) error {
	tx, err := conn.db.Begin()

	defer func(tx *sql.Tx) {
		_ = tx.Rollback()
	}(tx)

	if err != nil {
		return err
	}

	if err = insertPerson(tx, person{s.Name, s.Email, s.Phone}); err != nil {
		return err
	}

	if _, err = tx.Exec("UPDATE student SET person_id=$1 WHERE faculty_number=$2", s.Email, s.FacultyNumber); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (conn dbConnection) getAllTeachers() (teachers []Teacher, err error) {
	if err = conn.db.Select(&teachers, "SELECT name as Name, phone as Phone, email as Email FROM teacher JOIN person p on p.email = teacher.person_id WHERE teacher.active=TRUE"); err != nil {
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

	if err = insertPerson(tx, person{t.Name, t.Email, t.Phone}); err != nil {
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

	if err = insertPerson(tx, person{t.Name, t.Email, t.Phone}); err != nil {
		return err
	}

	//TODO: Fix
	if _, err = tx.Exec("UPDATE teacher SET person_id=$1 WHERE id=$2", t.Email, "fix"); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (conn dbConnection) delete(table, uuid string) (err error) {
	switch table {
	case "course":
		_, err = conn.db.Exec("UPDATE course SET deleted=TRUE WHERE name=$1", uuid)
	default:
		err = fmt.Errorf("unknown table")
	}

	if err != nil {
		log.Printf("Failed to delete %s with id %s", table, uuid)
		return err
	}
	return nil
}

func (conn dbConnection) getUsers(role string) (any, error) {
	switch role {
	case "student":
		return conn.getAllStudents()
	case "teacher":
		return conn.getAllTeachers()
	default:
		return nil, fmt.Errorf("unknown role")
	}
}

func insertPerson(tx *sql.Tx, p person) error {
	defaultPass := []byte(uniuri.NewLen(7))
	from := os.Getenv("MAIL")
	emailCred := os.Getenv("PASSWD")

	toList := []string{"ilianbb4@gmail.com"}
	host := "smtp.gmail.com"
	port := "587"
	body := []byte(fmt.Sprintf("To: %s\r\n"+"Subject: Technical university password!\r\n"+"\r\n"+"This is your password: %s\r\n", p.Email, defaultPass))

	auth := smtp.PlainAuth("", from, emailCred, host)

	if err := smtp.SendMail(host+":"+port, auth, from, toList, body); err != nil {
		fmt.Println(err)
		if err = tx.Rollback(); err != nil {
			return err
		}
	}

	hashedPassword, err := bcrypt.GenerateFromPassword(defaultPass, bcrypt.DefaultCost)
	if err != nil {
		log.Println(err)
		if err = tx.Rollback(); err != nil {
			return err
		}
	}

	if _, err = tx.Exec("INSERT INTO person(name, email, phone, password) VALUES ($1, $2, $3, $4)", p.Name, p.Email, p.Phone, hashedPassword); err != nil {
		return err
	}
	return nil
}

func (conn dbConnection) getTeacherExams() (exams []Exam, err error) {
	if err = conn.db.Select(&exams, "SELECT c.name as CourseName, p.name as StudentName, student_faculty_number as StudentFacultyNumber, points as Points FROM exam JOIN student s on s.faculty_number = exam.student_faculty_number JOIN person p on p.email = s.person_id JOIN course c on c.id = exam.course_id WHERE exam.deleted=FALSE"); err != nil {
		log.Printf("Failed to get exams")
		return nil, err
	}
	return exams, nil
}

func (conn dbConnection) archiveUser(email, role string) (err error) {

	switch role {
	case "student":
		_, err = conn.db.Exec("UPDATE student SET active=FALSE WHERE person_id=$1", email)
	case "teacher":
		_, err = conn.db.Exec("UPDATE teacher SET active=FALSE WHERE person_id=$1", email)
	default:
		err = fmt.Errorf("unknown table")
	}

	if err != nil {
		log.Printf("Failed to delete %s with id %s", role, email)
		return err
	}
	return nil
}
