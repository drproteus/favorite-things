package items

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Item struct {
	ID           string    `json:"id"`
	OwnerID      string    `json:"ownerId"`
	OwnerName    string    `json:"ownerName"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	CollectionID string    `json:"collectionId"`
	Visibility   string    `json:"visibility"`
	Tags         []string  `json:"tags"`
	Images       []Image   `json:"images"`
	Rating       *int      `json:"rating"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type Image struct {
	ID       string `json:"id"`
	URL      string `json:"url"`
	Position int    `json:"position"`
	key      string
}

type Filter struct {
	OwnerID      string
	CollectionID string
	Query        string
	Tag          string
	Visibility   string
	Limit        int
	Offset       int
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// itemSelect has no viewer-specific columns, so it is safe for any caller.
const itemSelect = `
SELECT i.id::text, i.owner_id::text, u.name, i.title, i.description, i.collection_id::text,
       i.visibility, i.tags, i.rating::int, i.created_at, i.updated_at
FROM items i
JOIN users u ON u.id = i.owner_id`

// publicClause is true when an item is public and no folder above it is private.
const publicClause = `(i.visibility = 'public' AND NOT EXISTS (
	WITH RECURSIVE up AS (
		SELECT id, parent_id, visibility FROM collections WHERE id = i.collection_id
		UNION ALL
		SELECT c.id, c.parent_id, c.visibility FROM collections c JOIN up ON c.id = up.parent_id
	)
	SELECT 1 FROM up WHERE visibility = 'private'))`

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// List returns public items, or all items owned by Filter.OwnerID.
func (s *Store) List(ctx context.Context, f Filter) ([]Item, error) {
	var args []any
	var conds []string
	add := func(format string, v any) {
		args = append(args, v)
		conds = append(conds, fmt.Sprintf(format, len(args)))
	}

	if f.OwnerID != "" {
		add("i.owner_id = $%d::uuid", f.OwnerID)
	} else {
		conds = append(conds, publicClause)
	}
	if f.CollectionID != "" {
		add("i.collection_id = $%d::uuid", f.CollectionID)
	}
	if f.Visibility != "" {
		add("i.visibility = $%d", f.Visibility)
	}
	if f.Query != "" {
		args = append(args, "%"+likeEscaper.Replace(f.Query)+"%")
		n := len(args)
		conds = append(conds, fmt.Sprintf("(i.title ILIKE $%d OR i.description ILIKE $%d)", n, n))
	}
	if f.Tag != "" {
		add("$%d = ANY(i.tags)", normalizeTag(f.Tag))
	}

	args = append(args, f.Limit, f.Offset)
	query := itemSelect + " WHERE " + strings.Join(conds, " AND ") +
		fmt.Sprintf(" ORDER BY i.updated_at DESC, i.id LIMIT $%d OFFSET $%d", len(args)-1, len(args))

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Item, error) { return scanItem(r) })
	if err != nil {
		return nil, err
	}
	if err := s.loadImages(ctx, list); err != nil {
		return nil, err
	}
	return list, nil
}

