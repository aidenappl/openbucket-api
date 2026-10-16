package routers

import (
	"net/http"
	"testing"

	"github.com/aidenappl/openbucket-api/structs"
	"github.com/aidenappl/openbucket-api/tools"
)

func TestCheckPasswordChange(t *testing.T) {
	hash, err := tools.HashPassword("correct-horse-1")
	if err != nil {
		t.Fatal(err)
	}
	local := &structs.User{ID: 1, AuthType: "local", PasswordHash: &hash}
	sso := &structs.User{ID: 2, AuthType: "sso"}
	s := func(v string) *string { return &v }

	tests := []struct {
		name    string
		user    *structs.User
		body    UpdateSelfRequest
		status  int
		wantMsg string
	}{
		{"valid change", local, UpdateSelfRequest{Password: s("battery-staple-2"), CurrentPassword: s("correct-horse-1")}, 0, ""},
		{"no current password", local, UpdateSelfRequest{Password: s("battery-staple-2")}, http.StatusBadRequest, "current_password is required to change the password"},
		{"empty current password", local, UpdateSelfRequest{Password: s("battery-staple-2"), CurrentPassword: s("")}, http.StatusBadRequest, "current_password is required to change the password"},
		{"wrong current password", local, UpdateSelfRequest{Password: s("battery-staple-2"), CurrentPassword: s("guess")}, http.StatusBadRequest, "current password is incorrect"},
		{"too short", local, UpdateSelfRequest{Password: s("short"), CurrentPassword: s("correct-horse-1")}, http.StatusBadRequest, ""},
		{"unchanged", local, UpdateSelfRequest{Password: s("correct-horse-1"), CurrentPassword: s("correct-horse-1")}, http.StatusBadRequest, "the new password must differ from the current one"},
		{"sso account", sso, UpdateSelfRequest{Password: s("battery-staple-2"), CurrentPassword: s("anything")}, http.StatusBadRequest, "this account signs in with SSO and has no password to change"},
		{"local account without a hash", &structs.User{ID: 3, AuthType: "local"}, UpdateSelfRequest{Password: s("battery-staple-2"), CurrentPassword: s("x")}, http.StatusBadRequest, "current password is incorrect"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, msg := checkPasswordChange(tt.user, tt.body)
			if status != tt.status {
				t.Fatalf("status = %d (%q), want %d", status, msg, tt.status)
			}
			if tt.wantMsg != "" && msg != tt.wantMsg {
				t.Fatalf("msg = %q, want %q", msg, tt.wantMsg)
			}
		})
	}
}
