// Command admin manages administrator access from the command line.
//
//	go run ./cmd/admin list
//	go run ./cmd/admin grant  <email|user-id>
//	go run ./cmd/admin revoke <email|user-id>
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"favorite-things/backend/internal/config"
	"favorite-things/backend/internal/database"
)

const usage = `usage:
  admin list                    list users and their admin status
  admin grant  <email|user-id>  make a user an administrator
  admin revoke <email|user-id>  remove administrator access`

func main() {
	// godotenv never overrides variables already set, so the first file found wins per key.
	for _, path := range []string{".env", "../.env"} {
		_ = godotenv.Load(path)
	}
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("\n" + usage)
	}

	ctx := context.Background()
	cfg := config.Load()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	// Migrations are idempotent, so this keeps the CLI usable on a fresh database.
	if err := database.Migrate(ctx, pool); err != nil {
		return err
	}

	switch args[0] {
	case "list":
		return list(ctx, pool)
	case "grant", "revoke":
		if len(args) != 2 {
			return fmt.Errorf("usage: admin %s <email|user-id>", args[0])
		}
		return setAdmin(ctx, pool, strings.TrimSpace(args[1]), args[0] == "grant")
	default:
		return errors.New("unknown command " + strconv.Quote(args[0]) + "\n" + usage)
	}
}

func list(ctx context.Context, pool *pgxpool.Pool) error {
	rows, err := pool.Query(ctx, `
		SELECT id::text, COALESCE(email, ''), name, is_admin
		FROM users ORDER BY created_at`)
	if err != nil {
		return err
	}
	defer rows.Close()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tEMAIL\tNAME\tADMIN")
	for rows.Next() {
		var id, email, name string
		var isAdmin bool
		if err := rows.Scan(&id, &email, &name, &isAdmin); err != nil {
			return err
		}
		if email == "" {
			email = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%t\n", id, email, name, isAdmin)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return w.Flush()
}

func setAdmin(ctx context.Context, pool *pgxpool.Pool, target string, grant bool) error {
	where := "lower(email) = lower($1)"
	if _, err := uuid.Parse(target); err == nil {
		where = "id = $1::uuid"
	}

	var id, email, name string
	err := pool.QueryRow(ctx, fmt.Sprintf(`
		UPDATE users SET is_admin = $2, updated_at = now()
		WHERE %s
		RETURNING id::text, COALESCE(email, ''), name`, where), target, grant).
		Scan(&id, &email, &name)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("no user matches %q; have the person sign in once first (or use the user ID from 'admin list')", target)
	}
	if err != nil {
		return err
	}

	action := "granted admin to"
	if !grant {
		action = "revoked admin from"
	}
	fmt.Printf("%s %s (%s) [%s]\n", action, name, email, id)
	return nil
}
