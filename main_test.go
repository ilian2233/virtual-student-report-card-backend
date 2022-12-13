package main

import (
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

	h := handler{
		secretKet: os.Getenv("SECRET_KEY"),
		db:        db,
	}

	mainHandler := http.NewServeMux()
	mainHandler.HandleFunc("/login", h.handleLogin)

	defaultResponseCompareFunc := func(expected, actual *http.Response) bool {
		return expected.StatusCode == actual.StatusCode
	}

	requestWithAuth := func(method, target string, body io.Reader) *http.Request {
		r := httptest.NewRequest(method, target, body)
		r.Header.Add("Authorization", os.Getenv("TOKEN"))
		return r
	}

	tests := []struct {
		name                string
		req                 *http.Request
		expectedResp        *http.Response
		responseCompareFunc func(expected, actual *http.Response) bool
	}{
		{
			"Unsuccessful auth",
			httptest.NewRequest(http.MethodPost, "/login", nil),
			&http.Response{
				StatusCode: http.StatusBadRequest,
			},
			defaultResponseCompareFunc,
		},
		{
			"Successful auth",
			httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{ "Email":"test@test.com", "Password": "test_pas_123"}`)),
			&http.Response{
				StatusCode: http.StatusOK,
			},
			defaultResponseCompareFunc,
		},
		{
			"Get student exams without auth",
			httptest.NewRequest(http.MethodGet, "/student/exams", nil),
			&http.Response{
				StatusCode: http.StatusForbidden,
			},
			defaultResponseCompareFunc,
		},
		{
			name: "Get student exams",
			req:  requestWithAuth(http.MethodGet, "/student/exams", nil),
			expectedResp: &http.Response{
				StatusCode: 200,
			},
			responseCompareFunc: defaultResponseCompareFunc,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			respRec := httptest.NewRecorder()

			mainHandler.ServeHTTP(respRec, test.req)

			if !test.responseCompareFunc(respRec.Result(), test.expectedResp) {
				t.Errorf("Expected response %d, but got %d", test.expectedResp.StatusCode, respRec.Result().StatusCode)
			}

		})
	}
}
