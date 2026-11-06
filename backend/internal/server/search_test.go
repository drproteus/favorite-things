package server_test

import (
	"net/http"
	"testing"
)

func idsOf(t *testing.T, body []byte) map[string]bool {
	t.Helper()
	ids := map[string]bool{}
	for _, it := range decodeList(t, body) {
		ids[it["id"].(string)] = true
	}
	return ids
}

func TestSearchMatchesTitleDescriptionAndTags(t *testing.T) {
	f := newFixture(t)
	alice := f.newUser("Alice")
	bob := f.newUser("Bob")

	gameBoy := f.item(alice, "Game Boy Color", "", "public")
	f.do(alice, http.MethodPut, "/api/items/"+gameBoy, map[string]any{
		"title": "Game Boy Color", "visibility": "public", "description": "Blue, with #mint",
	})
	plain := f.item(alice, "Plain card", "", "public")
	bobRetro := f.item(bob, "Bob retro thing", "", "public")

	search := func(query string, auth *account) map[string]bool {
		t.Helper()
		status, body := f.do(nil, http.MethodGet, "/api/items?scope=public&q="+query, nil)
		if auth != nil {
			status, body = f.do(auth, http.MethodGet, "/api/items?scope=mine&q="+query, nil)
		}
		if status != http.StatusOK {
			t.Fatalf("search %q: %d %s", query, status, body)
		}
		return idsOf(t, body)
	}

	if got := search("color", nil); !got[gameBoy] || got[plain] {
		t.Errorf("title search for 'color': %v", got)
	}
	if got := search("mint", nil); !got[gameBoy] {
		t.Errorf("tag search for 'mint' misses the tagged item: %v", got)
	}
	if got := search("blue", nil); !got[gameBoy] {
		t.Errorf("description search for 'blue' misses the item: %v", got)
	}
	if got := search("retro", nil); !got[bobRetro] {
		t.Errorf("Explore search misses other users' public items: %v", got)
	}
	if got := search("retro", alice); got[bobRetro] {
		t.Errorf("'mine' search returned another user's item: %v", got)
	}
}

func TestPublicListingFiltersByOwnerAndFolder(t *testing.T) {
	f := newFixture(t)
	alice := f.newUser("Alice")
	bob := f.newUser("Bob")

	status, body := f.do(alice, http.MethodPost, "/api/collections", map[string]any{
		"name": "Shelf", "visibility": "public", "parentId": f.root(alice),
	})
	if status != http.StatusCreated {
		t.Fatalf("create Shelf: %d %s", status, body)
	}
	shelf := decodeMap(t, body)["id"].(string)

	inShelf := f.item(alice, "in shelf", shelf, "public")
	atRoot := f.item(alice, "at root", "", "public")
	bobItem := f.item(bob, "bob item", "", "public")

	status, body = f.do(nil, http.MethodGet, "/api/items?scope=public&owner="+alice.id, nil)
	if status != http.StatusOK {
		t.Fatalf("owner filter: %d %s", status, body)
	}
	got := idsOf(t, body)
	if !got[inShelf] || !got[atRoot] || got[bobItem] {
		t.Errorf("owner filter returned %v", got)
	}

	status, body = f.do(nil, http.MethodGet, "/api/items?scope=public&owner="+alice.id+"&collection="+shelf, nil)
	if status != http.StatusOK {
		t.Fatalf("owner and folder filter: %d %s", status, body)
	}
	if got := idsOf(t, body); !got[inShelf] || got[atRoot] {
		t.Errorf("owner and folder filter returned %v", got)
	}
}
