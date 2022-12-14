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

func Test_login(t *testing.T) {
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
	}{
		{
			"Unsuccessful auth",
			httptest.NewRequest(http.MethodPost, "/login", nil),
			http.StatusBadRequest,
		},
		{
			"Successful auth",
			httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{ "Email":"test@test.com", "Password": "test_pas_123"}`)),
			http.StatusOK,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			respRec := httptest.NewRecorder()

			h.ServeHTTP(respRec, test.req)

			if respRec.Result().StatusCode != test.expectedStatusCode {
				t.Fatalf("Expected response %d, but got %d", test.expectedStatusCode, respRec.Result().StatusCode)
			}
		})
	}
}

func Test_exams(t *testing.T) {
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
			"Get student exams without auth",
			httptest.NewRequest(http.MethodGet, "/student/exams", nil),
			http.StatusForbidden,
			[]byte(`{"message":"unauthorized"}`),
		},
		{
			"Get student exams",
			requestWithAuth(http.MethodGet, "/student/exams", nil, "student"),
			http.StatusOK,
			[]byte(`[{"Id":"f83f29bb-ab1c-4b99-9297-75ca1afaaee1","StudentName":"ivan1","CourseName":"Math","Points":56}]`),
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
