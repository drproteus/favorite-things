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
	ID          string    `json:"id"`
	OwnerID     string    `json:"ownerId"`
	OwnerName   string    `json:"ownerName"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	Visibility  string    `json:"visibility"`
	Tags        []string  `json:"tags"`
	Images      []Image   `json:"images"`
	AvgRating   float64   `json:"avgRating"`
	RatingCount int       `json:"ratingCount"`
	MyRating    *int      `json:"myRating"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type Image struct {
	ID       string `json:"id"`
	URL      string `json:"url"`
	Position int    `json:"position"`
	key      string
}

type Filter struct {
	OwnerID    string
	Query      string
	Tag        string
	Category   string
	Visibility string
	Limit      int
	Offset     int
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// itemSelect expects $1 to be the viewer's user id (or NULL for anonymous).
const itemSelect = `
SELECT i.id::text, i.owner_id::text, u.name, i.title, i.description, i.category, i.visibility,
       i.created_at, i.updated_at,
       COALESCE((SELECT avg(r.rating)::float8 FROM item_ratings r WHERE r.item_id = i.id), 0),
       (SELECT count(*)::int FROM item_ratings r WHERE r.item_id = i.id),
       (SELECT r.rating::int FROM item_ratings r WHERE r.item_id = i.id AND r.user_id = $1::uuid)
FROM items i
JOIN users u ON u.id = i.owner_id`

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func (s *Store) List(ctx context.Context, viewer string, f Filter) ([]Item, error) {
	args := []any{nullable(viewer)}
	var conds []string

	if f.OwnerID != "" {
		args = append(args, f.OwnerID)
		conds = append(conds, fmt.Sprintf("i.owner_id = $%d::uuid", len(args)))
	} else {
		conds = append(conds, "i.visibility = 'public'")
	}
	if f.Visibility != "" {
		args = append(args, f.Visibility)
		conds = append(conds, fmt.Sprintf("i.visibility = $%d", len(args)))
	}
	if f.Category != "" {
		args = append(args, strings.TrimSpace(f.Category))
		conds = append(conds, fmt.Sprintf("i.category ILIKE $%d", len(args)))
	}
	if f.Query != "" {
		args = append(args, "%"+likeEscaper.Replace(f.Query)+"%")
		n := len(args)
		conds = append(conds, fmt.Sprintf("(i.title ILIKE $%d OR i.description ILIKE $%d)", n, n))
	}
	if f.Tag != "" {
		args = append(args, normalizeTag(f.Tag))
		conds = append(conds, fmt.Sprintf(`EXISTS (
			SELECT 1 FROM item_tags it JOIN tags t ON t.id = it.tag_id
			WHERE it.item_id = i.id AND t.name = $%d)`, len(args)))
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
	if err := s.loadDetails(ctx, list); err != nil {
		return nil, err
	}
	return list, nil
}

// Get returns the item if it is public or owned by viewer; otherwise nil.
func (s *Store) Get(ctx context.Context, viewer, id string) (*Item, error) {
	row := s.pool.QueryRow(ctx,
		itemSelect+` WHERE i.id = $2::uuid AND (i.visibility = 'public' OR i.owner_id = $1::uuid)`,
		nullable(viewer), id)
	it, err := scanItem(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	list := []Item{it}
	if err := s.loadDetails(ctx, list); err != nil {
		return nil, err
	}
	return &list[0], nil
}

func (s *Store) Create(ctx context.Context, ownerID string, in Input) (string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO items (owner_id, title, description, category, visibility)
		VALUES ($1::uuid, $2, $3, $4, $5)
		RETURNING id::text`,
		ownerID, in.Title, in.Description, in.Category, in.Visibility).Scan(&id)
	if err != nil {
		return "", err
	}
	if err := syncTags(ctx, tx, ownerID, id, in.Tags); err != nil {
		return "", err
	}
	return id, tx.Commit(ctx)
}

func (s *Store) Update(ctx context.Context, ownerID, id string, in Input) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		UPDATE items SET title = $3, description = $4, category = $5, visibility = $6, updated_at = now()
		WHERE id = $2::uuid AND owner_id = $1::uuid`,
		ownerID, id, in.Title, in.Description, in.Category, in.Visibility)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	if err := syncTags(ctx, tx, ownerID, id, in.Tags); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
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

// SetRating creates or updates a user's rating. The item must be public or owned by the user.
func (s *Store) SetRating(ctx context.Context, userID, itemID string, rating int) (bool, error) {
	tag, err := s.pool.Exec(ctx, `
		INSERT INTO item_ratings (item_id, user_id, rating)
		SELECT i.id, $2::uuid, $3::smallint FROM items i
		WHERE i.id = $1::uuid AND (i.visibility = 'public' OR i.owner_id = $2::uuid)
		ON CONFLICT (item_id, user_id) DO UPDATE SET rating = EXCLUDED.rating, updated_at = now()`,
		itemID, userID, rating)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (s *Store) DeleteRating(ctx context.Context, userID, itemID string) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM item_ratings WHERE item_id = $1::uuid AND user_id = $2::uuid`, itemID, userID)
	return err
}

func (s *Store) ListTags(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT t.name FROM tags t
		WHERE t.user_id = $1::uuid AND EXISTS (SELECT 1 FROM item_tags it WHERE it.tag_id = t.id)
		ORDER BY t.name`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

func (s *Store) loadDetails(ctx context.Context, list []Item) error {
	if len(list) == 0 {
		return nil
	}
	ids := make([]string, len(list))
	index := make(map[string]int, len(list))
	for i := range list {
		ids[i] = list[i].ID
		index[list[i].ID] = i
		list[i].Tags = []string{}
		list[i].Images = []Image{}
	}

	tagRows, err := s.pool.Query(ctx, `
		SELECT it.item_id::text, t.name FROM item_tags it
		JOIN tags t ON t.id = it.tag_id
		WHERE it.item_id = ANY($1) ORDER BY t.name`, ids)
	if err != nil {
		return err
	}
	for tagRows.Next() {
		var itemID, name string
		if err := tagRows.Scan(&itemID, &name); err != nil {
			tagRows.Close()
			return err
		}
		i := index[itemID]
		list[i].Tags = append(list[i].Tags, name)
	}
	tagRows.Close()
	if err := tagRows.Err(); err != nil {
		return err
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
	err := row.Scan(&it.ID, &it.OwnerID, &it.OwnerName, &it.Title, &it.Description, &it.Category,
		&it.Visibility, &it.CreatedAt, &it.UpdatedAt, &it.AvgRating, &it.RatingCount, &it.MyRating)
	return it, err
}

func syncTags(ctx context.Context, tx pgx.Tx, ownerID, itemID string, tags []string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM item_tags WHERE item_id = $1::uuid`, itemID); err != nil {
		return err
	}
	for _, name := range tags {
		var tagID int64
		err := tx.QueryRow(ctx, `
			INSERT INTO tags (user_id, name) VALUES ($1::uuid, $2)
			ON CONFLICT (user_id, name) DO UPDATE SET name = EXCLUDED.name
			RETURNING id`, ownerID, name).Scan(&tagID)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO item_tags (item_id, tag_id) VALUES ($1::uuid, $2) ON CONFLICT DO NOTHING`,
			itemID, tagID); err != nil {
			return err
		}
	}
	return nil
}

func normalizeTag(t string) string {
	return strings.ToLower(strings.Join(strings.Fields(t), " "))
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
