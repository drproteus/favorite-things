package server_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"favorite-things/backend/internal/auth"
	"favorite-things/backend/internal/collections"
	"favorite-things/backend/internal/comments"
	"favorite-things/backend/internal/config"
	"favorite-things/backend/internal/database"
	"favorite-things/backend/internal/items"
	"favorite-things/backend/internal/server"
	"favorite-things/backend/internal/storage"
	"favorite-things/backend/internal/suggest"
)

// These tests run the whole HTTP stack against a real database. They need TEST_DATABASE_URL, for example
//
//	TEST_DATABASE_URL=postgres://favs:favs@localhost:5433/favorites_test?sslmode=disable go test ./internal/server
//
// and are skipped when it isn't set. Every test creates its own users and removes them afterwards.
// Users are isolated: nobody can see or change another user's items or folders.

type fixture struct {
	t    *testing.T
	pool *pgxpool.Pool
	url  string
}

type account struct {
	id    string
	token string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	return newFixtureWith(t, suggest.NewHandler(false, nil, 0))
}

func newFixtureWith(t *testing.T, suggestH *suggest.Handler) *fixture {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run the access tests")
	}
	ctx := context.Background()
	pool, err := database.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}

	cfg := config.Load()
	files := storage.New(cfg)
	authSvc := auth.NewService(pool, time.Hour, false)
	itemStore := items.NewStore(pool)
	folderStore := collections.NewStore(pool)
	handler := server.New(
		cfg,
		authSvc,
		auth.NewHandler(authSvc, cfg),
		items.NewHandler(itemStore, files, folderStore),
		collections.NewHandler(folderStore, files),
		comments.NewHandler(comments.NewStore(pool), itemStore),
		suggestH,
	)
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &fixture{t: t, pool: pool, url: srv.URL}
}

