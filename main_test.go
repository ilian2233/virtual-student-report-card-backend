package main

import (
	"log"
	"net/http"
	"net/http/httptest"
	"os"
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

	tests := []struct {
		name                string
		req                 *http.Request
		expectedResp        *http.Response
		responseCompareFunc func(expected, actual *http.Response) bool
	}{
		{
			"Successful auth",
			httptest.NewRequest("POST", "/login", nil),
			&http.Response{
				StatusCode: 400,
			},
			defaultResponseCompareFunc,
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
