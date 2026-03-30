package tools

// SessionClaims holds the bucket connection details for an active session.
// Credentials are stored in decrypted form only while in memory.
type SessionClaims struct {
	SessionID   int64 // Database session ID (for caching)
	FortaUserID int64
	BucketName  string
	Nickname    string
	Region      string
	Endpoint    string
	AccessKey   *string
	SecretKey   *string
}
