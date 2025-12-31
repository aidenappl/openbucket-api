package routers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/aidenappl/openbucket-api/env"
	"github.com/aidenappl/openbucket-api/responder"
	"github.com/aidenappl/openbucket-api/tools"
	"github.com/golang-jwt/jwt/v5"
)

var jwtSecret = []byte(env.JWT_SECRET)

type CreateSessionRequest struct {
	BucketName string  `json:"bucket"`
	Nickname   string  `json:"nickname"`
	Region     string  `json:"region"`
	Endpoint   string  `json:"endpoint"`
	AccessKey  *string `json:"access_key_id"`
	SecretKey  *string `json:"secret_access_key"`
}

type CreateSessionResponse struct {
	Token string `json:"token"`
}

func HandleCreateSession(w http.ResponseWriter, r *http.Request) {
	var body CreateSessionRequest

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		responder.SendError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Basic validation
	if body.BucketName == "" || body.Region == "" || body.Endpoint == "" {
		responder.SendError(w, http.StatusBadRequest, "Missing required fields", nil)
		return
	}

	claims := tools.SessionClaims{
		BucketName: body.BucketName,
		Nickname:   body.Nickname,
		Region:     body.Region,
		Endpoint:   body.Endpoint,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(12 * time.Hour)),
		},
	}

	if body.AccessKey != nil {
		accessKeyEnc, err := tools.Encrypt(*body.AccessKey)
		if err != nil {
			responder.SendError(w, http.StatusConflict, "Encryption failed", err)
			return
		}
		claims.AccessKey = &accessKeyEnc
	}

	if body.SecretKey != nil {
		secretKeyEnc, err := tools.Encrypt(*body.SecretKey)
		if err != nil {
			responder.SendError(w, http.StatusConflict, "Encryption failed", err)
			return
		}
		claims.SecretKey = &secretKeyEnc
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(jwtSecret)
	if err != nil {
		responder.SendError(w, http.StatusInternalServerError, "Failed to sign token", err)
		return
	}

	responder.New(w, CreateSessionResponse{
		Token: signedToken,
	}, "successfully created session")
}
