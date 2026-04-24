package routers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/aidenappl/openbucket-api/db"
	"github.com/aidenappl/openbucket-api/query"
	"github.com/aidenappl/openbucket-api/responder"
	"github.com/aidenappl/openbucket-api/tools"
	"github.com/gorilla/mux"
)

// HandleAdminListInstances returns all registered openbucket-go instances.
func HandleAdminListInstances(w http.ResponseWriter, r *http.Request) {
	req := query.ListInstancesRequest{}

	if activeStr := r.URL.Query().Get("active"); activeStr != "" {
		active := activeStr == "true"
		req.Active = &active
	}

	instances, err := query.ListInstances(db.DB, req)
	if err != nil {
		responder.SendError(w, http.StatusInternalServerError, "failed to list instances", err)
		return
	}

	responder.New(w, instances)
}

type CreateInstanceBody struct {
	Name       string `json:"name"`
	Endpoint   string `json:"endpoint"`
	AdminToken string `json:"admin_token"`
}

// HandleAdminCreateInstance registers a new openbucket-go instance.
func HandleAdminCreateInstance(w http.ResponseWriter, r *http.Request) {
	var body CreateInstanceBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		responder.SendError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.Name == "" || body.Endpoint == "" || body.AdminToken == "" {
		responder.SendError(w, http.StatusBadRequest, "name, endpoint, and admin_token are required")
		return
	}

	if err := tools.ValidateExternalURL(body.Endpoint); err != nil {
		responder.SendError(w, http.StatusBadRequest, err.Error())
		return
	}

	inst, err := query.CreateInstance(db.DB, query.CreateInstanceRequest{
		Name:       body.Name,
		Endpoint:   body.Endpoint,
		AdminToken: body.AdminToken,
	})
	if err != nil {
		responder.SendError(w, http.StatusInternalServerError, "failed to create instance", err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	responder.New(w, inst, "instance created")
}

type UpdateInstanceBody struct {
	Name       *string `json:"name"`
	Endpoint   *string `json:"endpoint"`
	AdminToken *string `json:"admin_token"`
	Active     *bool   `json:"active"`
}

// HandleAdminUpdateInstance updates an existing instance.
func HandleAdminUpdateInstance(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		responder.SendError(w, http.StatusBadRequest, "invalid instance ID")
		return
	}

	var body UpdateInstanceBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		responder.SendError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.Endpoint != nil {
		if err := tools.ValidateExternalURL(*body.Endpoint); err != nil {
			responder.SendError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	inst, err := query.UpdateInstance(db.DB, id, query.UpdateInstanceRequest{
		Name:       body.Name,
		Endpoint:   body.Endpoint,
		AdminToken: body.AdminToken,
		Active:     body.Active,
	})
	if err != nil {
		responder.SendError(w, http.StatusInternalServerError, "failed to update instance", err)
		return
	}

	responder.New(w, inst, "instance updated")
}

// HandleAdminDeleteInstance removes an instance.
func HandleAdminDeleteInstance(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		responder.SendError(w, http.StatusBadRequest, "invalid instance ID")
		return
	}

	if err := query.DeleteInstance(db.DB, id); err != nil {
		responder.SendError(w, http.StatusInternalServerError, "failed to delete instance", err)
		return
	}

	responder.New(w, nil, "instance deleted")
}
