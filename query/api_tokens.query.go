package query

import (
	"database/sql"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/aidenappl/openbucket-api/db"
	"github.com/aidenappl/openbucket-api/structs"
)

var apiTokenColumns = []string{
	"id", "user_id", "name", "token_hash",
	"expires_at", "last_used_at", "active", "updated_at", "inserted_at",
}

type apiTokenScanner interface {
	Scan(dest ...interface{}) error
}

func scanApiToken(row apiTokenScanner) (*structs.ApiToken, error) {
	var t structs.ApiToken
	err := row.Scan(
		&t.ID, &t.UserID, &t.Name, &t.TokenHash,
		&t.ExpiresAt, &t.LastUsedAt, &t.Active, &t.UpdatedAt, &t.InsertedAt,
	)
	return &t, err
}

type ListApiTokensRequest struct {
	UserID *int
	Limit  int
	Offset int
}

func ListApiTokens(engine db.Queryable, req ListApiTokensRequest) ([]structs.ApiToken, error) {
	if req.Limit == 0 || req.Limit > db.MAX_LIMIT {
		req.Limit = db.DEFAULT_LIMIT
	}

	q := sq.Select(apiTokenColumns...).
		From("api_tokens").
		Where(sq.Eq{"active": true}).
		OrderBy("id DESC").
		Limit(uint64(req.Limit)).
		Offset(uint64(req.Offset))

	if req.UserID != nil {
		q = q.Where(sq.Eq{"user_id": *req.UserID})
	}

	query, args, err := q.ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	rows, err := engine.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list api tokens: %w", err)
	}
	defer rows.Close()

	tokens := []structs.ApiToken{}
	for rows.Next() {
		t, err := scanApiToken(rows)
		if err != nil {
			return nil, fmt.Errorf("scan api token: %w", err)
		}
		tokens = append(tokens, *t)
	}
	return tokens, rows.Err()
}

func GetApiTokenByID(engine db.Queryable, id int) (*structs.ApiToken, error) {
	query, args, err := sq.Select(apiTokenColumns...).
		From("api_tokens").
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	t, err := scanApiToken(engine.QueryRow(query, args...))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get api token: %w", err)
	}
	return t, nil
}

// GetApiTokenByHash resolves a token hash to its active, unexpired record.
// Returns (nil, nil) when no usable token matches.
func GetApiTokenByHash(engine db.Queryable, tokenHash string) (*structs.ApiToken, error) {
	query, args, err := sq.Select(apiTokenColumns...).
		From("api_tokens").
		Where(sq.Eq{"token_hash": tokenHash}).
		Where(sq.Eq{"active": true}).
		Where(sq.Or{
			sq.Eq{"expires_at": nil},
			sq.Expr("expires_at > NOW()"),
		}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	t, err := scanApiToken(engine.QueryRow(query, args...))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get api token by hash: %w", err)
	}
	return t, nil
}

type CreateApiTokenRequest struct {
	UserID    int
	Name      string
	TokenHash string
	ExpiresAt *time.Time
}

func CreateApiToken(engine db.Queryable, req CreateApiTokenRequest) (*structs.ApiToken, error) {
	query, args, err := sq.Insert("api_tokens").
		Columns("user_id", "name", "token_hash", "expires_at").
		Values(req.UserID, req.Name, req.TokenHash, req.ExpiresAt).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	result, err := engine.Exec(query, args...)
	if err != nil {
		return nil, fmt.Errorf("create api token: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}

	return GetApiTokenByID(engine, int(id))
}

// RevokeApiToken soft-deletes a token by clearing its active flag. Rows are
// retained so last_used_at stays auditable after revocation.
func RevokeApiToken(engine db.Queryable, id int) error {
	query, args, err := sq.Update("api_tokens").
		Set("active", false).
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build query: %w", err)
	}

	if _, err := engine.Exec(query, args...); err != nil {
		return fmt.Errorf("revoke api token: %w", err)
	}
	return nil
}

// TouchApiToken records that a token was just used for authentication.
func TouchApiToken(engine db.Queryable, id int) error {
	query, args, err := sq.Update("api_tokens").
		Set("last_used_at", time.Now()).
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build query: %w", err)
	}

	if _, err := engine.Exec(query, args...); err != nil {
		return fmt.Errorf("touch api token: %w", err)
	}
	return nil
}
