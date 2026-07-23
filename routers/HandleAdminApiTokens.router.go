package routers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aidenappl/openbucket-api/db"
	"github.com/aidenappl/openbucket-api/middleware"
	"github.com/aidenappl/openbucket-api/query"
	"github.com/aidenappl/openbucket-api/responder"
	"github.com/aidenappl/openbucket-api/structs"
	"github.com/aidenappl/openbucket-api/tools"
	"github.com/gorilla/mux"
)

// HandleAdminListApiTokens handles GET /admin/api-tokens.
// Optionally filter by user_id; omit it to list all active tokens.
func HandleAdminListApiTokens(w http.ResponseWriter, r *http.Request) {
	req := query.ListApiTokensRequest{}

	if userIDStr := r.URL.Query().Get("user_id"); userIDStr != "" {
		userID, err := strconv.Atoi(userIDStr)
		if err != nil {
			responder.SendError(w, http.StatusBadRequest, "invalid user_id")
			return
		}
		req.UserID = &userID
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil {
			responder.SendError(w, http.StatusBadRequest, "invalid limit")
			return
		}
		req.Limit = limit
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		offset, err := strconv.Atoi(offsetStr)
		if err != nil {
			responder.SendError(w, http.StatusBadRequest, "invalid offset")
			return
		}
		req.Offset = offset
	}

	tokens, err := query.ListApiTokens(db.DB, req)
	if err != nil {
		responder.SendError(w, http.StatusInternalServerError, "failed to list api tokens", err)
		return
	}

	responder.New(w, tokens, "api tokens retrieved")
}

// HandleAdminCreateApiToken handles POST /admin/api-tokens.
// The token is minted for the authenticated caller. The plaintext value is
// returned exactly once — only its hash is persisted.
func HandleAdminCreateApiToken(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUserFromContext(r.Context())
	if !ok || user == nil {
		responder.SendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var body structs.CreateApiTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		responder.SendError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		responder.SendError(w, http.StatusBadRequest, "name is required")
		return
	}
	if len(body.Name) > 255 {
		responder.SendError(w, http.StatusBadRequest, "name too long (max 255 characters)")
		return
	}

	var expiresAt *time.Time
	switch strings.ToLower(strings.TrimSpace(body.ExpiresIn)) {
	case "never":
		// expiresAt stays nil — token never expires
	case "", "90d":
		t := time.Now().Add(90 * 24 * time.Hour)
		expiresAt = &t
	case "30d":
		t := time.Now().Add(30 * 24 * time.Hour)
		expiresAt = &t
	case "365d":
		t := time.Now().Add(365 * 24 * time.Hour)
		expiresAt = &t
	default:
		responder.SendError(w, http.StatusBadRequest, "invalid expires_in value, use: 30d, 90d, 365d, or never")
		return
	}

	plaintext, hash, err := tools.GenerateApiToken()
	if err != nil {
		responder.SendError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	token, err := query.CreateApiToken(db.DB, query.CreateApiTokenRequest{
		UserID:    user.ID,
		Name:      body.Name,
		TokenHash: hash,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		responder.SendError(w, http.StatusInternalServerError, "failed to create api token", err)
		return
	}

	responder.New(w, structs.CreateApiTokenResponse{
		ApiToken: *token,
		Token:    plaintext,
	}, "api token created — copy it now, it will not be shown again")
}

// HandleAdminRevokeApiToken handles DELETE /admin/api-tokens/{id}.
// Revocation is a soft delete: the row is retained so last_used_at stays
// auditable after the token stops working.
func HandleAdminRevokeApiToken(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		responder.SendError(w, http.StatusBadRequest, "invalid id")
		return
	}

	token, err := query.GetApiTokenByID(db.DB, id)
	if err != nil {
		responder.SendError(w, http.StatusInternalServerError, "failed to get api token", err)
		return
	}
	if token == nil {
		responder.SendError(w, http.StatusNotFound, "api token not found")
		return
	}

	if err := query.RevokeApiToken(db.DB, id); err != nil {
		responder.SendError(w, http.StatusInternalServerError, "failed to revoke api token", err)
		return
	}

	responder.New(w, nil, "api token revoked")
}
