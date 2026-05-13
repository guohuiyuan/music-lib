package apple

import (
	"fmt"
	"testing"
)

func TestApplePlaylistCategoriesIntegration(t *testing.T) {
	client := New("storefront=cn")
	categories, err := client.GetPlaylistCategories()
	if err != nil {
		t.Fatalf("GetPlaylistCategories failed: %v", err)
	}
	t.Logf("Got %d categories", len(categories))
	for i, c := range categories {
		if i >= 5 {
			break
		}
		t.Logf("  [%d] ID=%s Name=%s", i, c.ID, c.Name)
	}
	if len(categories) == 0 {
		t.Fatal("No categories returned")
	}

	// Test GetCategoryPlaylists with first category
	first := categories[0]
	playlists, err := client.GetCategoryPlaylists(first.ID, 1, 5)
	if err != nil {
		t.Fatalf("GetCategoryPlaylists(%s / %s) failed: %v", first.ID, first.Name, err)
	}
	t.Logf("Category %q has %d playlists (page 1, limit 5)", first.Name, len(playlists))
	for i, p := range playlists {
		fmt.Printf("  [%d] %s (ID=%s)\n", i, p.Name, p.ID)
	}
	if len(playlists) == 0 {
		t.Fatal("No playlists returned")
	}

	// Test with limit=120 (what the web UI uses)
	playlists2, err := client.GetCategoryPlaylists(first.ID, 1, 120)
	if err != nil {
		t.Fatalf("GetCategoryPlaylists(%s, 1, 120) failed: %v", first.ID, err)
	}
	t.Logf("Category %q with limit=120: got %d playlists", first.Name, len(playlists2))

	// Test K-Pop (1019399551) which was reported as 404
	kpop, err := client.GetCategoryPlaylists("1019399551", 1, 120)
	if err != nil {
		t.Fatalf("GetCategoryPlaylists(K-Pop) failed: %v", err)
	}
	t.Logf("K-Pop with limit=120: got %d playlists", len(kpop))
}
