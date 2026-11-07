package server_test

import (
	"net/http"
	"testing"
)

func (f *fixture) comments(a *account, itemID string) (int, []byte) {
	f.t.Helper()
	return f.do(a, http.MethodGet, "/api/items/"+itemID+"/comments", nil)
}

func TestItemCommentsAreForSignedInViewersAndOwnerCanEdit(t *testing.T) {
	f := newFixture(t)
	alice := f.newUser("Alice")
	bob := f.newUser("Bob")
	item := f.item(alice, "Camera", f.root(alice), "public")

	// Anonymous users can neither read nor write comments, even on a public item.
	if status, body := f.comments(nil, item); status != http.StatusUnauthorized {
		t.Errorf("anonymous comments: %d %s, want 401", status, body)
	}
	if status, body := f.do(nil, http.MethodPost, "/api/items/"+item+"/comments", map[string]any{"body": "hello"}); status != http.StatusUnauthorized {
		t.Errorf("anonymous create: %d %s, want 401", status, body)
	}

	// Another signed-in user can see comments but can't add one while comments are owner-only.
	status, body := f.comments(bob, item)
	if status != http.StatusOK {
		t.Fatalf("list comments: %d %s", status, body)
	}
	if got := decodeList(t, body); len(got) != 0 {
		t.Errorf("new item has comments: %v", got)
	}
	if status, body := f.do(bob, http.MethodPost, "/api/items/"+item+"/comments", map[string]any{"body": "hello"}); status != http.StatusForbidden {
		t.Errorf("non-owner create: %d %s, want 403", status, body)
	}

	status, body = f.do(alice, http.MethodPost, "/api/items/"+item+"/comments", map[string]any{
		"body": "  First **comment** with #camera  ",
	})
	if status != http.StatusCreated {
		t.Fatalf("create comment: %d %s", status, body)
	}
	comment := decodeMap(t, body)
	if got := comment["body"]; got != "First **comment** with #camera" {
		t.Errorf("body = %q", got)
	}
	if got := comment["visibility"]; got != "signed_in" {
		t.Errorf("visibility = %v, want signed_in", got)
	}
	if got := comment["authorId"]; got != alice.id {
		t.Errorf("author = %v, want %s", got, alice.id)
	}
	commentID := comment["id"].(string)

	// A signed-in viewer of the public item sees the comment.
	status, body = f.comments(bob, item)
	if status != http.StatusOK {
		t.Fatalf("bob list comments: %d %s", status, body)
	}
	if got := decodeList(t, body); len(got) != 1 || got[0]["id"] != commentID {
		t.Fatalf("bob comments = %v, want the owner's comment", got)
	}

	// Only the author can update, and owners can't loosen visibility yet.
	if status, body := f.do(bob, http.MethodPut, "/api/items/"+item+"/comments/"+commentID, map[string]any{"body": "hijacked"}); status != http.StatusNotFound {
		t.Errorf("non-author update: %d %s, want 404", status, body)
	}
	if status, body := f.do(alice, http.MethodPut, "/api/items/"+item+"/comments/"+commentID, map[string]any{"body": "  ", "visibility": "signed_in"}); status != http.StatusBadRequest {
		t.Errorf("empty update: %d %s, want 400", status, body)
	}
	if status, body := f.do(alice, http.MethodPut, "/api/items/"+item+"/comments/"+commentID, map[string]any{"body": "public now", "visibility": "public"}); status != http.StatusBadRequest {
		t.Errorf("public visibility: %d %s, want 400", status, body)
	}

	status, body = f.do(alice, http.MethodPut, "/api/items/"+item+"/comments/"+commentID, map[string]any{
		"body": "Edited comment",
	})
	if status != http.StatusOK {
		t.Fatalf("update comment: %d %s", status, body)
	}
	updated := decodeMap(t, body)
	if got := updated["body"]; got != "Edited comment" {
		t.Errorf("updated body = %v", got)
	}
	if got := updated["visibility"]; got != "signed_in" {
		t.Errorf("updated visibility = %v, want signed_in", got)
	}
}

func TestItemCommentsRespectItemVisibility(t *testing.T) {
	f := newFixture(t)
	alice := f.newUser("Alice")
	bob := f.newUser("Bob")
	item := f.item(alice, "Private item", f.root(alice), "private")

	status, body := f.do(alice, http.MethodPost, "/api/items/"+item+"/comments", map[string]any{"body": "private note"})
	if status != http.StatusCreated {
		t.Fatalf("create comment: %d %s", status, body)
	}
	if status, body := f.comments(alice, item); status != http.StatusOK {
		t.Errorf("owner list comments: %d %s", status, body)
	}

	// A private item stays invisible, and therefore its comments do too.
	if status, body := f.comments(bob, item); status != http.StatusNotFound {
		t.Errorf("other user list comments on private item: %d %s, want 404", status, body)
	}
	if status, body := f.do(bob, http.MethodPost, "/api/items/"+item+"/comments", map[string]any{"body": "nope"}); status != http.StatusNotFound {
		t.Errorf("other user create on private item: %d %s, want 404", status, body)
	}
}
