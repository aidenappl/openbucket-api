package structs

import "time"

type Instance struct {
	ID         int       `json:"id"`
	Name       string    `json:"name"`
	Endpoint   string    `json:"endpoint"`
	AdminToken string    `json:"-"`
	Active     bool      `json:"active"`
	UpdatedAt  time.Time `json:"updated_at"`
	InsertedAt time.Time `json:"inserted_at"`
}
