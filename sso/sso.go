package sso

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aidenappl/openbucket-api/db"
	"github.com/aidenappl/openbucket-api/env"
	"github.com/aidenappl/openbucket-api/query"
	"github.com/aidenappl/openbucket-api/tools"
)

// SSOConfig holds all SSO configuration values.
type SSOConfig struct {
	Enabled        bool
	ClientID       string
	ClientSecret   string
	AuthorizeURL   string
	TokenURL       string
	UserInfoURL    string
	RedirectURL    string
	LogoutURL      string
	Scopes         string
	UserIdentifier string
	ButtonLabel    string
	AutoProvision  bool
	PostLoginURL   string
}

func or(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// LoadConfig reads SSO configuration from the database.
// Falls back to environment variables if DB settings don't exist.
func LoadConfig() *SSOConfig {
	settings, err := query.GetSettingsByPrefix(db.DB, "sso.")
	if err != nil || len(settings) == 0 {
		return &SSOConfig{
			Enabled:        env.SSOClientID != "" && env.SSOAuthorizeURL != "",
			ClientID:       env.SSOClientID,
			ClientSecret:   env.SSOClientSecret,
			AuthorizeURL:   env.SSOAuthorizeURL,
			TokenURL:       env.SSOTokenURL,
			UserInfoURL:    env.SSOUserInfoURL,
			RedirectURL:    env.SSORedirectURL,
			LogoutURL:      env.SSOLogoutURL,
			Scopes:         env.SSOScopes,
			UserIdentifier: env.SSOUserIdentifier,
			ButtonLabel:    env.SSOButtonLabel,
			AutoProvision:  env.SSOAutoProvision,
			PostLoginURL:   env.SSOPostLoginURL,
		}
	}

	cfg := &SSOConfig{
		Enabled:        settings["sso.enabled"] == "true",
		ClientID:       strings.TrimSpace(settings["sso.client_id"]),
		AuthorizeURL:   strings.TrimSpace(settings["sso.authorize_url"]),
		TokenURL:       strings.TrimSpace(settings["sso.token_url"]),
		UserInfoURL:    strings.TrimSpace(settings["sso.userinfo_url"]),
		RedirectURL:    strings.TrimSpace(settings["sso.redirect_url"]),
		LogoutURL:      strings.TrimSpace(settings["sso.logout_url"]),
		Scopes:         strings.TrimSpace(or(settings["sso.scopes"], "openid email profile")),
		UserIdentifier: strings.TrimSpace(or(settings["sso.user_identifier"], "email")),
		ButtonLabel:    or(settings["sso.button_label"], "Sign in with SSO"),
		AutoProvision:  settings["sso.auto_provision"] != "false",
		PostLoginURL:   strings.TrimSpace(or(settings["sso.post_login_url"], env.SSOPostLoginURL)),
	}

	// Decrypt client secret from DB
	if secret, ok := settings["sso.client_secret"]; ok && secret != "" {
		decrypted, err := tools.Decrypt(secret)
		if err == nil {
			cfg.ClientSecret = decrypted
		} else {
			log.Printf("WARNING: failed to decrypt SSO client secret, SSO may not work: %v", err)
		}
	}

	return cfg
}

// PostLoginRedirectURL returns the URL to redirect users to after SSO login.
func (c *SSOConfig) PostLoginRedirectURL() string {
	if c.PostLoginURL != "" && c.PostLoginURL != "/" {
		return c.PostLoginURL
	}
	if c.RedirectURL != "" {
		if u, err := url.Parse(c.RedirectURL); err == nil {
			u.Path = "/"
			u.RawQuery = ""
			return u.String()
		}
	}
	return "/"
}

func IsConfigured() bool {
	cfg := LoadConfig()
	return cfg.Enabled && cfg.ClientID != "" && cfg.AuthorizeURL != "" && cfg.TokenURL != ""
}

// Config returns the public SSO configuration for the frontend login page.
func Config() map[string]any {
	cfg := LoadConfig()
	if !cfg.Enabled || cfg.ClientID == "" || cfg.AuthorizeURL == "" || cfg.TokenURL == "" {
		return map[string]any{"enabled": false}
	}
	return map[string]any{
		"enabled":      true,
		"button_label": cfg.ButtonLabel,
		"login_url":    "/auth/sso/login",
	}
}

// generateState creates a random state parameter and stores it in the DB.
func generateState() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	state := base64.URLEncoding.EncodeToString(b)

	expiry := time.Now().Add(10 * time.Minute).Format(time.RFC3339)
	_ = query.SetSetting(db.DB, "sso_state:"+state, expiry)

	// Cleanup expired states (best-effort)
	go func() {
		states, _ := query.GetSettingsByPrefix(db.DB, "sso_state:")
		for k, v := range states {
			if t, err := time.Parse(time.RFC3339, v); err == nil && time.Now().After(t) {
				_ = query.DeleteSetting(db.DB, k)
			}
		}
	}()

	return state
}

