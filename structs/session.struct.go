package structs

import "time"

type Session struct {
	ID          int64     `json:"id"`
	FortaUserID int64     `json:"forta_user_id"`
	BucketName  string    `json:"bucket"`
	Nickname    string    `json:"nickname"`
	Region      string    `json:"region"`
	Endpoint    string    `json:"endpoint"`
	AccessKey   *string   `json:"-"`
	SecretKey   *string   `json:"-"`
	InsertedAt  time.Time `json:"inserted_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type PublicSession struct {
	ID         int64     `json:"id"`
	BucketName string    `json:"bucket"`
	Nickname   string    `json:"nickname"`
	Region     string    `json:"region"`
	Endpoint   string    `json:"endpoint"`
	InsertedAt time.Time `json:"inserted_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (s Session) ToPublic() PublicSession {
	return PublicSession{
		ID:         s.ID,
		BucketName: s.BucketName,
		Nickname:   s.Nickname,
		Region:     s.Region,
		Endpoint:   s.Endpoint,
		InsertedAt: s.InsertedAt,
		UpdatedAt:  s.UpdatedAt,
	}
}
