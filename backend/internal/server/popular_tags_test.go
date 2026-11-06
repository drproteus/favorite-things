package server_test

import (
	"net/http"
	"testing"
)

func TestPopularTagsCountOnlyPublicItems(t *testing.T) {
	f := newFixture(t)
	alice := f.newUser("Alice")
	bob := f.newUser("Bob")

	hidden := f.folder(alice, "Hidden", f.root(alice))
	for _, post := range []struct {
		who   *account
		title string
		desc  string
		vis   string
		in    string
	}{
		{alice, "one", "Blue #retro #mint", "public", ""},
		{bob, "two", "Also #retro", "public", ""},
		{alice, "private", "#secretpick", "private", ""},
		{alice, "in private folder", "#hiddenpick", "public", hidden},
	} {
		body := map[string]any{"title": post.title, "description": post.desc, "visibility": post.vis}
		if post.in != "" {
			body["collectionId"] = post.in
		}
		if status, b := f.do(post.who, http.MethodPost, "/api/items", body); status != http.StatusCreated {
			t.Fatalf("create %s: %d %s", post.title, status, b)
		}
	}

	status, body := f.do(nil, http.MethodGet, "/api/tags/popular?limit=20", nil)
	if status != http.StatusOK {
		t.Fatalf("popular tags: %d %s", status, body)
	}
	list := decodeList(t, body)

	counts := map[string]float64{}
	for i, entry := range list {
		tag := entry["tag"].(string)
		counts[tag] = entry["count"].(float64)
		if i == 0 && tag != "retro" {
			t.Errorf("most used tag = %s, want retro", tag)
		}
	}
	if counts["retro"] != 2 {
		t.Errorf("retro count = %v, want 2", counts["retro"])
	}
	for _, tag := range []string{"secretpick", "hiddenpick"} {
		if _, ok := counts[tag]; ok {
			t.Errorf("#%s is on a private item or in a private folder but is listed", tag)
		}
	}
}
