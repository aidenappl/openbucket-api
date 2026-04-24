package routers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"

	"github.com/aidenappl/openbucket-api/db"
	"github.com/aidenappl/openbucket-api/query"
	"github.com/aidenappl/openbucket-api/sso"
	"github.com/aidenappl/openbucket-api/structs"
)

// HandleSSOConfig returns the public SSO configuration for the frontend login page.
func HandleSSOConfig(w http.ResponseWriter, r *http.Request) {
	cfg := sso.Config()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"success":true,"data":%s}`, mustJSON(cfg))
}

// HandleSSOLogin redirects the user to the SSO provider's authorization URL.
func HandleSSOLogin(w http.ResponseWriter, r *http.Request) {
	sso.LoginHandler(w, r)
}

// HandleSSOCallback handles the OAuth2 callback from the SSO provider.
func HandleSSOCallback(w http.ResponseWriter, r *http.Request) {
	cfg := sso.LoadConfig()

	// Check for errors from provider
	if errCode := r.URL.Query().Get("error"); errCode != "" {
		redirectWithError(w, r, cfg, "sso_denied")
		return
	}

	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" || state == "" {
		redirectWithError(w, r, cfg, "sso_missing_params")
		return
	}

	if !sso.ValidateState(state) {
		redirectWithError(w, r, cfg, "sso_state_expired")
		return
	}

	tokenResp, err := sso.ExchangeCode(code)
	if err != nil {
		log.Printf("SSO token exchange failed: %v", err)
		redirectWithError(w, r, cfg, "sso_exchange_failed")
		return
	}

	userInfo, err := sso.FetchUserInfo(tokenResp.AccessToken)
	if err != nil {
		log.Printf("SSO userinfo fetch failed: %v", err)
		redirectWithError(w, r, cfg, "sso_userinfo_failed")
		return
	}

	email := sso.GetUserEmail(userInfo)
	if email == "" {
		redirectWithError(w, r, cfg, "sso_no_email")
		return
	}

	subject := ""
	if sub, ok := userInfo["sub"]; ok {
		subject = fmt.Sprint(sub)
	} else if id, ok := userInfo["id"]; ok {
		subject = fmt.Sprint(id)
	}

	name := sso.GetUserName(userInfo)
	picture := sso.GetUserPicture(userInfo)

	// Resolve or create user
	user := resolveOrCreateSSOUser(cfg, email, subject, name, picture)
	if user == nil {
		redirectWithError(w, r, cfg, "sso_provision_failed")
		return
	}

	if !user.Active {
		redirectWithError(w, r, cfg, "sso_account_disabled")
		return
	}

	// Update profile image on each login
	if picture != "" {
		_, _ = query.UpdateUser(db.DB, user.ID, query.UpdateUserRequest{
			ProfileImageURL: &picture,
		})
	}

	if !setAuthCookies(w, user.ID) {
		return
	}
	http.Redirect(w, r, cfg.PostLoginRedirectURL(), http.StatusFound)
}

func resolveOrCreateSSOUser(cfg *sso.SSOConfig, email, subject, name, picture string) *structs.User {
	// 1. Try by SSO subject
	if subject != "" {
		if user, err := query.GetUserBySSOSubject(db.DB, subject); err == nil && user != nil {
			return user
		}
	}

	// 2. Try by email + auth_type=sso
	if user, err := query.GetUserByEmailAndAuthType(db.DB, email, "sso"); err == nil && user != nil {
		if subject != "" && (user.SSOSubject == nil || *user.SSOSubject == "") {
			_ = query.UpdateUserSSOSubject(db.DB, user.ID, subject)
		}
		return user
	}

	// 3. Auto-provision
	if !cfg.AutoProvision {
		return nil
	}

	var ssoSubject, namePtr, picturePtr *string
	if subject != "" {
		ssoSubject = &subject
	}
	if name != "" {
		namePtr = &name
	}
	if picture != "" {
		picturePtr = &picture
	}

	user, err := query.CreateUser(db.DB, query.CreateUserRequest{
		Email:           email,
		Name:            namePtr,
		AuthType:        "sso",
		SSOSubject:      ssoSubject,
		ProfileImageURL: picturePtr,
		Role:            "pending",
	})
	if err != nil {
		log.Printf("SSO auto-provision failed: %v", err)
		return nil
	}
	return user
}

func redirectWithError(w http.ResponseWriter, r *http.Request, cfg *sso.SSOConfig, errorCode string) {
	redirectURL := cfg.PostLoginRedirectURL()
	u, err := url.Parse(redirectURL)
	if err != nil {
		http.Error(w, "SSO configuration error", http.StatusInternalServerError)
		return
	}
	q := u.Query()
	q.Set("error", errorCode)
	u.RawQuery = q.Encode()
	u.Path = "/login"
	http.Redirect(w, r, u.String(), http.StatusFound)
}

func mustJSON(v map[string]any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}
