package sso

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync/atomic"

	oidcsession "github.com/aidenappl/go-oidc-session"
	"github.com/aidenappl/go-oidc-session/sqlstore"
	"github.com/aidenappl/openbucket-api/db"
	"github.com/aidenappl/openbucket-api/env"
	"github.com/aidenappl/openbucket-api/query"
	"github.com/aidenappl/openbucket-api/structs"
	"github.com/aidenappl/openbucket-api/tools"
)

// SessionCookie is the browser's SSO session cookie: opaque, HttpOnly, set on
// and only sent to the API host. It stands for an oidc_sessions row holding the
// provider's tokens, encrypted with the same key as every other secret here.
// The session lives as long as the provider session: Authenticate refreshes the
// provider's tokens when due and ends the session when the provider refuses.
const SessionCookie = "openbucket_session"

// current is the live Manager, rebuilt by Reload when the SSO settings change.
var current atomic.Pointer[oidcsession.Manager]

// Manager returns the live SSO session manager.
func Manager() *oidcsession.Manager {
	if m := current.Load(); m != nil {
		return m
	}
	Reload()
	return current.Load()
}

// Store is the session/state store over the main database (migration 011).
func Store() *sqlstore.Store {
	return &sqlstore.Store{DB: db.DB, Sealer: cryptoSealer{}}
}

// Reload rebuilds the Manager from the current SSO settings.
func Reload() {
	cfg := LoadConfig()
	frontend := cfg.PostLoginRedirectURL()
	store := Store()
	login := loginPageURL(frontend)

	current.Store(&oidcsession.Manager{
		Providers: func(context.Context, string) (*oidcsession.Provider, error) {
			c := LoadConfig()
			if !c.Enabled || c.ClientID == "" {
				return nil, errors.New("sso is not configured")
			}
			return c.Provider(), nil
		},
		DefaultProvider:  ProviderSlug,
		States:           store,
		Sessions:         store,
		Users:            resolver{},
		CookieName:       SessionCookie,
		Insecure:         env.CookieInsecure,
		DefaultReturnURL: frontend,
		LoginPageURL:     login,
		// signed_out=1 stops the login page from signing straight back in
		// silently. Register this exact URL as a logout redirect URI with the
		// provider, or it will not be honoured.
		PostLogoutURL: login + "?signed_out=1",
		Logf: func(_ context.Context, format string, args ...any) {
			log.Printf("SSO: "+format, args...)
		},
	})
}

// loginPageURL is the frontend's /login on the post-login URL's origin.
func loginPageURL(frontend string) string {
	u, err := url.Parse(frontend)
	if err != nil {
		return "/login"
	}
	u.Path, u.RawQuery = "/login", ""
	return u.String()
}

// cryptoSealer encrypts session tokens with tools.Encrypt (AES-GCM).
type cryptoSealer struct{}

func (cryptoSealer) Seal(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	return tools.Encrypt(plaintext)
}

func (cryptoSealer) Open(sealed string) (string, error) {
	if sealed == "" {
		return "", nil
	}
	return tools.Decrypt(sealed)
}

// resolver maps a verified identity onto an OpenBucket user: the stored
// provider subject, then an SSO user with the same email (healing its subject),
// then auto-provisioning a pending user when enabled.
type resolver struct{}

func (resolver) ResolveUser(_ context.Context, p *oidcsession.Provider, id oidcsession.Identity) (int64, error) {
	if id.Email == "" {
		return 0, &oidcsession.LoginError{Code: "sso_no_email"}
	}
	name, picture := "", ""
	if id.Name != nil {
		name = *id.Name
	}
	if id.Picture != nil {
		picture = *id.Picture
	}

	user, err := resolveUser(p, id.Email, id.Subject, name, picture)
	if err != nil {
		return 0, err
	}
	if !user.Active {
		return 0, &oidcsession.LoginError{Code: "sso_account_disabled"}
	}
	if picture != "" {
		if _, err := query.UpdateUser(db.DB, user.ID, query.UpdateUserRequest{ProfileImageURL: &picture}); err != nil {
			log.Printf("SSO: could not update profile image for user %d: %v", user.ID, err)
		}
	}
	return int64(user.ID), nil
}

func (resolver) LinkIdentity(context.Context, *oidcsession.Provider, int64, oidcsession.Identity) error {
	return errors.New("account linking is not supported")
}

func resolveUser(p *oidcsession.Provider, email, subject, name, picture string) (*structs.User, error) {
	if subject != "" {
		user, err := query.GetUserBySSOSubject(db.DB, subject)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("lookup by subject: %w", err)
		}
		if err == nil && user != nil {
			return user, nil
		}
	}

	user, err := query.GetUserByEmailAndAuthType(db.DB, email, "sso")
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("lookup by email: %w", err)
	}
	if err == nil && user != nil {
		// Older rows stored the email as the subject; replace it with the real
		// OIDC sub so the subject lookup finds the user next time.
		if subject != "" && (user.SSOSubject == nil || *user.SSOSubject == "" || LooksLikeEmail(*user.SSOSubject)) {
			if err := query.UpdateUserSSOSubject(db.DB, user.ID, subject); err != nil {
				log.Printf("SSO: failed to heal sso_subject for user %d: %v", user.ID, err)
			}
		}
		return user, nil
	}

	if !p.AutoProvision {
		return nil, &oidcsession.LoginError{Code: "sso_provision_failed"}
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
	created, err := query.CreateUser(db.DB, query.CreateUserRequest{
		Email:           email,
		Name:            namePtr,
		AuthType:        "sso",
		SSOSubject:      ssoSubject,
		ProfileImageURL: picturePtr,
		Role:            "pending",
	})
	if err != nil {
		return nil, fmt.Errorf("auto-provision: %w", err)
	}
	return created, nil
}

// SessionRequest reports whether r carries an SSO session cookie.
func SessionRequest(r *http.Request) bool {
	c, err := r.Cookie(SessionCookie)
	return err == nil && c.Value != ""
}
