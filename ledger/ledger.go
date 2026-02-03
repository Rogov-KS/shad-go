//go:build !solution

package ledger

import (
	"context"
	"database/sql"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type MyLedger struct {
	db *sql.DB
}

func New(ctx context.Context, dsn string) (MyLedger, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return MyLedger{}, err
	}

	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS balances (
			ID VARCHAR(255) PRIMARY KEY,
			Balance BIGINT NOT NULL DEFAULT 0 CHECK (Balance >= 0)
		)
	`)
	if err != nil {
		return MyLedger{}, err
	}

	dao := MyLedger{db: db}
	return dao, nil
}

func (dao *MyLedger) Close() error {
	return dao.db.Close()
}

func (dao *MyLedger) CreateAccount(ctx context.Context, id ID) error {
	_, err := dao.db.ExecContext(ctx, `
		INSERT INTO balances (ID) VALUES ($1)
	`, id)
	return err
}

func (dao *MyLedger) GetBalance(ctx context.Context, id ID) (Money, error) {
	var amount Money
	err := dao.db.QueryRowContext(ctx,
		"SELECT Balance FROM balances WHERE ID = $1",
		id).Scan(&amount)
	if err != nil {
		return 0, err
	}
	return amount, nil
}

func (dao *MyLedger) Deposit(ctx context.Context, id ID, amount Money) error {
	if amount < 0 {
		return ErrNegativeAmount
	}
	result, err := dao.db.ExecContext(ctx, `
		UPDATE balances SET Balance = Balance + $1 WHERE ID = $2
	`, amount, id)
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

func (dao *MyLedger) Withdraw(ctx context.Context, id ID, amount Money) error {
	if amount < 0 {
		return ErrNegativeAmount
	}

	tx, err := dao.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var balance Money
	err = tx.QueryRowContext(ctx, `SELECT Balance FROM balances WHERE ID = $1 FOR UPDATE`, id).Scan(&balance)
	if err != nil {
		if err == sql.ErrNoRows {
			return sql.ErrNoRows
		}
		return err
	}

	if balance < amount {
		return ErrNoMoney
	}

	_, err = tx.ExecContext(ctx, `UPDATE balances SET Balance = Balance - $1 WHERE ID = $2`, amount, id)
	if err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (dao *MyLedger) Transfer(ctx context.Context, from, to ID, amount Money) error {
	if amount < 0 {
		return ErrNegativeAmount
	}

	tx, err := dao.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Блокируем строки в лексикографическом порядке для предотвращения дедлоков
	var firstID, secondID ID
	var firstBalance, secondBalance Money
	var isFromFirst bool

	if from < to {
		firstID, secondID = from, to
		isFromFirst = true
	} else {
		firstID, secondID = to, from
		isFromFirst = false
	}

	err = tx.QueryRowContext(ctx, `SELECT Balance FROM balances WHERE ID = $1 FOR UPDATE`, firstID).Scan(&firstBalance)
	if err != nil {
		return err
	}

	err = tx.QueryRowContext(ctx, `SELECT Balance FROM balances WHERE ID = $1 FOR UPDATE`, secondID).Scan(&secondBalance)
	if err != nil {
		return err
	}

	// Проверяем достаточность средств на счете отправителя
	var fromBalance Money
	if isFromFirst {
		fromBalance = firstBalance
	} else {
		fromBalance = secondBalance
	}

	if fromBalance < amount {
		return ErrNoMoney
	}

	result, err := tx.ExecContext(ctx, `UPDATE balances SET Balance = Balance - $1 WHERE ID = $2`, amount, from)
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

	result, err = tx.ExecContext(ctx, `UPDATE balances SET Balance = Balance + $1 WHERE ID = $2`, amount, to)
	if err != nil {
		return err
	}
	rowsAffected, err = result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	if err = tx.Commit(); err != nil {
		return err
	}
	return nil
}
