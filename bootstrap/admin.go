package bootstrap

import (
	"fmt"
	"log"

	"github.com/aidenappl/openbucket-api/db"
	"github.com/aidenappl/openbucket-api/env"
	"github.com/aidenappl/openbucket-api/query"
	"github.com/aidenappl/openbucket-api/tools"
)

// EnsureAdminUser checks if any user exists in the database.
// If no users exist and OB_ADMIN_EMAIL + OB_ADMIN_PASSWORD are set,
// it creates an initial local admin user.
func EnsureAdminUser(engine db.Queryable) error {
	count, err := query.CountUsers(engine)
	if err != nil {
		return fmt.Errorf("failed to count users: %w", err)
	}

	if count > 0 {
		return nil
	}

	if env.AdminEmail == "" || env.AdminPassword == "" {
		log.Println("  No users exist and OB_ADMIN_EMAIL/OB_ADMIN_PASSWORD not set. No admin user created.")
		return nil
	}

	hash, err := tools.HashPassword(env.AdminPassword)
	if err != nil {
		return fmt.Errorf("failed to hash admin password: %w", err)
	}

	_, err = query.CreateUser(engine, query.CreateUserRequest{
		Email:        env.AdminEmail,
		Name:         strPtr("Admin"),
		AuthType:     "local",
		PasswordHash: &hash,
		Role:         "admin",
	})
	if err != nil {
		return fmt.Errorf("failed to create admin user: %w", err)
	}

	fmt.Println()
	fmt.Println("  Bootstrap admin user created successfully.")
	fmt.Printf("  Email: %s\n", env.AdminEmail)
	fmt.Println("  Role:  admin")
	fmt.Println()

	return nil
}

func strPtr(s string) *string {
	return &s
}
