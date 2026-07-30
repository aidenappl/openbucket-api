package routers

import (
	"encoding/json"
	"net/http"

	"github.com/aidenappl/openbucket-api/db"
	"github.com/aidenappl/openbucket-api/query"
	"github.com/aidenappl/openbucket-api/responder"
	"github.com/aidenappl/openbucket-api/sso"
	"github.com/aidenappl/openbucket-api/tools"
)

// HandleAdminGetSSOConfig returns the full SSO configuration (admin only).
func HandleAdminGetSSOConfig(w http.ResponseWriter, r *http.Request) {
	cfg := sso.LoadConfig()

	// Return everything except the client secret
	data := map[string]any{
		"enabled":         cfg.Enabled,
		"client_id":       cfg.ClientID,
		"authorize_url":   cfg.AuthorizeURL,
		"token_url":       cfg.TokenURL,
		"introspect_url":  cfg.IntrospectURL,
		"userinfo_url":    cfg.UserInfoURL,
		"redirect_url":    cfg.RedirectURL,
		"logout_url":      cfg.LogoutURL,
		"scopes":          cfg.Scopes,
		"user_identifier": cfg.UserIdentifier,
		"button_label":    cfg.ButtonLabel,
		"auto_provision":  cfg.AutoProvision,
		"post_login_url":  cfg.PostLoginURL,
		"has_secret":      cfg.ClientSecret != "",
	}

	responder.New(w, data, "SSO configuration retrieved")
}

type UpdateSSOConfigRequest struct {
	Enabled        *bool   `json:"enabled"`
	ClientID       *string `json:"client_id"`
	ClientSecret   *string `json:"client_secret"`
	AuthorizeURL   *string `json:"authorize_url"`
	TokenURL       *string `json:"token_url"`
	UserInfoURL    *string `json:"userinfo_url"`
	IntrospectURL  *string `json:"introspect_url"`
	RedirectURL    *string `json:"redirect_url"`
	LogoutURL      *string `json:"logout_url"`
	Scopes         *string `json:"scopes"`
	UserIdentifier *string `json:"user_identifier"`
	ButtonLabel    *string `json:"button_label"`
	AutoProvision  *bool   `json:"auto_provision"`
	PostLoginURL   *string `json:"post_login_url"`
}

// HandleAdminUpdateSSOConfig updates the SSO configuration (admin only).
func HandleAdminUpdateSSOConfig(w http.ResponseWriter, r *http.Request) {
	var body UpdateSSOConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		responder.SendError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate URLs if provided
	if body.AuthorizeURL != nil && *body.AuthorizeURL != "" {
		if err := tools.ValidateExternalURL(*body.AuthorizeURL); err != nil {
			responder.SendError(w, http.StatusBadRequest, "authorize_url: "+err.Error())
			return
		}
	}
	if body.TokenURL != nil && *body.TokenURL != "" {
		if err := tools.ValidateExternalURL(*body.TokenURL); err != nil {
			responder.SendError(w, http.StatusBadRequest, "token_url: "+err.Error())
			return
		}
	}
	if body.UserInfoURL != nil && *body.UserInfoURL != "" {
		if err := tools.ValidateExternalURL(*body.UserInfoURL); err != nil {
			responder.SendError(w, http.StatusBadRequest, "userinfo_url: "+err.Error())
			return
		}
	}
	// Validated on the same terms as its siblings. This one is a server-to-server
	// POST carrying the user's token and this service's client credentials, so an
	// unvalidated value here would send both to wherever an admin typed.
	if body.IntrospectURL != nil && *body.IntrospectURL != "" {
		if err := tools.ValidateExternalURL(*body.IntrospectURL); err != nil {
			responder.SendError(w, http.StatusBadRequest, "introspect_url: "+err.Error())
			return
		}
	}

	// Apply updates
	if body.Enabled != nil {
		val := "false"
		if *body.Enabled {
			val = "true"
		}
		_ = query.SetSetting(db.DB, "sso.enabled", val)
	}
	if body.ClientID != nil {
		_ = query.SetSetting(db.DB, "sso.client_id", *body.ClientID)
	}
	if body.ClientSecret != nil && *body.ClientSecret != "" {
		encrypted, err := tools.Encrypt(*body.ClientSecret)
		if err != nil {
			responder.SendError(w, http.StatusInternalServerError, "failed to encrypt client secret")
			return
		}
		_ = query.SetSetting(db.DB, "sso.client_secret", encrypted)
	}
	if body.AuthorizeURL != nil {
		_ = query.SetSetting(db.DB, "sso.authorize_url", *body.AuthorizeURL)
	}
	if body.TokenURL != nil {
		_ = query.SetSetting(db.DB, "sso.token_url", *body.TokenURL)
	}
	if body.UserInfoURL != nil {
		_ = query.SetSetting(db.DB, "sso.userinfo_url", *body.UserInfoURL)
	}
	// ⚠️ WITHOUT THIS THE REVOCATION CHECKPOINT CANNOT RUN.
	//
	// LoadConfig has always read sso.introspect_url, but no handler ever wrote it —
	// so it was env-only and unset in production, and Introspect had no endpoint to
	// call. For forta-api the value is https://auth.appleby.cloud/oauth/introspect.
	if body.IntrospectURL != nil {
		_ = query.SetSetting(db.DB, "sso.introspect_url", *body.IntrospectURL)
	}
	if body.RedirectURL != nil {
		_ = query.SetSetting(db.DB, "sso.redirect_url", *body.RedirectURL)
	}
	if body.LogoutURL != nil {
		_ = query.SetSetting(db.DB, "sso.logout_url", *body.LogoutURL)
	}
	if body.Scopes != nil {
		_ = query.SetSetting(db.DB, "sso.scopes", *body.Scopes)
	}
	if body.UserIdentifier != nil {
		_ = query.SetSetting(db.DB, "sso.user_identifier", *body.UserIdentifier)
	}
	if body.ButtonLabel != nil {
		_ = query.SetSetting(db.DB, "sso.button_label", *body.ButtonLabel)
	}
	if body.AutoProvision != nil {
		val := "false"
		if *body.AutoProvision {
			val = "true"
		}
		_ = query.SetSetting(db.DB, "sso.auto_provision", val)
	}
	if body.PostLoginURL != nil {
		_ = query.SetSetting(db.DB, "sso.post_login_url", *body.PostLoginURL)
	}

	responder.New(w, nil, "SSO configuration updated")
}
