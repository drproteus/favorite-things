package comments

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Comment struct {
	ID              string    `json:"id"`
	ItemID          string    `json:"itemId"`
	AuthorID        string    `json:"authorId"`
	AuthorName      string    `json:"authorName"`
	AuthorAvatarURL string    `json:"authorAvatarUrl"`
	Body            string    `json:"body"`
	Visibility      string    `json:"visibility"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

const commentSelect = `
SELECT c.id::text, c.item_id::text, c.author_id::text, u.name, u.avatar_url,
       c.body, c.visibility, c.created_at, c.updated_at
FROM item_comments c
JOIN users u ON u.id = c.author_id`

// List returns the comments on an item that viewerID may see. Signed-in viewers can
// see signed_in and public comments; anonymous viewers, when comments are eventually
// exposed to them, only see public ones. A comment marked private is visible only to
// its author.
func (s *Store) List(ctx context.Context, itemID, viewerID string) ([]Comment, error) {
	var rows pgx.Rows
	var err error
	if viewerID == "" {
		rows, err = s.pool.Query(ctx, commentSelect+`
			WHERE c.item_id = $1::uuid AND c.visibility = 'public'
			ORDER BY c.created_at, c.id`, itemID)
	} else {
		rows, err = s.pool.Query(ctx, commentSelect+`
			WHERE c.item_id = $1::uuid
			  AND (c.visibility IN ('public', 'signed_in')
			       OR (c.visibility = 'private' AND c.author_id = $2::uuid))
			ORDER BY c.created_at, c.id`, itemID, viewerID)
	}
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Comment, error) { return scanComment(r) })
}

// Create adds a comment by authorID. An empty visibility uses the table default.
func (s *Store) Create(ctx context.Context, itemID, authorID, body, visibility string) (Comment, error) {
	row := s.pool.QueryRow(ctx, `
		WITH created AS (
			INSERT INTO item_comments (item_id, author_id, body, visibility)
			VALUES ($1::uuid, $2::uuid, $3, COALESCE(NULLIF($4, ''), 'signed_in'))
			RETURNING id, item_id, author_id, body, visibility, created_at, updated_at
		)
		SELECT c.id::text, c.item_id::text, c.author_id::text, u.name, u.avatar_url,
		       c.body, c.visibility, c.created_at, c.updated_at
		FROM created c
		JOIN users u ON u.id = c.author_id`, itemID, authorID, body, visibility)
	return scanComment(row)
}

// Update changes a comment's body and optionally its visibility. Only the author
// can update it. An empty visibility keeps the current one.
func (s *Store) Update(ctx context.Context, itemID, commentID, authorID, body, visibility string) (Comment, bool, error) {
	row := s.pool.QueryRow(ctx, `
		WITH changed AS (
			UPDATE item_comments
			SET body = $3,
			    visibility = COALESCE(NULLIF($4, ''), visibility),
			    updated_at = now()
			WHERE id = $1::uuid AND item_id = $2::uuid AND author_id = $5::uuid
			RETURNING id, item_id, author_id, body, visibility, created_at, updated_at
		)
		SELECT c.id::text, c.item_id::text, c.author_id::text, u.name, u.avatar_url,
		       c.body, c.visibility, c.created_at, c.updated_at
		FROM changed c
		JOIN users u ON u.id = c.author_id`, commentID, itemID, body, visibility, authorID)
	comment, err := scanComment(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Comment{}, false, nil
	}
	if err != nil {
		return Comment{}, false, err
	}
	return comment, true, nil
}

func scanComment(row interface{ Scan(dest ...any) error }) (Comment, error) {
	var c Comment
	err := row.Scan(&c.ID, &c.ItemID, &c.AuthorID, &c.AuthorName, &c.AuthorAvatarURL,
		&c.Body, &c.Visibility, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}
