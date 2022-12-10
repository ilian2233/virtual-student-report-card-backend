package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
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

	if !claims["roles"].(roles).contains(role) {
		return "", errMissingRole
	}

	id := claims["uuid"].(string)
	if _, err = uuid.Parse(id); err != nil {
		return "", jwt.ErrTokenInvalidId
	}

	return id, nil
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

	if reqHeader["Authorization"] == nil {
		return nil, fmt.Errorf("can not find token in header")
	}

	token, err := jwt.Parse(reqHeader["Authorization"][0], func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("there was an error in parsing")
		}
		return os.Getenv("SECRET_KEY"), nil
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
