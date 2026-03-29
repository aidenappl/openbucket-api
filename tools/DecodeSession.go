package tools

// SessionClaims holds the bucket connection details for an active session.
// Credentials are stored in decrypted form only while in memory.
type SessionClaims struct {
	FortaUserID int64
	BucketName  string
	Nickname    string
	Region      string
	Endpoint    string
	AccessKey   *string
	SecretKey   *string
}
