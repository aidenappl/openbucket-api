package main

import (
	"log"
	"net/http"

	"github.com/aidenappl/openbucket-api/env"
	"github.com/aidenappl/openbucket-api/middleware"
	"github.com/aidenappl/openbucket-api/routers"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

func main() {
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
		AllowedOrigins:   []string{"*"},
		AllowCredentials: true,
		AllowedHeaders:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
	})

	log.Printf("✅ OpenBucket API running on port %s\n", env.Port)
	log.Fatal(http.ListenAndServe(":"+env.Port, corsMiddleware.Handler(r)))
}