// ValidateState checks that a state parameter is valid and not expired.
func ValidateState(state string) bool {
	key := "sso_state:" + state
	val, err := query.GetSetting(db.DB, key)
	if err != nil || val == "" {
		return false
	}

	expiry, err := time.Parse(time.RFC3339, val)
	if err != nil || time.Now().After(expiry) {
		_ = query.DeleteSetting(db.DB, key)
		return false
	}

	// Delete immediately to prevent replay
	_ = query.DeleteSetting(db.DB, key)

	return true
}

// LoginHandler redirects the user to the SSO provider's authorization URL.
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	cfg := LoadConfig()
	if !cfg.Enabled || cfg.ClientID == "" || cfg.AuthorizeURL == "" {
		http.Error(w, "SSO not configured", http.StatusNotFound)
		return
	}

	state := generateState()

	params := url.Values{
		"client_id":     {cfg.ClientID},
		"redirect_uri":  {cfg.RedirectURL},
		"response_type": {"code"},
		"scope":         {cfg.Scopes},
		"state":         {state},
	}

	http.Redirect(w, r, cfg.AuthorizeURL+"?"+params.Encode(), http.StatusFound)
}

// TokenResponse from the OAuth2 token endpoint.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
}

// ExchangeCode exchanges an authorization code for tokens.
// Tries three methods in order for maximum provider compatibility.
func ExchangeCode(code string) (*TokenResponse, error) {
	cfg := LoadConfig()

	data := url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {cfg.RedirectURL},
	}

	// 1. JSON body (Forta-style)
	if resp, err := exchangeWithJSON(cfg, code); err == nil {
		return resp, nil
	}
	// 2. Basic auth header (OAuth2 RFC 6749 preferred)
	if resp, err := exchangeWithBasicAuth(cfg, data); err == nil {
		return resp, nil
	}
	// 3. Credentials in POST body (OAuth2 alternative)
	return exchangeWithBodyAuth(cfg, data)
}

