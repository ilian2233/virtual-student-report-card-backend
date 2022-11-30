package main

const schema = `
DROP TABLE IF EXISTS course_with_points;
DROP TABLE IF EXISTS exam;
DROP TABLE IF EXISTS course;
DROP TABLE IF EXISTS curriculum;
DROP TABLE IF EXISTS admin;
DROP TABLE IF EXISTS student;
DROP TABLE IF EXISTS teacher;
DROP TABLE IF EXISTS person;
-- Temporary while working on the query

CREATE TABLE IF NOT EXISTS person (
   	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    phone TEXT UNIQUE,
    email TEXT NOT NULL UNIQUE CHECK (email ~ '^[A-Za-z0-9._%-]+@[A-Za-z0-9.-]+[.][A-Za-z]+$'),
    password TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS admin (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    person_id UUID REFERENCES person(id) NOT NULL,
    active BOOLEAN DEFAULT TRUE
);

CREATE TABLE IF NOT EXISTS student (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    person_id UUID REFERENCES person(id) NOT NULL
);

CREATE TABLE IF NOT EXISTS teacher (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    person_id UUID REFERENCES person(id) NOT NULL,
    active BOOLEAN DEFAULT TRUE
);

CREATE TABLE IF NOT EXISTS curriculum (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    required_curriculum_points_to_pass INT DEFAULT 100 CHECK (required_curriculum_points_to_pass > 0)
);

-- Given subject of study e.g. math
CREATE TABLE IF NOT EXISTS course (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    teacher_id UUID REFERENCES teacher(id) NOT NULL,
    name TEXT NOT NULL,
    number_of_seats INT DEFAULT 50 CHECK (number_of_seats > 0)
);

-- Exists because different courses can have different points depending in the curriculum
CREATE TABLE IF NOT EXISTS course_with_points (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    curriculum_id UUID REFERENCES curriculum(id) NOT NULL,
    course_id UUID REFERENCES course(id) NOT NULL,
    curriculum_points INT CHECK (curriculum_points > 0)
);

CREATE TABLE IF NOT EXISTS exam (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    course_id UUID REFERENCES course(id) NOT NULL,
    student_id UUID REFERENCES student(id) NOT NULL,
    points INT CHECK (points > 0)
);`

const addExampleData = `
INSERT INTO person(name, phone, email, password) VALUES 
    ('ivan', '0881234563', 'test@test.com', 'test_pas_123'),
    ('ivan1', '0881234564', 'test1@test.com', 'test_pas_123'),
    ('ivan2', '0881234565', 'test2@test.com', 'test_pas_123');

INSERT INTO admin(person_id) VALUES 
    ((SELECT id FROM person WHERE name='ivan'));

INSERT INTO student(person_id) VALUES 
    ((SELECT id FROM person WHERE name='ivan1'));

INSERT INTO teacher(person_id) VALUES 
    ((SELECT id FROM person WHERE name='ivan2'));

INSERT INTO curriculum(name) VALUES ('SIT'), ('KST');

INSERT INTO course(teacher_id, name) VALUES 
    ((SELECT id FROM teacher LIMIT 1),'Math'), 
    ((SELECT id FROM teacher LIMIT 1),'Programming Basics');

INSERT INTO course_with_points(curriculum_id, course_id, curriculum_points) VALUES 
    ((SELECT id FROM curriculum WHERE name='SIT'),(SELECT id FROM course WHERE name='Math'), 3);

INSERT INTO exam(course_id, student_id, points) VALUES 
    ((SELECT id FROM course WHERE name='Math'),(SELECT id FROM student LIMIT 1), 56);
`
