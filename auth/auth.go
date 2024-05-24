package auth

import (
	"xicserver/models"

	jwt "github.com/golang-jwt/jwt"
)

func IsValidToken(tokenString string) (bool, error) {
	claims := &models.MyClaims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return JwtKey, nil
	})

	if err != nil {
		return false, err
	}

	return token.Valid, nil
}
