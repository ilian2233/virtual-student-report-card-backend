package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/jmoiron/sqlx"
)

//TODO: Update teacher and student insert and update strategies

const (
	schema = `
DROP TABLE IF EXISTS exam;
DROP TABLE IF EXISTS course;
DROP TABLE IF EXISTS curriculum;
DROP TABLE IF EXISTS admin;
DROP TABLE IF EXISTS student;
DROP TABLE IF EXISTS teacher;
DROP TABLE IF EXISTS person;
-- Temporary while working on the query

CREATE TABLE IF NOT EXISTS person (
    email TEXT NOT NULL PRIMARY KEY UNIQUE CHECK (email ~ '^[A-Za-z0-9._%-]+@[A-Za-z0-9.-]+[.][A-Za-z]+$'),
    name TEXT NOT NULL,
    phone TEXT UNIQUE,
    password TEXT NOT NULL
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

CREATE TABLE IF NOT EXISTS curriculum (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    required_curriculum_points_to_pass INT DEFAULT 100 CHECK (required_curriculum_points_to_pass > 0),
    deleted BOOL DEFAULT FALSE
);

-- Given subject of study e.g. math
CREATE TABLE IF NOT EXISTS course (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    teacher_id UUID REFERENCES teacher(id) NOT NULL,
    curriculum_id UUID REFERENCES curriculum(id) NOT NULL,
    name TEXT NOT NULL UNIQUE,
    number_of_seats INT DEFAULT 50 CHECK (number_of_seats > 0),
    deleted BOOL DEFAULT FALSE
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
    ('ivan', '0881234563', 'test@test.com', 'test_pas_123'),
    ('ivan1', '0881234564', 'test1@test.com', 'test_pas_123'),
    ('ivan2', '0881234565', 'test2@test.com', 'test_pas_123');

INSERT INTO admin(person_id) VALUES 
    ((SELECT email FROM person WHERE name='ivan'));

INSERT INTO student(person_id) VALUES 
    ((SELECT email FROM person WHERE name='ivan1'));

INSERT INTO teacher(person_id) VALUES 
    ((SELECT email FROM person WHERE name='ivan2'));

INSERT INTO curriculum(name) VALUES ('SIT'), ('KST');

INSERT INTO course(teacher_id, curriculum_id, name) VALUES 
    ((SELECT id FROM teacher LIMIT 1), (SELECT id FROM curriculum LIMIT 1), 'Math'), 
    ((SELECT id FROM teacher LIMIT 1), (SELECT id FROM curriculum LIMIT 1), 'Programming Basics');

INSERT INTO exam(course_id, student_id, points) VALUES 
    ((SELECT id FROM course WHERE name='Math'),(SELECT id FROM student LIMIT 1), 56);
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

	db.MustExec(schema)
	log.Println("DB schema created successfully")

	db.MustExec(addExampleData)
	log.Println("DB populated with example data")

	return dbConnection{
		db: db,
	}, nil
}

func (conn dbConnection) validateUserLogin(email string, password []byte) bool {
	var p Student
	if err := conn.db.Get(&p, "SELECT email, password FROM person WHERE email=$1", email); err != nil {
		log.Printf("Failed to query db,\n %e", err)
		return false
	}

	//TODO: Uncomment when passwords are heshed
	//if err := bcrypt.CompareHashAndPassword([]byte(p.Password), password); err != nil {
	//	log.Printf("Password did not match \n%e", err)
	//	return false
	//}
	if p.Password != string(password) {
		log.Printf("Password did not match")
		return false
	}

	return true
}

func (conn dbConnection) getUserRoles(uuid string) (roles []string) {
	if err := conn.db.Get(nil, "SELECT id FROM admin WHERE person_id=$1", uuid); err == nil {
		roles = append(roles, "Admin")
	}

	if err := conn.db.Get(nil, "SELECT id FROM student WHERE person_id=$1", uuid); err == nil {
		roles = append(roles, "Student")
	}

	if err := conn.db.Get(nil, "SELECT id FROM teacher WHERE person_id=$1", uuid); err == nil {
		roles = append(roles, "Teacher")
	}

	return roles
}

func (conn dbConnection) getStudentExams(uuid string) (exams []studentExam, err error) {
	if err = conn.db.Select(&exams, "SELECT name as courseName, points FROM exam JOIN course c on c.id = exam.course_id WHERE student_id=$1 AND c.deleted=FALSE", uuid); err == nil {
		return exams, err
	}
	return exams, nil
}

func (conn dbConnection) insertExam(teacherUUID string, e exam) error {
	var courses []course
	if err := conn.db.Select(&courses, "SELECT id FROM course WHERE teacher_id=$1 AND deleted=FALSE", teacherUUID); err == nil {
		log.Printf("Failed to get teacher courses")
		return err
	}

	//TODO: Check if teacher leads exam

	if _, err := conn.db.Exec("INSERT INTO exam(course_id, student_id, points) VALUES ($1, $2, $3)", e.courseID, e.studentID, e.points); err != nil {
		return err
	}
	return nil
}

func (conn dbConnection) getAllCurriculums() (curriculums []curriculum, err error) {
	if err = conn.db.Select(&curriculums, "SELECT id, name, required_curriculum_points_to_pass as requiredPoints FROM curriculum WHERE deleted=FALSE"); err == nil {
		log.Printf("Failed to get curriculums")
		return nil, err
	}
	return curriculums, nil
}

func (conn dbConnection) insertCurriculum(c curriculum) error {
	if _, err := conn.db.Exec("INSERT INTO curriculum(name, required_curriculum_points_to_pass) VALUES ($1, $2)", c.name, c.requiredPoints); err != nil {
		return err
	}
	return nil
}

func (conn dbConnection) updateCurriculum(c curriculum) error {
	if _, err := conn.db.Exec("UPDATE curriculum SET name=$1, required_curriculum_points_to_pass=$2 WHERE id=$3", c.name, c.requiredPoints, c.id); err != nil {
		return err
	}
	return nil
}

func (conn dbConnection) getAllCourses() (courses []course, err error) {
	if err = conn.db.Select(&courses, "SELECT id, teacher_id as teacherId, curriculum_id as curriculumId, name, number_of_seats as numberOfSeats FROM course WHERE deleted=FALSE"); err == nil {
		log.Printf("Failed to get courses")
		return nil, err
	}
	return courses, nil
}

func (conn dbConnection) insertCourse(c course) error {
	if _, err := conn.db.Exec("INSERT INTO course(teacher_id, curriculum_id, name, number_of_seats) VALUES ($1, $2, $3, $4)", c.teacherId, c.curriculumId, c.name, c.numberOfSeats); err != nil {
		return err
	}
	return nil
}

func (conn dbConnection) updateCourse(c course) error {
	if _, err := conn.db.Exec("UPDATE course SET teacher_id=$1, curriculum_id=$2, name=$3, number_of_seats=$4 WHERE id=$5", c.teacherId, c.curriculumId, c.name, c.numberOfSeats, c.id); err != nil {
		return err
	}
	return nil
}

func (conn dbConnection) getAllExams() (exams []exam, err error) {
	if err = conn.db.Select(&exams, "SELECT id, course_id as courseID, student_id as studentID, points FROM exam WHERE deleted=FALSE"); err == nil {
		log.Printf("Failed to get exams")
		return nil, err
	}
	return exams, nil
}

func (conn dbConnection) getAllStudents() (students []Student, err error) {
	if err = conn.db.Select(&students, "SELECT id, person_id as personID FROM student"); err == nil {
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

	if _, err = tx.Exec("UPDATE student SET person_id=$1 WHERE id=$2", s.Email, s.id); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (conn dbConnection) getAllTeachers() (teachers []Teacher, err error) {
	if err = conn.db.Select(&teachers, "SELECT id, person_id as personID FROM teacher"); err == nil {
		log.Printf("Failed to get teachers")
		return nil, err
	}
	return teachers, nil
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

	if _, err = tx.Exec("UPDATE teacher SET person_id=$1 WHERE id=$2", t.Email, t.id); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (conn dbConnection) delete(table, uuid string) error {
	if _, err := conn.db.Exec("UPDATE $1 SET deleted=TRUE WHERE id=$2", table, uuid); err == nil {
		log.Printf("Failed to delete %s with id %s", table, uuid)
		return err
	}
	return nil
}

func insertPerson(tx *sql.Tx, s Student) error {
	//TODO: Hash Password
	//TODO: Add salt
	hashedPassword := s.Password

	if _, err := tx.Exec("INSERT INTO person(name, email, phone, password) VALUES ($1, $2, $3, $4)", s.name, s.Email, s.phone, hashedPassword); err != nil {
		return err
	}
	return nil
}