// Get returns the item if it is public or owned by viewer; otherwise nil.
func (s *Store) Get(ctx context.Context, viewer, id string) (*Item, error) {
	row := s.pool.QueryRow(ctx,
		itemSelect+` WHERE i.id = $2::uuid AND (`+publicClause+` OR i.owner_id = $1::uuid)`,
		nullable(viewer), id)
	it, err := scanItem(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	list := []Item{it}
	if err := s.loadImages(ctx, list); err != nil {
		return nil, err
	}
	return &list[0], nil
}

func (s *Store) Create(ctx context.Context, ownerID string, in Input) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO items (owner_id, collection_id, title, description, visibility, tags)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6::text[])
		RETURNING id::text`,
		ownerID, in.CollectionID, in.Title, in.Description, in.Visibility, in.Tags).Scan(&id)
	return id, err
}

// Update changes the item. An empty CollectionID keeps the item in its current folder.
func (s *Store) Update(ctx context.Context, ownerID, id string, in Input) (bool, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE items SET title = $3, description = $4, visibility = $5, tags = $6::text[],
		                 collection_id = COALESCE($7::uuid, collection_id), updated_at = now()
		WHERE id = $2::uuid AND owner_id = $1::uuid`,
		ownerID, id, in.Title, in.Description, in.Visibility, in.Tags, nullable(in.CollectionID))
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// Delete removes the item and returns its image keys so the caller can clean up object storage.
func (s *Store) Delete(ctx context.Context, ownerID, id string) ([]string, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		SELECT ii.s3_key FROM item_images ii
		JOIN items i ON i.id = ii.item_id
		WHERE i.id = $2::uuid AND i.owner_id = $1::uuid`, ownerID, id)
	if err != nil {
		return nil, false, err
	}
	keys, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, false, err
	}

	tag, err := tx.Exec(ctx, `DELETE FROM items WHERE id = $2::uuid AND owner_id = $1::uuid`, ownerID, id)
	if err != nil {
		return nil, false, err
	}
	if tag.RowsAffected() == 0 {
		return nil, false, nil
	}
	return keys, true, tx.Commit(ctx)
}

// AddImage appends an image unless the item already has max images.
func (s *Store) AddImage(ctx context.Context, itemID, key, contentType string, size int64, max int) (Image, bool, error) {
	var img Image
	err := s.pool.QueryRow(ctx, `
		INSERT INTO item_images (item_id, s3_key, content_type, size_bytes, position)
		SELECT $1::uuid, $2::text, $3::text, $4::bigint, COALESCE(MAX(position) + 1, 0)
		FROM item_images WHERE item_id = $1::uuid
		HAVING COUNT(*) < $5::int
		RETURNING id::text, s3_key, position`,
		itemID, key, contentType, size, max).Scan(&img.ID, &img.key, &img.Position)
	if errors.Is(err, pgx.ErrNoRows) {
		return Image{}, false, nil
	}
	if err != nil {
		return Image{}, false, err
	}
	return img, true, nil
}

func (s *Store) DeleteImage(ctx context.Context, ownerID, itemID, imageID string) (string, bool, error) {
	var key string
	err := s.pool.QueryRow(ctx, `
		DELETE FROM item_images ii USING items i
		WHERE ii.id = $1::uuid AND ii.item_id = $2::uuid
		  AND i.id = ii.item_id AND i.owner_id = $3::uuid
		RETURNING ii.s3_key`, imageID, itemID, ownerID).Scan(&key)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return key, true, nil
}

// SetRating stores the owner's rating for their own item. A nil rating clears it.
func (s *Store) SetRating(ctx context.Context, ownerID, itemID string, rating *int) (bool, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE items SET rating = $3::smallint
		WHERE id = $2::uuid AND owner_id = $1::uuid`, ownerID, itemID, rating)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// ListTags returns every hashtag the user has used on their items, for filtering.
func (s *Store) ListTags(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT tag FROM items, unnest(tags) AS tag
		WHERE owner_id = $1::uuid
		ORDER BY tag`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

func (s *Store) loadImages(ctx context.Context, list []Item) error {
	if len(list) == 0 {
		return nil
	}
	ids := make([]string, len(list))
	index := make(map[string]int, len(list))
	for i := range list {
		ids[i] = list[i].ID
		index[list[i].ID] = i
		list[i].Images = []Image{}
	}

	imgRows, err := s.pool.Query(ctx, `
		SELECT id::text, item_id::text, s3_key, position FROM item_images
		WHERE item_id = ANY($1) ORDER BY position, created_at`, ids)
	if err != nil {
		return err
	}
	for imgRows.Next() {
		var itemID string
		var img Image
		if err := imgRows.Scan(&img.ID, &itemID, &img.key, &img.Position); err != nil {
			imgRows.Close()
			return err
		}
		i := index[itemID]
		list[i].Images = append(list[i].Images, img)
	}
	imgRows.Close()
	return imgRows.Err()
}

func scanItem(row interface{ Scan(dest ...any) error }) (Item, error) {
	var it Item
	err := row.Scan(&it.ID, &it.OwnerID, &it.OwnerName, &it.Title, &it.Description, &it.CollectionID,
		&it.Visibility, &it.Tags, &it.Rating, &it.CreatedAt, &it.UpdatedAt)
	if it.Tags == nil {
		it.Tags = []string{}
	}
	return it, err
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
