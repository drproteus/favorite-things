package server_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestNewItemsAndFoldersDefaultToPublic(t *testing.T) {
	f := newFixture(t)
	alice := f.newUser("Alice")

	status, body := f.do(alice, http.MethodPost, "/api/items", map[string]any{"title": "no visibility given"})
	if status != http.StatusCreated {
		t.Fatalf("create item: %d %s", status, body)
	}
	if got := decodeMap(t, body)["visibility"]; got != "public" {
		t.Errorf("new item visibility = %v, want public", got)
	}

	status, body = f.do(alice, http.MethodPost, "/api/collections", map[string]any{"name": "no visibility given"})
	if status != http.StatusCreated {
		t.Fatalf("create folder: %d %s", status, body)
	}
	if got := decodeMap(t, body)["visibility"]; got != "public" {
		t.Errorf("new folder visibility = %v, want public", got)
	}
}

func TestSettingsChangeTheDefaultVisibility(t *testing.T) {
	f := newFixture(t)
	alice := f.newUser("Alice")

	status, body := f.do(alice, http.MethodPut, "/api/me", map[string]any{
		"profile": "Collector of odd things.", "defaultVisibility": "private",
	})
	if status != http.StatusOK {
		t.Fatalf("update settings: %d %s", status, body)
	}
	saved := decodeMap(t, body)
	if saved["profile"] != "Collector of odd things." || saved["defaultVisibility"] != "private" {
		t.Errorf("settings response = %v", saved)
	}

	status, body = f.do(alice, http.MethodGet, "/api/me", nil)
	if status != http.StatusOK {
		t.Fatalf("me: %d %s", status, body)
	}
	me := decodeMap(t, body)
	if me["profile"] != "Collector of odd things." || me["defaultVisibility"] != "private" {
		t.Errorf("/api/me does not reflect the settings: %v", me)
	}

	status, body = f.do(alice, http.MethodPost, "/api/items", map[string]any{"title": "now private by default"})
	if status != http.StatusCreated {
		t.Fatalf("create item: %d %s", status, body)
	}
	if got := decodeMap(t, body)["visibility"]; got != "private" {
		t.Errorf("new item visibility = %v after changing the default, want private", got)
	}

	// An explicit choice still wins over the default.
	status, body = f.do(alice, http.MethodPost, "/api/items", map[string]any{"title": "explicit", "visibility": "public"})
	if status != http.StatusCreated || decodeMap(t, body)["visibility"] != "public" {
		t.Errorf("explicit visibility was overridden: %d %s", status, body)
	}
}

func TestSettingsValidation(t *testing.T) {
	f := newFixture(t)
	alice := f.newUser("Alice")

	status, body := f.do(alice, http.MethodPut, "/api/me", map[string]any{"profile": "", "defaultVisibility": "secret"})
	expectRejected(t, "invalid default visibility", status, body)

	status, body = f.do(alice, http.MethodPut, "/api/me", map[string]any{
		"profile": strings.Repeat("a", 2001), "defaultVisibility": "public",
	})
	expectRejected(t, "profile too long", status, body)

	status, body = f.do(nil, http.MethodPut, "/api/me", map[string]any{"profile": "x", "defaultVisibility": "public"})
	if status != http.StatusUnauthorized {
		t.Errorf("anonymous settings update: %d %s, want 401", status, body)
	}
}

func TestPublicProfileShowsOnlyPublicParts(t *testing.T) {
	f := newFixture(t)
	alice := f.newUser("Alice")
	f.do(alice, http.MethodPut, "/api/me", map[string]any{"profile": "Records and retro games.", "defaultVisibility": "public"})

	status, body := f.do(nil, http.MethodGet, "/api/users/"+alice.id, nil)
	if status != http.StatusOK {
		t.Fatalf("public profile: %d %s", status, body)
	}
	profile := decodeMap(t, body)
	if profile["profile"] != "Records and retro games." || profile["name"] != "Alice" {
		t.Errorf("public profile = %v", profile)
	}
	if _, ok := profile["email"]; ok {
		t.Errorf("public profile exposes the email address: %v", profile)
	}

	if status, _ := f.do(nil, http.MethodGet, "/api/users/00000000-0000-0000-0000-000000000000", nil); status != http.StatusNotFound {
		t.Errorf("unknown user: status %d, want 404", status)
	}
	if status, _ := f.do(nil, http.MethodGet, "/api/users/not-a-uuid", nil); status != http.StatusNotFound {
		t.Errorf("invalid user id: status %d, want 404", status)
	}
}

func TestPublicProfileListsOnlyPublicItems(t *testing.T) {
	f := newFixture(t)
	alice := f.newUser("Alice")

	status, body := f.do(alice, http.MethodPost, "/api/collections", map[string]any{
		"name": "Shelf", "visibility": "public", "parentId": f.root(alice),
	})
	if status != http.StatusCreated {
		t.Fatalf("create public folder: %d %s", status, body)
	}
	shelf := decodeMap(t, body)["id"].(string)
	hidden := f.folder(alice, "Hidden", f.root(alice))
	publicItem := f.item(alice, "shown", shelf, "public")
	f.item(alice, "private item", shelf, "private")
	f.item(alice, "public but in a private folder", hidden, "public")

	status, body = f.do(nil, http.MethodGet, "/api/users/"+alice.id+"/items", nil)
	if status != http.StatusOK {
		t.Fatalf("profile items: %d %s", status, body)
	}
	list := decodeList(t, body)
	if len(list) != 1 || list[0]["id"] != publicItem {
		t.Errorf("profile lists %d items, want only the public one: %v", len(list), list)
	}
}
