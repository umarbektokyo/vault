package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// setupTestDir creates a temporary directory tree for testing.
// Structure:
//
//	root/
//	  .hidden
//	  alpha.txt     (11 bytes)
//	  beta.png      (0 bytes)
//	  subdir/
//	    nested.json (2 bytes)
func setupTestDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	os.WriteFile(filepath.Join(root, ".hidden"), []byte("secret"), 0644)
	os.WriteFile(filepath.Join(root, "alpha.txt"), []byte("hello world"), 0644)
	os.WriteFile(filepath.Join(root, "beta.png"), []byte(""), 0644)
	os.Mkdir(filepath.Join(root, "subdir"), 0755)
	os.WriteFile(filepath.Join(root, "subdir", "nested.json"), []byte("{}"), 0644)

	return root
}

func browseRequest(t *testing.T, handler http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", "/api/browse?path="+path, nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func decodeBrowse(t *testing.T, rr *httptest.ResponseRecorder) BrowseResponse {
	t.Helper()
	var resp BrowseResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	return resp
}

func decodeError(t *testing.T, rr *httptest.ResponseRecorder) ErrorResponse {
	t.Helper()
	var resp ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	return resp
}

func TestBrowseRoot(t *testing.T) {
	root := setupTestDir(t)
	mux := newMux(root, t.TempDir())

	rr := browseRequest(t, mux, "/")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	resp := decodeBrowse(t, rr)

	if resp.Path != "/" {
		t.Errorf("expected path /, got %s", resp.Path)
	}

	// Should have: subdir, alpha.txt, beta.png (no .hidden)
	if len(resp.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d: %+v", len(resp.Entries), resp.Entries)
	}
}

func TestBrowseHiddenFilesExcluded(t *testing.T) {
	root := setupTestDir(t)
	mux := newMux(root, t.TempDir())

	rr := browseRequest(t, mux, "/")
	resp := decodeBrowse(t, rr)

	for _, e := range resp.Entries {
		if e.Name == ".hidden" {
			t.Error("hidden file should not be listed")
		}
	}
}

func TestBrowseSortOrder(t *testing.T) {
	root := setupTestDir(t)
	mux := newMux(root, t.TempDir())

	rr := browseRequest(t, mux, "/")
	resp := decodeBrowse(t, rr)

	// Directories first
	if !resp.Entries[0].IsDir {
		t.Error("first entry should be a directory")
	}
	if resp.Entries[0].Name != "subdir" {
		t.Errorf("expected first entry 'subdir', got '%s'", resp.Entries[0].Name)
	}

	// Then files alphabetically
	if resp.Entries[1].Name != "alpha.txt" {
		t.Errorf("expected second entry 'alpha.txt', got '%s'", resp.Entries[1].Name)
	}
	if resp.Entries[2].Name != "beta.png" {
		t.Errorf("expected third entry 'beta.png', got '%s'", resp.Entries[2].Name)
	}
}

func TestBrowseSubdirectory(t *testing.T) {
	root := setupTestDir(t)
	mux := newMux(root, t.TempDir())

	rr := browseRequest(t, mux, "/subdir")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	resp := decodeBrowse(t, rr)

	if len(resp.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(resp.Entries))
	}
	if resp.Entries[0].Name != "nested.json" {
		t.Errorf("expected 'nested.json', got '%s'", resp.Entries[0].Name)
	}
	if resp.Entries[0].Ext != ".json" {
		t.Errorf("expected ext '.json', got '%s'", resp.Entries[0].Ext)
	}
}

