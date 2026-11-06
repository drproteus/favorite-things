package server_test

import (
	"net/http"
	"testing"
)

func TestPublicFolderViewRespectsPrivateFolders(t *testing.T) {
	f := newFixture(t)
	alice := f.newUser("Alice")
	root := f.root(alice)

	status, body := f.do(alice, http.MethodPost, "/api/collections", map[string]any{
		"name": "Shelf", "visibility": "public", "parentId": root,
	})
	if status != http.StatusCreated {
		t.Fatalf("create Shelf: %d %s", status, body)
	}
	shelf := decodeMap(t, body)["id"].(string)

	status, body = f.do(alice, http.MethodPost, "/api/collections", map[string]any{
		"name": "Sub", "visibility": "private", "parentId": shelf,
	})
	if status != http.StatusCreated {
		t.Fatalf("create Sub: %d %s", status, body)
	}
	sub := decodeMap(t, body)["id"].(string)

	hidden := f.folder(alice, "Hidden", root)
	shownItem := f.item(alice, "shown", shelf, "public")
	subItem := f.item(alice, "in private sub", sub, "public")
	hiddenItem := f.item(alice, "in hidden", hidden, "public")

	// Folder list: root and Shelf are visible to anyone; Sub and Hidden are not.
	status, body = f.do(nil, http.MethodGet, "/api/users/"+alice.id+"/collections", nil)
	if status != http.StatusOK {
		t.Fatalf("public folders: %d %s", status, body)
	}
	visible := map[string]bool{}
	for _, c := range decodeList(t, body) {
		visible[c["id"].(string)] = true
	}
	if !visible[root] || !visible[shelf] {
		t.Errorf("root or Shelf missing from the public folder list: %v", visible)
	}
	if visible[sub] || visible[hidden] {
		t.Errorf("a private folder or a folder inside one is listed publicly: %v", visible)
	}

	// Items: the folder filter returns only what is public inside that folder.
	itemsIn := func(folderID string) []string {
		t.Helper()
		status, body := f.do(nil, http.MethodGet, "/api/users/"+alice.id+"/items?collection="+folderID, nil)
		if status != http.StatusOK {
			t.Fatalf("items in %s: %d %s", folderID, status, body)
		}
		var ids []string
		for _, it := range decodeList(t, body) {
			ids = append(ids, it["id"].(string))
		}
		return ids
	}
	if got := itemsIn(shelf); len(got) != 1 || got[0] != shownItem {
		t.Errorf("Shelf items = %v, want only %s", got, shownItem)
	}
	if got := itemsIn(sub); len(got) != 0 {
		t.Errorf("items in a private folder are visible: %v (%s)", got, subItem)
	}
	if got := itemsIn(hidden); len(got) != 0 {
		t.Errorf("items in a hidden folder are visible: %v (%s)", got, hiddenItem)
	}

	if status, _ := f.do(nil, http.MethodGet, "/api/users/"+alice.id+"/items?collection=not-a-uuid", nil); status != http.StatusNotFound {
		t.Errorf("invalid folder id: status %d, want 404", status)
	}
}
