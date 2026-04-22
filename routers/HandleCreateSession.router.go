package routers

import (
	"encoding/json"
	"net/http"
	"net/url"

	forta "github.com/aidenappl/go-forta"
	"github.com/aidenappl/openbucket-api/db"
	"github.com/aidenappl/openbucket-api/query"
	"github.com/aidenappl/openbucket-api/responder"
	"github.com/aidenappl/openbucket-api/tools"
)

type CreateSessionRequest struct {
	BucketName string  `json:"bucket"`
	Nickname   string  `json:"nickname"`
	Region     string  `json:"region"`
	Endpoint   string  `json:"endpoint"`
	AccessKey  *string `json:"access_key_id"`
	SecretKey  *string `json:"secret_access_key"`
}

func HandleCreateSession(w http.ResponseWriter, r *http.Request) {
	fortaID, ok := forta.GetFortaIDFromContext(r.Context())
	if !ok {
		responder.SendError(w, http.StatusUnauthorized, "unauthenticated", nil)
		return
	}

	var body CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		responder.SendError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	if body.BucketName == "" || body.Region == "" || body.Endpoint == "" {
		responder.SendError(w, http.StatusBadRequest, "Missing required fields", nil)
		return
	}

	// Validate endpoint is a valid HTTP(S) URL
	epURL, err := url.Parse(body.Endpoint)
	if err != nil || (epURL.Scheme != "http" && epURL.Scheme != "https") || epURL.Host == "" {
		responder.SendError(w, http.StatusBadRequest, "endpoint must be a valid HTTP(S) URL", nil)
		return
	}

	req := query.InsertSessionRequest{
		FortaUserID: fortaID,
		BucketName:  body.BucketName,
		Nickname:    body.Nickname,
		Region:      body.Region,
		Endpoint:    body.Endpoint,
	}

	if body.AccessKey != nil {
		enc, err := tools.Encrypt(*body.AccessKey)
		if err != nil {
			responder.SendError(w, http.StatusInternalServerError, "Encryption failed", err)
			return
		}
		req.AccessKey = &enc
	}

	if body.SecretKey != nil {
		enc, err := tools.Encrypt(*body.SecretKey)
		if err != nil {
			responder.SendError(w, http.StatusInternalServerError, "Encryption failed", err)
			return
		}
		req.SecretKey = &enc
	}

	id, err := query.InsertSession(db.DB, req)
	if err != nil {
		responder.SendError(w, http.StatusInternalServerError, "Failed to create session", err)
		return
	}

	sess, err := query.GetSessionByID(db.DB, id)
	if err != nil {
		responder.SendError(w, http.StatusInternalServerError, "Failed to retrieve session", err)
		return
	}

	responder.New(w, sess.ToPublic(), "successfully created session")
}
