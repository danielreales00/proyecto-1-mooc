// Package dbx expone la mínima superficie común entre un pool de pgx y una
// transacción, para que una operación pueda ejecutarse indistintamente en
// ambos sin que el dominio importe pgx.
package dbx

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}
