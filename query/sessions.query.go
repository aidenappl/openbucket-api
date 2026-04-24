package query

import (
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/aidenappl/openbucket-api/db"
	"github.com/aidenappl/openbucket-api/structs"
)

var sessionCols = []string{
	"id",
	"user_id",
	"bucket_name",
	"nickname",
	"region",
	"endpoint",
	"access_key",
	"secret_key",
	"inserted_at",
	"updated_at",
}

func scanSession(row interface {
	Scan(dest ...interface{}) error
}) (*structs.Session, error) {
	var s structs.Session
	err := row.Scan(
		&s.ID,
		&s.UserID,
		&s.BucketName,
		&s.Nickname,
		&s.Region,
		&s.Endpoint,
		&s.AccessKey,
		&s.SecretKey,
		&s.InsertedAt,
		&s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// InsertSessionRequest holds the fields required to create a new session.
type InsertSessionRequest struct {
	UserID     int64
	BucketName string
	Nickname   string
	Region     string
	Endpoint   string
	AccessKey  *string
	SecretKey  *string
}

// InsertSession creates a new session row and returns the generated ID.
func InsertSession(engine db.Queryable, req InsertSessionRequest) (int64, error) {
	q := sq.Insert("sessions").
		Columns("user_id", "bucket_name", "nickname", "region", "endpoint", "access_key", "secret_key").
		Values(req.UserID, req.BucketName, req.Nickname, req.Region, req.Endpoint, req.AccessKey, req.SecretKey)

	qStr, args, err := q.ToSql()
	if err != nil {
		return 0, fmt.Errorf("failed to build insert query: %w", err)
	}

	result, err := engine.Exec(qStr, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to insert session: %w", err)
	}
	return result.LastInsertId()
}

// GetSessionByID returns the session with the given primary key.
func GetSessionByID(engine db.Queryable, id int64) (*structs.Session, error) {
	q := sq.Select(sessionCols...).
		From("sessions").
		Where(sq.Eq{"id": id}).
		Limit(1)

	qStr, args, err := q.ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	return scanSession(engine.QueryRow(qStr, args...))
}

// ListSessionsRequest holds filter parameters for ListSessions.
type ListSessionsRequest struct {
	Select *SelectSession
}

// SelectSession provides optional filter fields for ListSessions.
type SelectSession struct {
	IDs    []int64
	UserID int64
}

// GetSessionByUserAndBucket returns the session owned by the given user for the given bucket.
func GetSessionByUserAndBucket(engine db.Queryable, userID int64, bucketName string) (*structs.Session, error) {
	q := sq.Select(sessionCols...).
		From("sessions").
		Where(sq.Eq{"user_id": userID, "bucket_name": bucketName}).
		Limit(1)

	qStr, args, err := q.ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	return scanSession(engine.QueryRow(qStr, args...))
}

// ListSessions returns sessions matching the request filters.
func ListSessions(engine db.Queryable, req ListSessionsRequest) ([]structs.Session, error) {
	q := sq.Select(sessionCols...).From("sessions")

	if req.Select != nil {
		if len(req.Select.IDs) > 0 {
			q = q.Where(sq.Eq{"id": req.Select.IDs})
		}
		if req.Select.UserID != 0 {
			q = q.Where(sq.Eq{"user_id": req.Select.UserID})
		}
	}

	qStr, args, err := q.ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	rows, err := engine.Query(qStr, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	var sessions []structs.Session
	for rows.Next() {
		var s structs.Session
		if err := rows.Scan(
			&s.ID,
			&s.UserID,
			&s.BucketName,
			&s.Nickname,
			&s.Region,
			&s.Endpoint,
			&s.AccessKey,
			&s.SecretKey,
			&s.InsertedAt,
			&s.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}
