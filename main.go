package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/aidenappl/openbucket-api/bootstrap"
	"github.com/aidenappl/openbucket-api/db"
	"github.com/aidenappl/openbucket-api/env"
	"github.com/aidenappl/openbucket-api/middleware"
	"github.com/aidenappl/openbucket-api/routers"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

func maxBodyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 11<<20) // 11MB (10MB file + multipart overhead)
		next.ServeHTTP(w, r)
	})
}

func main() {
	// Load secrets from Keyring
	env.Init()

	// Initialize the database connection
	db.Init()

	// Verify database connectivity
	if err := db.PingDB(db.DB); err != nil {
		log.Fatal("database connect:", err)
	} else {
		fmt.Println("✅ Done")
	}

	// Run database migrations
	db.RunMigrations()

	// Bootstrap admin user (first-run only)
	if err := bootstrap.EnsureAdminUser(db.DB); err != nil {
		log.Printf("Warning: failed to bootstrap admin user: %v", err)
	}

	fmt.Println()
	// Initialize the router
	r := mux.NewRouter()

	// Base API Endpoint
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Welcome to the OpenBucket API!"))
	}).Methods(http.MethodGet)

	// Health Check Endpoint
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods(http.MethodGet)

	// Global middleware
	r.Use(maxBodyMiddleware)
	r.Use(middleware.LoggingMiddleware)
	r.Use(middleware.CSRFMiddleware)

	// ── Auth Endpoints (public) ──────────────────────────────────────────
	r.HandleFunc("/auth/login", routers.HandleLogin).Methods(http.MethodPost)
	r.HandleFunc("/auth/refresh", routers.HandleRefresh).Methods(http.MethodPost)
	r.HandleFunc("/auth/sso/config", routers.HandleSSOConfig).Methods(http.MethodGet)
	r.HandleFunc("/auth/sso/login", routers.HandleSSOLogin).Methods(http.MethodGet)
	r.HandleFunc("/auth/sso/callback", routers.HandleSSOCallback).Methods(http.MethodGet)

	// ── Auth Endpoints (protected) ───────────────────────────────────────
	r.HandleFunc("/auth/self", middleware.Protected(routers.HandleGetSelf)).Methods(http.MethodGet)
	r.HandleFunc("/auth/self", middleware.Protected(routers.HandleUpdateSelf)).Methods(http.MethodPut)
	r.HandleFunc("/auth/logout", middleware.Protected(routers.HandleLogout)).Methods(http.MethodPost)

	// ── Core V1 API (protected) ──────────────────────────────────────────
	core := r.PathPrefix("/core/v1/").Subrouter()
	core.Use(middleware.AuthMiddleware)
	core.Use(middleware.RejectPending)

	core.HandleFunc("/session", routers.HandleCreateSession).Methods(http.MethodPost)
	core.HandleFunc("/session/{id}", routers.HandleDeleteSession).Methods(http.MethodDelete)
	core.HandleFunc("/sessions", routers.HandleListSessions).Methods(http.MethodGet)

	// ── Bucket Operations (protected + session-scoped) ───────────────────
	bucket := core.PathPrefix("/{sessionId}").Subrouter()
	bucket.Use(middleware.SessionMiddleware)

	// Object Operations
	bucket.HandleFunc("/object", routers.HandleUpload).Methods(http.MethodPut)
	bucket.HandleFunc("/object", routers.HandleGetObject).Methods(http.MethodGet)
	bucket.HandleFunc("/object/head", routers.HandleGetObjectHead).Methods(http.MethodGet)
	bucket.HandleFunc("/object/head", routers.HandleGetObjectHead).Methods(http.MethodPost)
	bucket.HandleFunc("/object/acl", routers.HandleGetObjectACL).Methods(http.MethodGet)
	bucket.HandleFunc("/object/acl", routers.HandleModifyObjectACL).Methods(http.MethodPut)
	bucket.HandleFunc("/object/acl", routers.HandleModifyObjectACL).Methods(http.MethodPost)
	bucket.HandleFunc("/object", routers.HandleDeleteObject).Methods(http.MethodDelete)
	bucket.HandleFunc("/objects", routers.HandleListObjects).Methods(http.MethodGet)
	bucket.HandleFunc("/object/presign", routers.HandlePresign).Methods(http.MethodGet)
	bucket.HandleFunc("/object/rename", routers.HandleRenameObject).Methods(http.MethodPut)

	// Folder Operations
	bucket.HandleFunc("/folder", routers.HandleGetFolder).Methods(http.MethodGet)
	bucket.HandleFunc("/folders", routers.HandleListFolders).Methods(http.MethodGet)
	bucket.HandleFunc("/folder", routers.HandleCreateFolder).Methods(http.MethodPost)
	bucket.HandleFunc("/folder", routers.HandleUpdateFolder).Methods(http.MethodPut)
	bucket.HandleFunc("/folder", routers.HandleDeleteFolder).Methods(http.MethodDelete)

	// ── Admin Endpoints (admin role required) ────────────────────────────
	admin := r.PathPrefix("/admin/").Subrouter()
	admin.Use(middleware.AuthMiddleware)
	admin.Use(middleware.RejectPending)

	// User Management
	admin.HandleFunc("/users", middleware.RequireAdmin(routers.HandleAdminListUsers)).Methods(http.MethodGet)
	admin.HandleFunc("/users", middleware.RequireAdmin(routers.HandleAdminCreateUser)).Methods(http.MethodPost)
	admin.HandleFunc("/users/{id}", middleware.RequireAdmin(routers.HandleAdminUpdateUser)).Methods(http.MethodPut)
	admin.HandleFunc("/users/{id}", middleware.RequireAdmin(routers.HandleAdminDeleteUser)).Methods(http.MethodDelete)

	// SSO Configuration
	admin.HandleFunc("/sso-config", middleware.RequireAdmin(routers.HandleAdminGetSSOConfig)).Methods(http.MethodGet)
	admin.HandleFunc("/sso-config", middleware.RequireAdmin(routers.HandleAdminUpdateSSOConfig)).Methods(http.MethodPut)

	// Instance Management
	admin.HandleFunc("/instances", middleware.RequireAdmin(routers.HandleAdminListInstances)).Methods(http.MethodGet)
	admin.HandleFunc("/instances", middleware.RequireAdmin(routers.HandleAdminCreateInstance)).Methods(http.MethodPost)
	admin.HandleFunc("/instances/{id}", middleware.RequireAdmin(routers.HandleAdminUpdateInstance)).Methods(http.MethodPut)
	admin.HandleFunc("/instances/{id}", middleware.RequireAdmin(routers.HandleAdminDeleteInstance)).Methods(http.MethodDelete)

	// Instance Proxy — forwards to openbucket-go admin API
	admin.HandleFunc("/instances/{id}/proxy/{path:.*}", middleware.RequireAdmin(routers.HandleAdminInstanceProxy)).Methods(http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete)

	// ── CORS ─────────────────────────────────────────────────────────────
	corsMiddleware := cors.New(cors.Options{
		AllowedOrigins: []string{
			"https://openbucket.local.appleby.cloud:3010",
			"https://openbucket.appleby.cloud",
		},
		AllowCredentials: true,
		AllowedHeaders:   []string{"Content-Type", "Authorization", "X-Requested-With", "Accept", "X-CSRF-Token"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
	})

	// ── Server ───────────────────────────────────────────────────────────
	server := &http.Server{
		Addr:         ":" + env.Port,
		Handler:      corsMiddleware.Handler(r),
		ReadTimeout:  15 * 1e9,
		WriteTimeout: 60 * 1e9,
		IdleTimeout:  120 * 1e9,
	}

	if env.TLSCert != "" && env.TLSKey != "" {
		log.Printf("✅ OpenBucket API running (HTTPS) on port %s\n", env.Port)
		log.Fatal(server.ListenAndServeTLS(env.TLSCert, env.TLSKey))
	} else {
		log.Printf("✅ OpenBucket API running on port %s\n", env.Port)
		log.Fatal(server.ListenAndServe())
	}
}
