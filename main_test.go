package main

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/joho/godotenv"
)

func Test_main(t *testing.T) {
	if err := godotenv.Load(); err != nil {
		log.Fatalf("Error loading .env file \n%e", err)
	}

	db, err := createDatabaseConnection()
	if err != nil {
		log.Fatal(err)
	}

	h := setupHandler(db)

	tests := []struct {
		name               string
		req                *http.Request
		expectedStatusCode int
		expectedBody       []byte
	}{
		{
			"Unsuccessful auth",
			httptest.NewRequest(http.MethodPost, "/login", nil),
			http.StatusBadRequest,
			[]byte(`{"message":"Invalid body"}`),
		},
		{
			"Successful auth",
			httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{ "Email":"test@test.com", "Password": "test_pas_123"}`)),
			http.StatusOK,
			nil,
		},
		{
			"Get student exams without auth",
			httptest.NewRequest(http.MethodGet, "/student/exams", nil),
			http.StatusForbidden,
			[]byte(`{"message":"unauthorized"}`),
		},
		{
			"Get student exams",
			requestWithAuth(http.MethodGet, "/student/exams", nil, "student"),
			http.StatusOK,
			[]byte(`[{"StudentName":"ivan1","StudentFacultyNumber":"","CourseName":"Math","Points":56}]`),
		},
		{
			"Unauthorised access teacher",
			requestWithAuth(http.MethodGet, "/teacher/courses", nil, "student"),
			http.StatusForbidden,
			[]byte(`{"message":"unauthorized"}`),
		},
		{
			"Unauthorised access teacher",
			requestWithAuth(http.MethodGet, "/teacher/students", nil, "student"),
			http.StatusForbidden,
			[]byte(`{"message":"unauthorized"}`),
		},
		{
			"Get teacher courses",
			requestWithAuth(http.MethodGet, "/teacher/courses", nil, "teacher"),
			http.StatusOK,
			[]byte(`["Math","Programming Basics"]`),
		},
		{
			"Get teacher students",
			requestWithAuth(http.MethodGet, "/teacher/students", nil, "teacher"),
			http.StatusOK,
			[]byte(`["test1@test.com"]`),
		},
		{
			"Post exam with empty body",
			requestWithAuth(http.MethodPost, "/teacher/exams", nil, "teacher"),
			http.StatusBadRequest,
			[]byte(`{"message":"content must be provided in request body"}`),
		},
		{
			"Post exam success",
			requestWithAuth(http.MethodPost, "/teacher/exams", strings.NewReader(`{ "StudentFacultyNumber":"12312312", "CourseName": "Math", "Points": 42}`), "teacher"),
			http.StatusOK,
			[]byte(`{"message":"success"}`),
		},
		{
			"Post exam success1",
			requestWithAuth(http.MethodPost, "/teacher/exams", strings.NewReader(`{"CourseName":"Math","StudentFacultyNumber":"12312312","Points":34}`), "teacher"),
			http.StatusOK,
			[]byte(`{"message":"success"}`),
		},
		{
			"Post student success",
			requestWithAuth(http.MethodPost, "/admin/students", strings.NewReader(`{"Name": "ivan3","StudentFacultyNumber":"23423423", "Email": "test3@test.com", "Phone": "0881234567"}`), "admin"),
			http.StatusOK,
			[]byte(`{"message":"success"}`),
		},
		{
			"Get teachers",
			requestWithAuth(http.MethodGet, "/admin/teachers", nil, "admin"),
			http.StatusOK,
			[]byte(`["test2@test.com"]`),
		},
		{
			"Post student with empty data",
			requestWithAuth(http.MethodPost, "/admin/students", strings.NewReader(`{"Name":"","Email":"","Phone":""}`), "admin"),
			http.StatusBadRequest,
			[]byte(`{"message":"something went wrong"}`),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			respRec := httptest.NewRecorder()

			h.ServeHTTP(respRec, test.req)

			if respRec.Result().StatusCode != test.expectedStatusCode {
				t.Fatalf("Expected response %d, but got %d", test.expectedStatusCode, respRec.Result().StatusCode)
			}

			defer func(Body io.ReadCloser) {
				err = Body.Close()
				if err != nil {

				}
			}(respRec.Result().Body)

			if test.expectedBody == nil {
				return
			}

			body, err := io.ReadAll(respRec.Result().Body)
			if err != nil {
				t.Fatal(err)
			}

			if bytes.Compare(body, test.expectedBody) != 0 {
				t.Fatalf("Expected response %s, but got %s", test.expectedBody, body)
			}
		})
	}
}

func requestWithAuth(method, target string, body io.Reader, authLevel string) *http.Request {
	r := httptest.NewRequest(method, target, body)
	switch authLevel {
	case "student":
		r.Header.Add("Authorization", os.Getenv("TOKEN_STUDENT"))
	case "teacher":
		r.Header.Add("Authorization", os.Getenv("TOKEN_TEACHER"))
	default:
		r.Header.Add("Authorization", os.Getenv("TOKEN_ADMIN"))
	}
	return r
}
