package main

import (
	"log"
	"net/http"

	forta "github.com/aidenappl/go-forta"
	"github.com/aidenappl/openbucket-api/env"
	"github.com/aidenappl/openbucket-api/middleware"
	"github.com/aidenappl/openbucket-api/routers"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

func main() {
	// Initialize Forta authentication
	if err := forta.Setup(forta.Config{
		Domain:             env.FortaDomain,
		ClientID:           env.FortaClientID,
		ClientSecret:       env.FortaClientSecret,
		CallbackURL:        env.FortaCallbackURL,
		PostLoginRedirect:  env.FortaPostLoginRedirect,
		PostLogoutRedirect: env.FortaPostLogoutRedirect,
		CookieDomain:       env.FortaCookieDomain,
		CookieInsecure:     env.FortaCookieInsecure,
		JWTSigningKey:      env.FortaJWTSigningKey,
		FetchUserOnProtect: env.FortaFetchUserOnProtect,
		DisableAutoRefresh: env.FortaDisableAutoRefresh,
	}); err != nil {
		log.Fatal("forta setup:", err)
	}

	// Verify Forta API is reachable before accepting traffic
	if err := forta.Ping(); err != nil {
		log.Fatal("forta unreachable:", err)
	}

	log.Println("✅ Forta authentication initialized")

	// Initialize the router
	r := mux.NewRouter()

	// Request logger
	r.Use(middleware.LoggingMiddleware)

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

	// Forta Authentication Handlers
	r.HandleFunc("/forta/login", forta.LoginHandler).Methods(http.MethodGet)
	r.HandleFunc("/forta/callback", forta.CallbackHandler).Methods(http.MethodGet)
	r.HandleFunc("/forta/logout", forta.LogoutHandler).Methods(http.MethodGet)

	// Forta User Endpoints
	// Check auth status (public - no auth required)
	r.HandleFunc("/forta/check", routers.HandleCheckAuth).Methods(http.MethodGet)
	// Get current user (protected - requires valid Forta session)
	r.HandleFunc("/self", forta.Protected(routers.HandleGetCurrentUser)).Methods(http.MethodGet)

	// Token authentication middleware
	r.Use(middleware.TokenAuthMiddleware)

	// Core V1 API Endpoint
	core := r.PathPrefix("/core/v1/").Subrouter()
	core.HandleFunc("/session", routers.HandleCreateSession).Methods(http.MethodPost)
	core.HandleFunc("/sessions", routers.HandleParseSessions).Methods(http.MethodPut)

	// Bucket Operations
	bucket := core.PathPrefix("/{bucket}").Subrouter()

	// -- Object Operations --
	// Put Object
	bucket.HandleFunc("/object", routers.HandleUpload).Methods(http.MethodPut)
	// Get Object
	bucket.HandleFunc("/object", routers.HandleGetObject).Methods(http.MethodGet)
	// Get Object Head
	bucket.HandleFunc("/object/head", routers.HandleGetObjectHead).Methods(http.MethodGet)
	// Get [POST] Object Head (Bulk)
	bucket.HandleFunc("/object/head", routers.HandleGetObjectHead).Methods(http.MethodPost)
	// Get Object ACL
	bucket.HandleFunc("/object/acl", routers.HandleGetObjectACL).Methods(http.MethodGet)
	// Modify Object ACL
	bucket.HandleFunc("/object/acl", routers.HandleModifyObjectACL).Methods(http.MethodPut)
	// Delete Object
	bucket.HandleFunc("/object", routers.HandleDeleteObject).Methods(http.MethodDelete)
	// List Objects
	bucket.HandleFunc("/objects", routers.HandleListObjects).Methods(http.MethodGet)
	// Presign Object
	bucket.HandleFunc("/object/presign", routers.HandlePresign).Methods(http.MethodGet)
	// Rename Object
	bucket.HandleFunc("/object/rename", routers.HandleRenameObject).Methods(http.MethodPut)

	// -- Folder Operations --
	// Get Folder
	bucket.HandleFunc("/folder", routers.HandleGetFolder).Methods(http.MethodGet)
	// List Folders
	bucket.HandleFunc("/folders", routers.HandleListFolders).Methods(http.MethodGet)
	// Create Folder
	bucket.HandleFunc("/folder", routers.HandleCreateFolder).Methods(http.MethodPost)
	// Update Folder
	bucket.HandleFunc("/folder", routers.HandleUpdateFolder).Methods(http.MethodPut)
	// Delete Folder
	bucket.HandleFunc("/folder", routers.HandleDeleteFolder).Methods(http.MethodDelete)

	// Bucket Operations
	// List Buckets
	// Create Bucket
	// Delete Bucket
	// Get Bucket Info
	// List Objects in Bucket
	// Get Object Info

	// CORS Middleware
	corsMiddleware := cors.New(cors.Options{
		AllowedOrigins: []string{
			"https://openbucket.local.appleby.cloud:3010",
			"https://openbucket.appleby.cloud",
		},
		AllowCredentials: true,
		AllowedHeaders:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
	})

	log.Printf("✅ OpenBucket API running on port %s\n", env.Port)
	log.Fatal(http.ListenAndServe(":"+env.Port, corsMiddleware.Handler(r)))
}
