package routers

import (
	"net/http"

	"github.com/aidenappl/openbucket-api/middleware"
	"github.com/gorilla/mux"
)

// RegisterBucketRoutes mounts the session-scoped object and folder routes on
// r (the `/core/v1/{sessionId}` subrouter).
//
// ⚠️ Every route that changes a bucket goes through middleware.RequireEditor.
// RequireEditor existed for a long time without being applied anywhere, so a
// `viewer` could upload, delete, rename and change ACLs like an admin. Reads
// (including the bulk HEAD, which is a POST) stay open to viewers. Adding a
// mutating route here without RequireEditor reopens that hole —
// TestBucketRoutesEnforceRoles lists every route and fails if you do.
func RegisterBucketRoutes(r *mux.Router) {
	write := middleware.RequireEditor

	// Object operations
	// Uploads and downloads get long deadlines (middleware.LongTransfer).
	long := middleware.LongTransfer
	r.HandleFunc("/object", long(write(HandleUpload))).Methods(http.MethodPut)
	r.HandleFunc("/object", long(HandleGetObject)).Methods(http.MethodGet)
	r.HandleFunc("/object/head", HandleGetObjectHead).Methods(http.MethodGet)
	r.HandleFunc("/object/head", HandleGetObjectHead).Methods(http.MethodPost) // bulk HEAD: a read
	r.HandleFunc("/object/acl", HandleGetObjectACL).Methods(http.MethodGet)
	r.HandleFunc("/object/acl", write(HandleModifyObjectACL)).Methods(http.MethodPut)
	r.HandleFunc("/object/acl", write(HandleModifyObjectACL)).Methods(http.MethodPost) // bulk ACL
	r.HandleFunc("/object", write(HandleDeleteObject)).Methods(http.MethodDelete)
	r.HandleFunc("/objects", HandleListObjects).Methods(http.MethodGet)
	r.HandleFunc("/object/presign", HandlePresign).Methods(http.MethodGet)
	r.HandleFunc("/object/rename", write(HandleRenameObject)).Methods(http.MethodPut)

	// Folder operations
	r.HandleFunc("/folder", HandleGetFolder).Methods(http.MethodGet)
	r.HandleFunc("/folders", HandleListFolders).Methods(http.MethodGet)
	r.HandleFunc("/contents", HandleListContents).Methods(http.MethodGet) // folders + objects, one listing
	r.HandleFunc("/folder", write(HandleCreateFolder)).Methods(http.MethodPost)
	r.HandleFunc("/folder", write(HandleUpdateFolder)).Methods(http.MethodPut)
	r.HandleFunc("/folder", write(HandleDeleteFolder)).Methods(http.MethodDelete)
}
