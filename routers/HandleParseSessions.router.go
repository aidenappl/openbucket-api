package routers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/aidenappl/openbucket-api/responder"
	"github.com/aidenappl/openbucket-api/tools"
	"github.com/golang-jwt/jwt/v5"
)

type HandleParseSessionsRequest struct {
	Sessions []string `json:"sessions"`
}

func HandleParseSessions(w http.ResponseWriter, r *http.Request) {
	var body HandleParseSessionsRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		responder.SendError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	if len(body.Sessions) == 0 {
		responder.SendError(w, http.StatusBadRequest, "No sessions provided", nil)
		return
	}
	var parsedSessions []tools.SessionClaims

	for _, session := range body.Sessions {
		claims, err := tools.DecodeAndDecryptSession(session)
		if err != nil {
			if errors.Is(err, jwt.ErrTokenExpired) {
				// regenerate token
				log.Println("Session expired, regenerating token")
				token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
				signedToken, err := token.SignedString(jwtSecret)
				if err != nil {
					responder.SendError(w, http.StatusInternalServerError, "Failed to sign token", err)
					return
				}
				claims.Token = &signedToken
			} else {
				responder.SendError(w, http.StatusBadRequest, "Failed to decode session", err)
				return
			}
		}
		if claims == nil {
			responder.SendError(w, http.StatusBadRequest, "Invalid session format", nil)
			return
		}

		claims.AccessKey = nil
		claims.SecretKey = nil

		parsedSessions = append(parsedSessions, *claims)
	}

	responder.New(w, parsedSessions, "Sessions parsed successfully")
}