func exchangeWithJSON(cfg *SSOConfig, code string) (*TokenResponse, error) {
	payload := map[string]string{
		"client_id":     cfg.ClientID,
		"client_secret": cfg.ClientSecret,
		"code":          code,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", cfg.TokenURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	return doTokenRequest(req)
}

func exchangeWithBasicAuth(cfg *SSOConfig, data url.Values) (*TokenResponse, error) {
	req, err := http.NewRequest("POST", cfg.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(cfg.ClientID, cfg.ClientSecret)

	return doTokenRequest(req)
}

func exchangeWithBodyAuth(cfg *SSOConfig, data url.Values) (*TokenResponse, error) {
	data.Set("client_id", cfg.ClientID)
	data.Set("client_secret", cfg.ClientSecret)

	req, err := http.NewRequest("POST", cfg.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	return doTokenRequest(req)
}

func doTokenRequest(req *http.Request) (*TokenResponse, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	body, _ := io.ReadAll(resp.Body)

	// Standard OAuth2 format
	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err == nil && tokenResp.AccessToken != "" {
		return &tokenResp, nil
	}

	// Forta envelope format: {"success": true, "data": {"authorization": {"access_token": "..."}}}
	var envelope struct {
		Success bool `json:"success"`
		Data    struct {
			Authorization TokenResponse `json:"authorization"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil && envelope.Success && envelope.Data.Authorization.AccessToken != "" {
		return &envelope.Data.Authorization, nil
	}

	// Forta envelope with token at data level
	var envelope2 struct {
		Success bool          `json:"success"`
		Data    TokenResponse `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope2); err == nil && envelope2.Success && envelope2.Data.AccessToken != "" {
		return &envelope2.Data, nil
	}

	maxLen := len(body)
	if maxLen > 200 {
		maxLen = 200
	}
	return nil, fmt.Errorf("unrecognized token response format: %s", string(body[:maxLen]))
}

// FetchUserInfo calls the userinfo endpoint with the access token.
// Handles both flat OIDC-style JSON and Forta envelope format.
func FetchUserInfo(accessToken string) (map[string]any, error) {
	cfg := LoadConfig()
	if cfg.UserInfoURL == "" {
		return nil, fmt.Errorf("SSO userinfo URL not configured")
	}

	req, err := http.NewRequest("GET", cfg.UserInfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("userinfo request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("userinfo endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var userInfo map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("failed to decode userinfo: %w", err)
	}

	// Unwrap Forta envelope
	if _, hasSuccess := userInfo["success"]; hasSuccess {
		if data, ok := userInfo["data"].(map[string]any); ok {
			return data, nil
		}
	}

	return userInfo, nil
}

// GetUserIdentifier extracts the configured identifier field from userinfo.
func GetUserIdentifier(userInfo map[string]any) string {
	cfg := LoadConfig()
	field := cfg.UserIdentifier
	if field == "" {
		field = "email"
	}

	if val, ok := userInfo[field]; ok {
		return fmt.Sprint(val)
	}
	if email, ok := userInfo["email"]; ok {
		return fmt.Sprint(email)
	}
	return ""
}

// GetUserName extracts a display name from userinfo.
func GetUserName(userInfo map[string]any) string {
	if name, ok := userInfo["name"]; ok {
		s := fmt.Sprint(name)
		if s != "" && s != "<nil>" {
			return s
		}
	}
	if given, ok := userInfo["given_name"]; ok {
		s := fmt.Sprint(given)
		if s != "" && s != "<nil>" {
			name := s
			if family, ok := userInfo["family_name"]; ok {
				fs := fmt.Sprint(family)
				if fs != "" && fs != "<nil>" {
					name += " " + fs
				}
			}
			return strings.TrimSpace(name)
		}
	}
	if email := GetUserEmail(userInfo); email != "" {
		if at := strings.Index(email, "@"); at > 0 {
			return email[:at]
		}
	}
	return ""
}

// GetUserPicture extracts a profile image URL from userinfo.
func GetUserPicture(userInfo map[string]any) string {
	for _, field := range []string{"picture", "avatar_url", "photo", "profile_image_url", "image"} {
		if val, ok := userInfo[field]; ok {
			s := fmt.Sprint(val)
			if s != "" && s != "<nil>" && (strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")) {
				return s
			}
		}
	}
	return ""
}

// GetUserEmail extracts email from userinfo.
func GetUserEmail(userInfo map[string]any) string {
	for _, field := range []string{"email", "preferred_username", "upn", "mail"} {
		if val, ok := userInfo[field]; ok {
			s := fmt.Sprint(val)
			if s != "" && s != "<nil>" && strings.Contains(s, "@") {
				return s
			}
		}
	}
	return ""
}
