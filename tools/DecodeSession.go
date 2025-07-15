package tools

import (
	"errors"
	"fmt"

	"github.com/aidenappl/openbucket-api/env"
	"github.com/golang-jwt/jwt/v5"
)

type SessionClaims struct {
	BucketName string  `json:"bucket"`
	Nickname   string  `json:"nickname"`
	Region     string  `json:"region"`
	Endpoint   string  `json:"endpoint"`
	AccessKey  *string `json:"accessKey,omitempty"` // will be decrypted
	SecretKey  *string `json:"secretKey,omitempty"` // will be decrypted
	Token      *string `json:"token,omitempty"`     // optional, for session management
	jwt.RegisteredClaims
}

var secret = []byte(env.JWT_SECRET)

func DecodeAndDecryptSession(tokenString string) (*SessionClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &SessionClaims{}, func(token *jwt.Token) (interface{}, error) {
		return secret, nil
	})
	if err != nil || !token.Valid {
		if errors.Is(err, jwt.ErrTokenExpired) {
			claims, ok := token.Claims.(*SessionClaims)
			if !ok {
				return nil, errors.New("invalid claim structure")
			}
			return claims, err
		}
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*SessionClaims)
	if !ok {
		return nil, errors.New("invalid claim structure")
	}

	// Decrypt keys
	if claims.AccessKey != nil {
		decryptedAccess, err := Decrypt(*claims.AccessKey)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt access key: %w", err)
		}
		claims.AccessKey = &decryptedAccess
	}

	if claims.SecretKey != nil {
		decryptedSecret, err := Decrypt(*claims.SecretKey)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt secret key: %w", err)
		}
		claims.SecretKey = &decryptedSecret
	}

	claims.Token = &tokenString

	return claims, nil
}
