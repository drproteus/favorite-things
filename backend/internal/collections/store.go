// Package collections manages each user's folder tree. Every user has one root folder, and
// items and folders nest under it. An item belongs to exactly one folder.
package collections

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const RootName = "My collection"

var (
	ErrNotFound  = errors.New("folder not found")
	ErrRoot      = errors.New("the root folder can't be changed or deleted")
	ErrCycle     = errors.New("a folder can't be moved into itself or one of its subfolders")
	ErrNameTaken = errors.New("a folder with that name already exists here")
)

type Collection struct {
	ID         string    `json:"id"`
	ParentID   *string   `json:"parentId"`
	Name       string    `json:"name"`
	Visibility string    `json:"visibility"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// DeleteMode says what happens to a folder's contents when the folder is deleted.
type DeleteMode string

const (
	DeleteContents DeleteMode = "delete" // remove the items and subfolders too
	MoveContents   DeleteMode = "move"   // move the items and subfolders into the parent
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// RootID returns the user's root folder, creating it the first time it is needed.
func (s *Store) RootID(ctx context.Context, userID string) (string, error) {
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO collections (owner_id, name, visibility)
		VALUES ($1::uuid, $2, 'public')
		ON CONFLICT (owner_id) WHERE parent_id IS NULL DO NOTHING`, userID, RootName); err != nil {
		return "", err
	}
	var id string
	err := s.pool.QueryRow(ctx,
		`SELECT id::text FROM collections WHERE owner_id = $1::uuid AND parent_id IS NULL`, userID).Scan(&id)
	return id, err
}

// List returns every folder the user owns, root first.
func (s *Store) List(ctx context.Context, userID string) ([]Collection, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, parent_id::text, name, visibility, created_at, updated_at
		FROM collections WHERE owner_id = $1::uuid
		ORDER BY parent_id IS NOT NULL, lower(name)`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Collection, error) { return scanCollection(r) })
}

// Get returns the user's folder, or nil if it doesn't exist or belongs to someone else.
func (s *Store) Get(ctx context.Context, userID, id string) (*Collection, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id::text, parent_id::text, name, visibility, created_at, updated_at
		FROM collections WHERE id = $2::uuid AND owner_id = $1::uuid`, userID, id)
	c, err := scanCollection(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Store) Create(ctx context.Context, userID, parentID, name, visibility string) (Collection, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO collections (owner_id, parent_id, name, visibility)
		VALUES ($1::uuid, $2::uuid, $3, $4)
		RETURNING id::text, parent_id::text, name, visibility, created_at, updated_at`,
		userID, parentID, name, visibility)
	c, err := scanCollection(row)
	return c, mapErr(err)
}

// Update changes a folder's name, visibility and parent. The root folder can't be changed.
func (s *Store) Update(ctx context.Context, userID, id, name, visibility, parentID string) (Collection, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Collection{}, err
	}
	defer tx.Rollback(ctx)

	current, err := s.Get(ctx, userID, id)
	if err != nil {
		return Collection{}, err
	}
	if current == nil {
		return Collection{}, ErrNotFound
	}
	if current.ParentID == nil {
		return Collection{}, ErrRoot
	}
	if err := checkParent(ctx, tx, userID, id, parentID); err != nil {
		return Collection{}, err
	}

	row := tx.QueryRow(ctx, `
		UPDATE collections SET name = $3, visibility = $4, parent_id = $5::uuid, updated_at = now()
		WHERE id = $2::uuid AND owner_id = $1::uuid
		RETURNING id::text, parent_id::text, name, visibility, created_at, updated_at`,
		userID, id, name, visibility, parentID)
	c, err := scanCollection(row)
	if err != nil {
		return Collection{}, mapErr(err)
	}
	return c, tx.Commit(ctx)
}

// Delete removes a folder. With DeleteContents it also removes everything inside and returns the
// storage keys of the removed photos. With MoveContents the items and subfolders move to the parent.
func (s *Store) Delete(ctx context.Context, userID, id string, mode DeleteMode) ([]string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	current, err := s.Get(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, ErrNotFound
	}
	if current.ParentID == nil {
		return nil, ErrRoot
	}
	parentID := *current.ParentID

	var keys []string
	switch mode {
	case MoveContents:
		if _, err := tx.Exec(ctx, `UPDATE items SET collection_id = $2::uuid WHERE collection_id = $1::uuid`, id, parentID); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `UPDATE collections SET parent_id = $2::uuid WHERE parent_id = $1::uuid`, id, parentID); err != nil {
			return nil, mapErr(err)
		}
	default:
		rows, err := tx.Query(ctx, subtreeCTE+`
			SELECT ii.s3_key FROM item_images ii
			JOIN items i ON i.id = ii.item_id
			WHERE i.collection_id IN (SELECT id FROM subtree)`, id)
		if err != nil {
			return nil, err
		}
		if keys, err = pgx.CollectRows(rows, pgx.RowTo[string]); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, subtreeCTE+`
			DELETE FROM items WHERE collection_id IN (SELECT id FROM subtree)`, id); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, subtreeCTE+`
			DELETE FROM collections WHERE id IN (SELECT id FROM subtree)`, id); err != nil {
			return nil, err
		}
	}

	if mode == MoveContents {
		if _, err := tx.Exec(ctx, `DELETE FROM collections WHERE id = $1::uuid`, id); err != nil {
			return nil, err
		}
	}
	return keys, tx.Commit(ctx)
}

// subtreeCTE names the folder given as $1 and every folder beneath it.
const subtreeCTE = `
WITH RECURSIVE subtree AS (
	SELECT id FROM collections WHERE id = $1::uuid
	UNION ALL
	SELECT c.id FROM collections c JOIN subtree s ON c.parent_id = s.id
) `

// checkParent makes sure parentID is one of the user's folders and is not inside the folder being moved.
func checkParent(ctx context.Context, tx pgx.Tx, userID, id, parentID string) error {
	var ok bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM collections WHERE id = $2::uuid AND owner_id = $1::uuid)`,
		userID, parentID).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	var inside bool
	if err := tx.QueryRow(ctx, subtreeCTE+`SELECT EXISTS(SELECT 1 FROM subtree WHERE id = $2::uuid)`,
		id, parentID).Scan(&inside); err != nil {
		return err
	}
	if inside {
		return ErrCycle
	}
	return nil
}

func mapErr(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrNameTaken
	}
	return err
}

func scanCollection(row interface{ Scan(dest ...any) error }) (Collection, error) {
	var c Collection
	err := row.Scan(&c.ID, &c.ParentID, &c.Name, &c.Visibility, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}
