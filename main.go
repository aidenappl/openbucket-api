package main

import (
	"fmt"
	"log"
	"net/http"

	forta "github.com/aidenappl/go-forta"
	"github.com/aidenappl/openbucket-api/db"
	"github.com/aidenappl/openbucket-api/env"
	"github.com/aidenappl/openbucket-api/middleware"
	"github.com/aidenappl/openbucket-api/routers"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

func main() {
	// Load secrets from Keyring (if configured) and read env vars
	env.Init()

	// Initialize Forta authentication
	fmt.Print("Connecting to Forta... ")
	if err := forta.Setup(forta.Config{
		AppDomain:          env.FortaAppDomain,
		APIDomain:          env.FortaAPIDomain,
		LoginDomain:        env.FortaLoginDomain,
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
	} else {
		fmt.Println("✅ Done")
	}

	// Initialize the database connection
	db.Init()

	// Connect to the database
	if err := db.PingDB(db.DB); err != nil {
		log.Fatal("database connect:", err)
	}

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

	// Request logger
	r.Use(middleware.LoggingMiddleware)

	// Forta Authentication Handlers
	r.HandleFunc("/forta/login", forta.LoginHandler).Methods(http.MethodGet)
	r.HandleFunc("/forta/logout", forta.LogoutHandler).Methods(http.MethodGet)

	// Forta User Endpoints
	// Get current user (protected - requires valid Forta session)
	r.HandleFunc("/self", forta.Protected(routers.HandleGetCurrentUser)).Methods(http.MethodGet)

	// Core V1 API Endpoint
	core := r.PathPrefix("/core/v1/").Subrouter()
	core.HandleFunc("/session", forta.Protected(routers.HandleCreateSession)).Methods(http.MethodPost)
	core.HandleFunc("/sessions", forta.Protected(routers.HandleListSessions)).Methods(http.MethodGet)

	// Bucket Operations
	bucket := core.PathPrefix("/{sessionId}").Subrouter()
	bucket.Use(middleware.FortaMiddleware)
	bucket.Use(middleware.SessionMiddleware)

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

	if env.TLSCert != "" && env.TLSKey != "" {
		log.Printf("✅ OpenBucket API running (HTTPS) on port %s\n", env.Port)
		log.Fatal(http.ListenAndServeTLS(":"+env.Port, env.TLSCert, env.TLSKey, corsMiddleware.Handler(r)))
	} else {
		log.Printf("✅ OpenBucket API running on port %s\n", env.Port)
		log.Fatal(http.ListenAndServe(":"+env.Port, corsMiddleware.Handler(r)))
	}
}
