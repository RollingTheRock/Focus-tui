package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// rowScanner abstracts *sql.Row and pgx.Row.
type rowScanner interface {
	Scan(dest ...any) error
}

// rowIter abstracts *sql.Rows and pgx.Rows.
type rowIter interface {
	Next() bool
	Scan(dest ...any) error
	Close() error
	Err() error
}

// pgxRows wraps pgx.Rows to provide a Close() error method.
type pgxRows struct {
	pgx.Rows
}

func (r pgxRows) Close() error {
	r.Rows.Close()
	return nil
}

// toPgQuery converts SQLite-style ? placeholders to PostgreSQL $1, $2, ...
// It does NOT handle ? inside string literals — use with care.
func toPgQuery(query string) string {
	var b strings.Builder
	argIdx := 1
	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			b.WriteString(fmt.Sprintf("$%d", argIdx))
			argIdx++
		} else {
			b.WriteByte(query[i])
		}
	}
	return b.String()
}

// qRow returns a row scanner, using pgx in postgresql mode or sqlite otherwise.
func (s *Store) qRow(query string, args ...any) rowScanner {
	if s.mode == "postgresql" {
		return s.pgPool.QueryRow(context.Background(), toPgQuery(query), args...)
	}
	return s.db.QueryRow(query, args...)
}

// qRows returns a row iterator, using pgx in postgresql mode or sqlite otherwise.
func (s *Store) qRows(query string, args ...any) (rowIter, error) {
	if s.mode == "postgresql" {
		rows, err := s.pgPool.Query(context.Background(), toPgQuery(query), args...)
		if err != nil {
			return nil, err
		}
		return pgxRows{rows}, nil
	}
	return s.db.Query(query, args...)
}

// exec executes a query, using pgx in postgresql mode or sqlite otherwise.
func (s *Store) exec(query string, args ...any) (sql.Result, error) {
	if s.mode == "postgresql" {
		// pgxpool.Exec returns (pgconn.CommandTag, error). We wrap it to return sql.Result.
		tag, err := s.pgPool.Exec(context.Background(), toPgQuery(query), args...)
		if err != nil {
			return nil, err
		}
		return pgxResult{tag: tag}, nil
	}
	return s.db.Exec(query, args...)
}

// pgxResult adapts pgconn.CommandTag to sql.Result.
type pgxResult struct {
	tag interface{ RowsAffected() int64 }
}

func (r pgxResult) LastInsertId() (int64, error) { return 0, fmt.Errorf("not supported") }
func (r pgxResult) RowsAffected() (int64, error) { return r.tag.RowsAffected(), nil }

// insertReturningID executes an INSERT and returns the generated serial ID.
// In postgresql mode the query MUST contain a RETURNING id clause.
func (s *Store) insertReturningID(query string, args ...any) (int64, error) {
	if s.mode == "postgresql" {
		var id int64
		err := s.pgPool.QueryRow(context.Background(), toPgQuery(query), args...).Scan(&id)
		return id, err
	}
	res, err := s.db.Exec(query, args...)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// isNoRows reports whether err is a "no rows" error from either sqlite or pgx.
func isNoRows(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows)
}

// tbl returns the PostgreSQL projection table name when in postgresql mode,
// otherwise the legacy SQLite table name.
func (s *Store) tbl(sqliteName, pgName string) string {
	if s.mode == "postgresql" {
		return pgName
	}
	return sqliteName
}