// newUser creates a user with a live session. The user and everything they own are removed at test end.
func (f *fixture) newUser(name string) *account {
	f.t.Helper()
	ctx := context.Background()
	id := uuid.New()
	token := uuid.NewString()
	sum := sha256.Sum256([]byte(token))
	if _, err := f.pool.Exec(ctx, `INSERT INTO users (id, email, name) VALUES ($1, $2, $3)`,
		id, id.String()+"@test.invalid", name); err != nil {
		f.t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ($1, $2, now() + interval '1 hour')`,
		hex.EncodeToString(sum[:]), id); err != nil {
		f.t.Fatal(err)
	}
	f.t.Cleanup(func() {
		if _, err := f.pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id); err != nil {
			f.t.Errorf("cleanup user %s: %v", name, err)
		}
	})
	return &account{id: id.String(), token: token}
}

func (f *fixture) do(a *account, method, path string, body any) (int, []byte) {
	f.t.Helper()
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			f.t.Fatal(err)
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, f.url+path, reader)
	if err != nil {
		f.t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if a != nil {
		req.Header.Set("Cookie", "fv_session="+a.token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		f.t.Fatal(err)
	}
	return resp.StatusCode, out
}

func decodeMap(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("decode %s: %v", b, err)
	}
	return m
}

func decodeList(t *testing.T, b []byte) []map[string]any {
	t.Helper()
	var l []map[string]any
	if err := json.Unmarshal(b, &l); err != nil {
		t.Fatalf("decode %s: %v", b, err)
	}
	return l
}

func (f *fixture) root(a *account) string {
	f.t.Helper()
	status, b := f.do(a, http.MethodGet, "/api/collections", nil)
	if status != http.StatusOK {
		f.t.Fatalf("list folders: %d %s", status, b)
	}
	for _, c := range decodeList(f.t, b) {
		if c["parentId"] == nil {
			return c["id"].(string)
		}
	}
	f.t.Fatal("no root folder")
	return ""
}

func (f *fixture) folder(a *account, name, parent string) string {
	f.t.Helper()
	body := map[string]any{"name": name, "visibility": "private"}
	if parent != "" {
		body["parentId"] = parent
	}
	status, b := f.do(a, http.MethodPost, "/api/collections", body)
	if status != http.StatusCreated {
		f.t.Fatalf("create folder %s: %d %s", name, status, b)
	}
	return decodeMap(f.t, b)["id"].(string)
}

func (f *fixture) item(a *account, title, folder, visibility string) string {
	f.t.Helper()
	body := map[string]any{"title": title, "visibility": visibility}
	if folder != "" {
		body["collectionId"] = folder
	}
	status, b := f.do(a, http.MethodPost, "/api/items", body)
	if status != http.StatusCreated {
		f.t.Fatalf("create item %s: %d %s", title, status, b)
	}
	return decodeMap(f.t, b)["id"].(string)
}

// itemState returns the item as its owner sees it.
func (f *fixture) itemState(a *account, id string) map[string]any {
	f.t.Helper()
	status, b := f.do(a, http.MethodGet, "/api/items/"+id, nil)
	if status != http.StatusOK {
		f.t.Fatalf("get item %s: %d %s", id, status, b)
	}
	return decodeMap(f.t, b)
}

// folderState returns the named folder from the owner's folder list.
func (f *fixture) folderState(a *account, id string) map[string]any {
	f.t.Helper()
	status, b := f.do(a, http.MethodGet, "/api/collections", nil)
	if status != http.StatusOK {
		f.t.Fatalf("list folders: %d %s", status, b)
	}
	for _, c := range decodeList(f.t, b) {
		if c["id"] == id {
			return c
		}
	}
	f.t.Fatalf("folder %s not found for owner", id)
	return nil
}

func (f *fixture) itemExists(a *account, id string) bool {
	status, _ := f.do(a, http.MethodGet, "/api/items/"+id, nil)
	return status == http.StatusOK
}

func (f *fixture) folderExists(a *account, id string) bool {
	status, b := f.do(a, http.MethodGet, "/api/collections", nil)
	if status != http.StatusOK {
		f.t.Fatalf("list folders: %d %s", status, b)
	}
	for _, c := range decodeList(f.t, b) {
		if c["id"] == id {
			return true
		}
	}
	return false
}

func expectRejected(t *testing.T, what string, status int, body []byte) {
	t.Helper()
	if status < 400 || status >= 500 {
		t.Errorf("%s: expected a 4xx rejection, got %d %s", what, status, body)
	}
}

func TestIndividualMovesRejectOtherUsersFolders(t *testing.T) {
	f := newFixture(t)
	alice := f.newUser("Alice")
	bob := f.newUser("Bob")

	aliceFolder := f.folder(alice, "Alice shelf", f.root(alice))
	bobFolder := f.folder(bob, "Bob shelf", f.root(bob))
	bobItem := f.item(bob, "Bob item", bobFolder, "private")
	aliceItem := f.item(alice, "Alice item", aliceFolder, "private")

	t.Run("create item in someone else's folder", func(t *testing.T) {
		status, b := f.do(bob, http.MethodPost, "/api/items", map[string]any{
			"title": "sneaky", "visibility": "private", "collectionId": aliceFolder,
		})
		expectRejected(t, "create item", status, b)
	})

	t.Run("create folder inside someone else's folder", func(t *testing.T) {
		status, b := f.do(bob, http.MethodPost, "/api/collections", map[string]any{
			"name": "sneaky", "visibility": "private", "parentId": aliceFolder,
		})
		expectRejected(t, "create folder", status, b)
	})

	t.Run("owner moves own item into someone else's folder", func(t *testing.T) {
		status, b := f.do(bob, http.MethodPut, "/api/items/"+bobItem, map[string]any{
			"title": "Bob item", "visibility": "private", "collectionId": aliceFolder,
		})
		expectRejected(t, "update item", status, b)
		if got := f.itemState(bob, bobItem)["collectionId"]; got != bobFolder {
			t.Errorf("item moved despite rejection: now in %v", got)
		}
	})

	t.Run("another user edits someone's item to move it", func(t *testing.T) {
		status, b := f.do(alice, http.MethodPut, "/api/items/"+bobItem, map[string]any{
			"title": "hijacked", "visibility": "public", "collectionId": aliceFolder,
		})
		expectRejected(t, "update someone else's item", status, b)
		state := f.itemState(bob, bobItem)
		if state["collectionId"] != bobFolder || state["title"] != "Bob item" {
			t.Errorf("item changed by another user: %v", state)
		}
	})

	t.Run("move someone's folder into another user's folder", func(t *testing.T) {
		status, b := f.do(bob, http.MethodPut, "/api/collections/"+bobFolder, map[string]any{
			"name": "Bob shelf", "visibility": "private", "parentId": aliceFolder,
		})
		expectRejected(t, "move folder", status, b)
		if parent := f.folderState(bob, bobFolder)["parentId"]; parent != f.root(bob) {
			t.Errorf("folder moved despite rejection: parent %v", parent)
		}
	})

	t.Run("another user moves someone's folder", func(t *testing.T) {
		status, b := f.do(alice, http.MethodPut, "/api/collections/"+bobFolder, map[string]any{
			"name": "taken", "visibility": "public", "parentId": aliceFolder,
		})
		expectRejected(t, "update someone else's folder", status, b)
		if state := f.folderState(bob, bobFolder); state["name"] != "Bob shelf" {
			t.Errorf("folder renamed by another user: %v", state)
		}
	})

	t.Run("bulk move items into someone else's folder", func(t *testing.T) {
		status, b := f.do(bob, http.MethodPost, "/api/items/bulk/move", map[string]any{
			"ids": []string{bobItem}, "collectionId": aliceFolder,
		})
		expectRejected(t, "bulk move items", status, b)
		if got := f.itemState(bob, bobItem)["collectionId"]; got != bobFolder {
			t.Errorf("item moved despite rejection: now in %v", got)
		}
	})

	t.Run("bulk move someone else's items into my folder changes nothing", func(t *testing.T) {
		status, b := f.do(bob, http.MethodPost, "/api/items/bulk/move", map[string]any{
			"ids": []string{aliceItem}, "collectionId": bobFolder,
		})
		if status != http.StatusNoContent {
			t.Fatalf("bulk move: %d %s", status, b)
		}
		if got := f.itemState(alice, aliceItem)["collectionId"]; got != aliceFolder {
			t.Errorf("another user moved alice's item into %v", got)
		}
	})

	t.Run("bulk move someone else's folder into mine changes nothing", func(t *testing.T) {
		status, b := f.do(bob, http.MethodPost, "/api/collections/bulk/move", map[string]any{
			"ids": []string{aliceFolder}, "parentId": bobFolder,
		})
		if status != http.StatusNoContent {
			t.Fatalf("bulk move folders: %d %s", status, b)
		}
		if parent := f.folderState(alice, aliceFolder)["parentId"]; parent != f.root(alice) {
			t.Errorf("another user moved alice's folder under %v", parent)
		}
	})

	t.Run("bulk move my folder into someone else's folder", func(t *testing.T) {
		status, b := f.do(bob, http.MethodPost, "/api/collections/bulk/move", map[string]any{
			"ids": []string{bobFolder}, "parentId": aliceFolder,
		})
		expectRejected(t, "bulk move folder", status, b)
	})

	t.Run("bulk delete someone else's items and folders changes nothing", func(t *testing.T) {
		status, b := f.do(bob, http.MethodPost, "/api/items/bulk/delete", map[string]any{"ids": []string{aliceItem}})
		if status != http.StatusNoContent {
			t.Fatalf("bulk delete items: %d %s", status, b)
		}
		status, b = f.do(bob, http.MethodPost, "/api/collections/bulk/delete", map[string]any{"ids": []string{aliceFolder}})
		if status != http.StatusNoContent {
			t.Fatalf("bulk delete folders: %d %s", status, b)
		}
		if !f.itemExists(alice, aliceItem) || !f.folderExists(alice, aliceFolder) {
			t.Error("another user deleted alice's data")
		}
	})

	t.Run("bulk visibility on someone else's items and folders changes nothing", func(t *testing.T) {
		status, b := f.do(bob, http.MethodPost, "/api/items/bulk/visibility", map[string]any{
			"ids": []string{aliceItem}, "visibility": "public",
		})
		if status != http.StatusNoContent {
			t.Fatalf("bulk visibility items: %d %s", status, b)
		}
		status, b = f.do(bob, http.MethodPost, "/api/collections/bulk/visibility", map[string]any{
			"ids": []string{aliceFolder}, "visibility": "public", "recursive": true,
		})
		if status != http.StatusNoContent {
			t.Fatalf("bulk visibility folders: %d %s", status, b)
		}
		if got := f.itemState(alice, aliceItem)["visibility"]; got != "private" {
			t.Errorf("another user changed alice's item visibility to %v", got)
		}
		if got := f.folderState(alice, aliceFolder)["visibility"]; got != "private" {
			t.Errorf("another user changed alice's folder visibility to %v", got)
		}
	})

	t.Run("private items and folders are invisible to other users", func(t *testing.T) {
		if status, _ := f.do(bob, http.MethodGet, "/api/items/"+aliceItem, nil); status != http.StatusNotFound {
			t.Errorf("bob read alice's private item: status %d", status)
		}
		if status, _ := f.do(bob, http.MethodGet, "/api/items?scope=mine&collection="+aliceFolder, nil); status != http.StatusBadRequest {
			t.Errorf("bob listed alice's folder: status %d", status)
		}
		status, b := f.do(bob, http.MethodGet, "/api/collections", nil)
		if status != http.StatusOK {
			t.Fatalf("list folders: %d %s", status, b)
		}
		for _, c := range decodeList(t, b) {
			if c["id"] == aliceFolder {
				t.Error("bob's folder list includes alice's folder")
			}
		}
	})
}

func TestBulkMovesWithinOwnFoldersStillWork(t *testing.T) {
	f := newFixture(t)
	alice := f.newUser("Alice")
	a := f.folder(alice, "A", f.root(alice))
	b := f.folder(alice, "B", f.root(alice))
	i := f.item(alice, "item", a, "private")

	status, body := f.do(alice, http.MethodPost, "/api/items/bulk/move", map[string]any{"ids": []string{i}, "collectionId": b})
	if status != http.StatusNoContent {
		t.Fatalf("bulk move: %d %s", status, body)
	}
	if got := f.itemState(alice, i)["collectionId"]; got != b {
		t.Errorf("item not moved to B: in %v", got)
	}
	status, body = f.do(alice, http.MethodPost, "/api/collections/bulk/move", map[string]any{"ids": []string{b}, "parentId": a})
	if status != http.StatusNoContent {
		t.Fatalf("bulk move folder: %d %s", status, body)
	}
	if parent := f.folderState(alice, b)["parentId"]; parent != a {
		t.Errorf("folder B not moved under A: parent %v", parent)
	}
}

func TestRecursiveFolderVisibility(t *testing.T) {
	f := newFixture(t)
	alice := f.newUser("Alice")
	bob := f.newUser("Bob")

	outer := f.folder(alice, "Outer", f.root(alice))
	inner := f.folder(alice, "Inner", outer)
	innerItem := f.item(alice, "inner item", inner, "public")
	outerItem := f.item(alice, "outer item", outer, "private")

	// Making only the outer folder public must not change the folder inside it.
	status, body := f.do(alice, http.MethodPut, "/api/collections/"+outer, map[string]any{
		"name": "Outer", "visibility": "public", "parentId": f.root(alice), "recursive": false,
	})
	if status != http.StatusOK {
		t.Fatalf("update outer: %d %s", status, body)
	}
	if got := f.folderState(alice, inner)["visibility"]; got != "private" {
		t.Errorf("non-recursive change also changed the subfolder: %v", got)
	}
	if status, _ := f.do(bob, http.MethodGet, "/api/items/"+innerItem, nil); status != http.StatusNotFound {
		t.Errorf("public item inside a private folder is visible to others: %d", status)
	}

	// Recursive makes the subfolders and every item inside them match.
	status, body = f.do(alice, http.MethodPut, "/api/collections/"+outer, map[string]any{
		"name": "Outer", "visibility": "public", "parentId": f.root(alice), "recursive": true,
	})
	if status != http.StatusOK {
		t.Fatalf("recursive update: %d %s", status, body)
	}
	if got := f.folderState(alice, inner)["visibility"]; got != "public" {
		t.Errorf("recursive change did not reach the subfolder: %v", got)
	}
	if got := f.itemState(alice, outerItem)["visibility"]; got != "public" {
		t.Errorf("recursive change did not reach an item: %v", got)
	}
	if status, _ := f.do(bob, http.MethodGet, "/api/items/"+innerItem, nil); status != http.StatusOK {
		t.Errorf("item in a public folder tree is not visible to others: %d", status)
	}

	// Recursive private hides the whole tree again.
	status, body = f.do(alice, http.MethodPut, "/api/collections/"+outer, map[string]any{
		"name": "Outer", "visibility": "private", "parentId": f.root(alice), "recursive": true,
	})
	if status != http.StatusOK {
		t.Fatalf("recursive private: %d %s", status, body)
	}
	if got := f.itemState(alice, outerItem)["visibility"]; got != "private" {
		t.Errorf("recursive private did not reach an item: %v", got)
	}
	if status, _ := f.do(bob, http.MethodGet, "/api/items/"+innerItem, nil); status != http.StatusNotFound {
		t.Errorf("item in a private tree is visible to others: %d", status)
	}
}

func TestBulkVisibility(t *testing.T) {
	f := newFixture(t)
	alice := f.newUser("Alice")
	bob := f.newUser("Bob")

	folder := f.folder(alice, "Shelf", f.root(alice))
	sub := f.folder(alice, "Sub", folder)
	first := f.item(alice, "first", folder, "private")
	second := f.item(alice, "second", sub, "private")
	outside := f.item(alice, "outside", "", "private")

	status, body := f.do(alice, http.MethodPost, "/api/items/bulk/visibility", map[string]any{
		"ids": []string{first, second}, "visibility": "public",
	})
	if status != http.StatusNoContent {
		t.Fatalf("bulk visibility items: %d %s", status, body)
	}
	for _, id := range []string{first, second} {
		if got := f.itemState(alice, id)["visibility"]; got != "public" {
			t.Errorf("item %s not made public: %v", id, got)
		}
	}
	if got := f.itemState(alice, outside)["visibility"]; got != "private" {
		t.Errorf("item outside the selection changed: %v", got)
	}

	// Without recursive, only the selected folder changes and its subfolder keeps its own visibility.
	status, body = f.do(alice, http.MethodPost, "/api/collections/bulk/visibility", map[string]any{
		"ids": []string{folder}, "visibility": "public", "recursive": false,
	})
	if status != http.StatusNoContent {
		t.Fatalf("bulk visibility folders: %d %s", status, body)
	}
	if got := f.folderState(alice, folder)["visibility"]; got != "public" {
		t.Errorf("selected folder not public: %v", got)
	}
	if got := f.folderState(alice, sub)["visibility"]; got != "private" {
		t.Errorf("non-recursive bulk change reached the subfolder: %v", got)
	}

	// With recursive, the subfolder and the items inside it follow.
	status, body = f.do(alice, http.MethodPost, "/api/collections/bulk/visibility", map[string]any{
		"ids": []string{folder}, "visibility": "private", "recursive": true,
	})
	if status != http.StatusNoContent {
		t.Fatalf("recursive bulk visibility: %d %s", status, body)
	}
	if got := f.folderState(alice, sub)["visibility"]; got != "private" {
		t.Errorf("recursive bulk change did not reach the subfolder: %v", got)
	}
	if status, _ := f.do(bob, http.MethodGet, "/api/items/"+second, nil); status != http.StatusNotFound {
		t.Errorf("item in a private folder is visible to others: %d", status)
	}

	status, body = f.do(alice, http.MethodPost, "/api/items/bulk/visibility", map[string]any{
		"ids": []string{first}, "visibility": "secret",
	})
	expectRejected(t, "invalid visibility", status, body)
	if !strings.Contains(string(body), "visibility") {
		t.Errorf("unexpected error message: %s", body)
	}
}
