package db

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

type insertIDRunner interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func Rebind(dialect, query string) string {
	switch normalizeDialect(dialect) {
	case "postgres":
		var builder strings.Builder
		builder.Grow(len(query) + 8)
		placeholderIndex := 1
		for _, r := range query {
			if r == '?' {
				builder.WriteByte('$')
				builder.WriteString(strconv.Itoa(placeholderIndex))
				placeholderIndex++
				continue
			}
			builder.WriteRune(r)
		}
		return builder.String()
	default:
		return query
	}
}

func InsertID(ctx context.Context, dialect string, runner insertIDRunner, insertSQL, returningCol string, args ...any) (int64, error) {
	dialect = normalizeDialect(dialect)
	query := strings.TrimSpace(insertSQL)
	query = strings.TrimSuffix(query, ";")

	switch dialect {
	case "postgres":
		returningQuery := fmt.Sprintf("%s RETURNING %s;", query, returningCol)
		var id int64
		if err := runner.QueryRowContext(ctx, Rebind(dialect, returningQuery), args...).Scan(&id); err != nil {
			return 0, err
		}
		return id, nil
	default:
		result, err := runner.ExecContext(ctx, Rebind(dialect, query+";"), args...)
		if err != nil {
			return 0, err
		}
		return result.LastInsertId()
	}
}

func normalizeDialect(dialect string) string {
	dialect = strings.ToLower(strings.TrimSpace(dialect))
	if dialect == "" {
		return "sqlite"
	}
	return dialect
}
