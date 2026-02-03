//go:build !solution

package dao

import (
	"context"
	"database/sql"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type MyDao struct {
	db *sql.DB
}

func CreateDao(ctx context.Context, dsn string) (MyDao, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return MyDao{}, err
	}

	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users (
			ID SERIAL PRIMARY KEY,
			Name VARCHAR(255) NOT NULL
		)
	`)
	if err != nil {
		return MyDao{}, err
	}

	dao := MyDao{db: db}
	return dao, nil
}

func (dao *MyDao) Close() error {
	return dao.db.Close()
}

func (dao *MyDao) Create(ctx context.Context, u *User) (UserID, error) {
	var id UserID
	err := dao.db.QueryRowContext(ctx, `
		INSERT INTO users (Name) VALUES ($1) RETURNING ID
	`, u.Name).Scan(&id)

	if err != nil {
		return 0, err
	}

	return id, nil
}

func (dao *MyDao) Update(ctx context.Context, u *User) error {
	result, err := dao.db.ExecContext(ctx,
		"UPDATE users SET Name = $1 WHERE ID = $2",
		u.Name, u.ID)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (dao *MyDao) Delete(ctx context.Context, id UserID) error {
	result, err := dao.db.ExecContext(ctx,
		"DELETE FROM users WHERE ID = $1",
		id)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (dao *MyDao) Lookup(ctx context.Context, id UserID) (User, error) {
	var user User
	err := dao.db.QueryRowContext(ctx,
		"SELECT ID, Name FROM users WHERE ID = $1",
		id).Scan(&user.ID, &user.Name)
	if err != nil {
		return User{}, err
	}
	return user, nil
}

func (dao *MyDao) List(ctx context.Context) ([]User, error) {
	rows, err := dao.db.QueryContext(ctx,
		"SELECT ID, Name FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Name); err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}
