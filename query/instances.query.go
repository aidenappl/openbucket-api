package query

import (
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/aidenappl/openbucket-api/db"
	"github.com/aidenappl/openbucket-api/structs"
)

type ListInstancesRequest struct {
	Active *bool
	Limit  int
	Offset int
}

func ListInstances(engine db.Queryable, req ListInstancesRequest) ([]structs.Instance, error) {
	if req.Limit == 0 || req.Limit > db.MAX_LIMIT {
		req.Limit = db.DEFAULT_LIMIT
	}

	q := sq.Select("id", "name", "endpoint", "admin_token", "active", "updated_at", "inserted_at").
		From("instances").
		Limit(uint64(req.Limit)).
		Offset(uint64(req.Offset)).
		OrderBy("id ASC")

	if req.Active != nil {
		q = q.Where(sq.Eq{"active": *req.Active})
	}

	query, args, err := q.ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	rows, err := engine.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list instances: %w", err)
	}
	defer rows.Close()

	var instances []structs.Instance
	for rows.Next() {
		var inst structs.Instance
		if err := rows.Scan(&inst.ID, &inst.Name, &inst.Endpoint, &inst.AdminToken, &inst.Active, &inst.UpdatedAt, &inst.InsertedAt); err != nil {
			return nil, fmt.Errorf("scan instance: %w", err)
		}
		instances = append(instances, inst)
	}
	return instances, nil
}

func GetInstanceByID(engine db.Queryable, id int) (*structs.Instance, error) {
	query, args, err := sq.Select("id", "name", "endpoint", "admin_token", "active", "updated_at", "inserted_at").
		From("instances").
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	var inst structs.Instance
	err = engine.QueryRow(query, args...).Scan(&inst.ID, &inst.Name, &inst.Endpoint, &inst.AdminToken, &inst.Active, &inst.UpdatedAt, &inst.InsertedAt)
	if err != nil {
		return nil, fmt.Errorf("get instance: %w", err)
	}
	return &inst, nil
}

type CreateInstanceRequest struct {
	Name       string
	Endpoint   string
	AdminToken string
}

func CreateInstance(engine db.Queryable, req CreateInstanceRequest) (*structs.Instance, error) {
	query, args, err := sq.Insert("instances").
		Columns("name", "endpoint", "admin_token").
		Values(req.Name, req.Endpoint, req.AdminToken).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	result, err := engine.Exec(query, args...)
	if err != nil {
		return nil, fmt.Errorf("create instance: %w", err)
	}

	id, _ := result.LastInsertId()
	return GetInstanceByID(engine, int(id))
}

type UpdateInstanceRequest struct {
	Name       *string
	Endpoint   *string
	AdminToken *string
	Active     *bool
}

func UpdateInstance(engine db.Queryable, id int, req UpdateInstanceRequest) (*structs.Instance, error) {
	q := sq.Update("instances").Where(sq.Eq{"id": id})

	if req.Name != nil {
		q = q.Set("name", *req.Name)
	}
	if req.Endpoint != nil {
		q = q.Set("endpoint", *req.Endpoint)
	}
	if req.AdminToken != nil {
		q = q.Set("admin_token", *req.AdminToken)
	}
	if req.Active != nil {
		q = q.Set("active", *req.Active)
	}

	query, args, err := q.ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	_, err = engine.Exec(query, args...)
	if err != nil {
		return nil, fmt.Errorf("update instance: %w", err)
	}

	return GetInstanceByID(engine, id)
}

func DeleteInstance(engine db.Queryable, id int) error {
	query, args, err := sq.Delete("instances").Where(sq.Eq{"id": id}).ToSql()
	if err != nil {
		return fmt.Errorf("build query: %w", err)
	}

	_, err = engine.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("delete instance: %w", err)
	}
	return nil
}
