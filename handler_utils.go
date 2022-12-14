package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/golang-jwt/jwt/v4"
)

var (
	errForbiddenMethod = errors.New("method not allowed")
	errValidatingJWT   = errors.New("failed to validate jwt token")
	errMissingRole     = errors.New("role is not present in token")
)

func performChecks(methods []string, role string, r *http.Request) (string, error) {
	if !isMethodAllowed(methods, r.Method) {
		return "", errForbiddenMethod
	}

	token, err := validateToken(r.Header)
	if err != nil {
		return "", errValidatingJWT
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", jwt.ErrTokenInvalidClaims
	}

	roleClaims := Roles(claims["roles"].([]interface{}))

	if !roleClaims.contains(role) {
		return "", errMissingRole
	}

	email := claims["email"].(string)
	if email == "" {
		return "", jwt.ErrTokenInvalidId
	}

	return email, nil
}

func isMethodAllowed(methods []string, method string) bool {
	for _, v := range methods {
		if v == method {
			return true
		}
	}
	return false
}

func respondWithMessage(w http.ResponseWriter, messageTxt string, statusCode int) {
	type messageResponse struct {
		Message string `json:"message"`
	}

	mess := &messageResponse{Message: messageTxt}

	resp, _ := json.Marshal(mess)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_, _ = w.Write(resp)
}

func validateToken(reqHeader http.Header) (*jwt.Token, error) {

	if reqHeader.Get("Authorization") == "" {
		return nil, fmt.Errorf("can not find token in header")
	}

	token, err := jwt.Parse(reqHeader.Get("Authorization"), func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("there was an error in parsing")
		}
		return []byte(os.Getenv("SECRET_KEY")), nil
	})

	if token.Valid {
		return token, nil
	} else if errors.Is(err, jwt.ErrTokenMalformed) {
		return nil, fmt.Errorf("that's not even a token")
	} else if errors.Is(err, jwt.ErrTokenExpired) || errors.Is(err, jwt.ErrTokenNotValidYet) { // Token is either expired or not active yet
		return nil, fmt.Errorf("token is either expired or not active yet")
	} else {
		return nil, fmt.Errorf("couldn't handle this token \n%e", err)
	}
}

func corsHandler(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "OPTIONS" {
			headers := w.Header()
			headers.Add("Access-Control-Allow-Origin", "*")
			headers.Add("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, Authorization")
			headers.Add("Access-Control-Allow-Credentials", "true")
			headers.Add("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			w.WriteHeader(http.StatusOK)
			return
		} else {
			h(w, r)
			return
		}
	}
}