func TestBrowseEmptyPath(t *testing.T) {
	root := setupTestDir(t)
	mux := newMux(root, t.TempDir())

	req := httptest.NewRequest("GET", "/api/browse", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	resp := decodeBrowse(t, rr)
	if resp.Path != "/" {
		t.Errorf("empty path should default to /, got %s", resp.Path)
	}
}

func TestBrowsePathTraversal(t *testing.T) {
	root := setupTestDir(t)
	mux := newMux(root, t.TempDir())

	attacks := []string{
		"/../../../etc",
		"/..%2f..%2fetc",
		"/../",
		"/subdir/../../",
	}

	for _, path := range attacks {
		rr := browseRequest(t, mux, path)
		// Should either return the root (cleaned to /) or an error — never escape
		if rr.Code == http.StatusOK {
			resp := decodeBrowse(t, rr)
			// Verify it resolved within the content root
			for _, e := range resp.Entries {
				if e.Name == "etc" || e.Name == "passwd" {
					t.Errorf("path traversal succeeded with path %q", path)
				}
			}
		}
	}
}

func TestBrowseNonExistentPath(t *testing.T) {
	root := setupTestDir(t)
	mux := newMux(root, t.TempDir())

	rr := browseRequest(t, mux, "/does-not-exist")

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}

	resp := decodeError(t, rr)
	if resp.Error != "path not found" {
		t.Errorf("expected 'path not found', got '%s'", resp.Error)
	}
}

func TestBrowseFileNotDirectory(t *testing.T) {
	root := setupTestDir(t)
	mux := newMux(root, t.TempDir())

	rr := browseRequest(t, mux, "/alpha.txt")

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}

	resp := decodeError(t, rr)
	if resp.Error != "not a directory" {
		t.Errorf("expected 'not a directory', got '%s'", resp.Error)
	}
}

func TestBrowseFileMetadata(t *testing.T) {
	root := setupTestDir(t)
	mux := newMux(root, t.TempDir())

	rr := browseRequest(t, mux, "/")
	resp := decodeBrowse(t, rr)

	// Find alpha.txt
	var alpha *FileEntry
	for i := range resp.Entries {
		if resp.Entries[i].Name == "alpha.txt" {
			alpha = &resp.Entries[i]
			break
		}
	}
	if alpha == nil {
		t.Fatal("alpha.txt not found in listing")
	}

	if alpha.Size != 11 {
		t.Errorf("expected size 11, got %d", alpha.Size)
	}
	if alpha.Ext != ".txt" {
		t.Errorf("expected ext '.txt', got '%s'", alpha.Ext)
	}
	if alpha.IsDir {
		t.Error("alpha.txt should not be a directory")
	}
	if alpha.Path != "/alpha.txt" {
		t.Errorf("expected path '/alpha.txt', got '%s'", alpha.Path)
	}
	if alpha.ModTime.IsZero() {
		t.Error("ModTime should not be zero")
	}
}

func TestBrowseDirSizeIsZero(t *testing.T) {
	root := setupTestDir(t)
	mux := newMux(root, t.TempDir())

	rr := browseRequest(t, mux, "/")
	resp := decodeBrowse(t, rr)

	for _, e := range resp.Entries {
		if e.IsDir && e.Size != 0 {
			t.Errorf("directory %s should have size 0, got %d", e.Name, e.Size)
		}
	}
}

func TestFileServing(t *testing.T) {
	root := setupTestDir(t)
	mux := newMux(root, t.TempDir())

	req := httptest.NewRequest("GET", "/files/alpha.txt", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if body != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", body)
	}
}

func TestFileServingNested(t *testing.T) {
	root := setupTestDir(t)
	mux := newMux(root, t.TempDir())

	req := httptest.NewRequest("GET", "/files/subdir/nested.json", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	if rr.Body.String() != "{}" {
		t.Errorf("expected '{}', got '%s'", rr.Body.String())
	}
}

func TestFileServingNotFound(t *testing.T) {
	root := setupTestDir(t)
	mux := newMux(root, t.TempDir())

	req := httptest.NewRequest("GET", "/files/nonexistent.txt", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestContentTypeHeader(t *testing.T) {
	root := setupTestDir(t)
	mux := newMux(root, t.TempDir())

	rr := browseRequest(t, mux, "/")

	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got '%s'", ct)
	}
}
