package cache

import (
	"sync"
	"time"

	"github.com/aidenappl/openbucket-api/structs"
	"github.com/aws/aws-sdk-go/aws/session"
)

// Session cache with 5-minute TTL
type sessionCacheEntry struct {
	session   *structs.Session
	expiresAt time.Time
}

var (
	sessionCache   = make(map[int64]*sessionCacheEntry)
	sessionCacheMu sync.RWMutex
	sessionTTL     = 5 * time.Minute
)

// GetSession retrieves a cached session by session ID
func GetSession(sessionID int64) (*structs.Session, bool) {
	sessionCacheMu.RLock()
	entry, ok := sessionCache[sessionID]
	sessionCacheMu.RUnlock()
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.session, true
}

// SetSession caches a session with the configured TTL
func SetSession(sessionID int64, sess *structs.Session) {
	sessionCacheMu.Lock()
	sessionCache[sessionID] = &sessionCacheEntry{
		session:   sess,
		expiresAt: time.Now().Add(sessionTTL),
	}
	sessionCacheMu.Unlock()
}

// InvalidateSession removes a session from the cache
func InvalidateSession(sessionID int64) {
	sessionCacheMu.Lock()
	delete(sessionCache, sessionID)
	sessionCacheMu.Unlock()
}

// AWS Session cache with 5-minute TTL
type awsSessionCacheEntry struct {
	sess      *session.Session
	expiresAt time.Time
}

var (
	awsSessionCache   = make(map[int64]*awsSessionCacheEntry)
	awsSessionCacheMu sync.RWMutex
	awsSessionTTL     = 5 * time.Minute
)

// GetAWSSession retrieves a cached AWS session by session ID
func GetAWSSession(sessionID int64) (*session.Session, bool) {
	awsSessionCacheMu.RLock()
	entry, ok := awsSessionCache[sessionID]
	awsSessionCacheMu.RUnlock()
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.sess, true
}

// SetAWSSession caches an AWS session with the configured TTL
func SetAWSSession(sessionID int64, sess *session.Session) {
	awsSessionCacheMu.Lock()
	awsSessionCache[sessionID] = &awsSessionCacheEntry{
		sess:      sess,
		expiresAt: time.Now().Add(awsSessionTTL),
	}
	awsSessionCacheMu.Unlock()
}

// InvalidateAWSSession removes an AWS session from the cache
func InvalidateAWSSession(sessionID int64) {
	awsSessionCacheMu.Lock()
	delete(awsSessionCache, sessionID)
	awsSessionCacheMu.Unlock()
}

// InvalidateAllForSession removes both session and AWS session from cache
func InvalidateAllForSession(sessionID int64) {
	InvalidateSession(sessionID)
	InvalidateAWSSession(sessionID)
}
