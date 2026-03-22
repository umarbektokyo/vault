package main

import (
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type FileEntry struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	IsDir   bool      `json:"isDir"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modTime"`
	Ext     string    `json:"ext"`
}

type BrowseResponse struct {
	Path    string      `json:"path"`
	Entries []FileEntry `json:"entries"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func main() {
	contentRoot := "/contents"
	if v := os.Getenv("CONTENT_ROOT"); v != "" {
		contentRoot = v
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	frontendDir := "./frontend"
	if _, err := os.Stat(frontendDir); err != nil {
		frontendDir = "/app/frontend"
	}

	mux := newMux(contentRoot, frontendDir)

	log.Printf("Vault server starting on :%s", port)
	log.Printf("Serving files from %s", contentRoot)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}

func newMux(contentRoot, frontendDir string) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/browse", handleBrowse(contentRoot))

	fileServer := http.StripPrefix("/files/", http.FileServer(http.Dir(contentRoot)))
	mux.Handle("/files/", fileServer)

	frontend := http.FileServer(http.Dir(frontendDir))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(frontendDir, r.URL.Path)
		if _, err := os.Stat(path); os.IsNotExist(err) && !strings.Contains(r.URL.Path, ".") {
			http.ServeFile(w, r, filepath.Join(frontendDir, "index.html"))
			return
		}
		frontend.ServeHTTP(w, r)
	})

	return mux
}

func handleBrowse(contentRoot string) http.HandlerFunc {
	absContentRoot, err := filepath.Abs(contentRoot)
	if err != nil {
		log.Fatalf("invalid content root: %v", err)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		reqPath := r.URL.Query().Get("path")
		if reqPath == "" {
			reqPath = "/"
		}

		reqPath = filepath.Clean("/" + reqPath)
		fullPath := filepath.Join(absContentRoot, reqPath)

		absPath, err := filepath.Abs(fullPath)
		if err != nil || !strings.HasPrefix(absPath, absContentRoot) {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid path"})
			return
		}

		info, err := os.Stat(absPath)
		if err != nil {
			writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "path not found"})
			return
		}

		if !info.IsDir() {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "not a directory"})
			return
		}

		entries, err := os.ReadDir(absPath)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "cannot read directory"})
			return
		}

		files := make([]FileEntry, 0, len(entries))
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), ".") {
				continue
			}

			info, err := e.Info()
			if err != nil {
				continue
			}

			files = append(files, FileEntry{
				Name:    e.Name(),
				Path:    filepath.Join(reqPath, e.Name()),
				IsDir:   e.IsDir(),
				Size:    fileSize(info),
				ModTime: info.ModTime(),
				Ext:     strings.ToLower(filepath.Ext(e.Name())),
			})
		}

		sort.Slice(files, func(i, j int) bool {
			if files[i].IsDir != files[j].IsDir {
				return files[i].IsDir
			}
			return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
		})

		writeJSON(w, http.StatusOK, BrowseResponse{Path: reqPath, Entries: files})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func fileSize(info fs.FileInfo) int64 {
	if info.IsDir() {
		return 0
	}
	return info.Size()
}
