package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type FileEntry struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	IsDir     bool      `json:"isDir"`
	Size      int64     `json:"size"`
	ModTime   time.Time `json:"modTime"`
	Ext       string    `json:"ext"`
	HasSecret bool      `json:"hasSecret"`
}

type BrowseResponse struct {
	Path    string      `json:"path"`
	Entries []FileEntry `json:"entries"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// Config holds server configuration.
type Config struct {
	ContentRoot string
	FrontendDir string
	AuthUser    string
	AuthPass    string
	HMACSecret  []byte
}

// SecretsStore manages secret codes for files.
type SecretsStore struct {
	mu       sync.RWMutex
	codes    map[string]string // path -> HMAC hash of code
	filePath string
	hmacKey  []byte
}

func newSecretsStore(path string, hmacKey []byte) *SecretsStore {
	s := &SecretsStore{
		codes:    make(map[string]string),
		filePath: path,
		hmacKey:  hmacKey,
	}
	s.load()
	return s
}

func (s *SecretsStore) load() {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return
	}
	json.Unmarshal(data, &s.codes)
}

func (s *SecretsStore) save() error {
	data, err := json.MarshalIndent(s.codes, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0644)
}

func (s *SecretsStore) hashCode(code string) string {
	mac := hmac.New(sha256.New, s.hmacKey)
	mac.Write([]byte(code))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *SecretsStore) Set(path, code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.codes[path] = s.hashCode(code)
	return s.save()
}

func (s *SecretsStore) Remove(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.codes, path)
	return s.save()
}

func (s *SecretsStore) Verify(path, code string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	hash, ok := s.codes[path]
	if !ok {
		return false
	}
	return hmac.Equal([]byte(hash), []byte(s.hashCode(code)))
}

func (s *SecretsStore) HasSecret(path string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.codes[path]
	return ok
}

func (s *SecretsStore) RenamePath(oldPath, newPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	updated := false
	for k, v := range s.codes {
		if k == oldPath {
			delete(s.codes, k)
			s.codes[newPath] = v
			updated = true
		} else if strings.HasPrefix(k, oldPath+"/") {
			delete(s.codes, k)
			s.codes[newPath+k[len(oldPath):]] = v
			updated = true
		}
	}
	if updated {
		return s.save()
	}
	return nil
}

func (s *SecretsStore) DeletePath(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	updated := false
	for k := range s.codes {
		if k == path || strings.HasPrefix(k, path+"/") {
			delete(s.codes, k)
			updated = true
		}
	}
	if updated {
		return s.save()
	}
	return nil
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

	frontendDir := "./frontend/dist"
	candidates := []string{"./frontend/dist", "./frontend", "../frontend/dist", "/app/frontend"}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			frontendDir = c
			break
		}
	}

	authUser := os.Getenv("VAULT_USER")
	authPass := os.Getenv("VAULT_PASS")

	hmacSecret := []byte(os.Getenv("VAULT_SECRET"))
	if len(hmacSecret) == 0 && authPass != "" {
		hmacSecret = []byte("vault-secret-" + authPass)
	}
	if len(hmacSecret) == 0 {
		hmacSecret = []byte("vault-default-key")
	}

	cfg := Config{
		ContentRoot: contentRoot,
		FrontendDir: frontendDir,
		AuthUser:    authUser,
		AuthPass:    authPass,
		HMACSecret:  hmacSecret,
	}

	mux := newMux(cfg)

	log.Printf("Vault server starting on :%s", port)
	log.Printf("Serving files from %s", contentRoot)
	if authUser != "" {
		log.Printf("Authentication enabled for user: %s", authUser)
	}
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}

func newMux(cfg Config) *http.ServeMux {
	absContentRoot, err := filepath.Abs(cfg.ContentRoot)
	if err != nil {
		log.Fatalf("invalid content root: %v", err)
	}

	secrets := newSecretsStore(
		filepath.Join(absContentRoot, ".vault-secrets.json"),
		cfg.HMACSecret,
	)

	mux := http.NewServeMux()

	mux.HandleFunc("/api/browse", handleBrowse(absContentRoot, secrets))
	mux.HandleFunc("/api/auth", handleAuth(cfg))
	mux.HandleFunc("/api/login", handleLogin(cfg))
	mux.HandleFunc("/api/logout", handleLogout())
	mux.HandleFunc("/api/upload", requireAuth(cfg, handleUpload(absContentRoot)))
	mux.HandleFunc("/api/mkdir", requireAuth(cfg, handleMkdir(absContentRoot)))
	mux.HandleFunc("/api/rename", requireAuth(cfg, handleRename(absContentRoot, secrets)))
	mux.HandleFunc("/api/move", requireAuth(cfg, handleMove(absContentRoot, secrets)))
	mux.HandleFunc("/api/delete", requireAuth(cfg, handleDelete(absContentRoot, secrets)))
	mux.HandleFunc("/api/secret", requireAuth(cfg, handleSecret(absContentRoot, secrets)))
	mux.HandleFunc("/api/unlock", handleUnlock(absContentRoot, secrets, cfg))

	mux.HandleFunc("/files/", handleFiles(absContentRoot, secrets, cfg))

	frontendDir := cfg.FrontendDir
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

// --- Auth helpers ---

func signValue(value string, key []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}

func isAuthenticated(r *http.Request, cfg Config) bool {
	if cfg.AuthUser == "" || cfg.AuthPass == "" {
		return false
	}
	cookie, err := r.Cookie("vault_session")
	if err != nil {
		return false
	}
	parts := strings.SplitN(cookie.Value, "|", 2)
	if len(parts) != 2 {
		return false
	}
	username, sig := parts[0], parts[1]
	expected := signValue(username, cfg.HMACSecret)
	return hmac.Equal([]byte(sig), []byte(expected)) && username == cfg.AuthUser
}

func requireAuth(cfg Config, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isAuthenticated(r, cfg) {
			writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
			return
		}
		next(w, r)
	}
}

// getUnlockedPaths returns the set of paths unlocked by the request's cookie.
func getUnlockedPaths(r *http.Request, cfg Config) map[string]bool {
	cookie, err := r.Cookie("vault_unlocked")
	if err != nil {
		return nil
	}
	parts := strings.SplitN(cookie.Value, ".", 2)
	if len(parts) != 2 {
		return nil
	}
	b64Payload, sig := parts[0], parts[1]
	expected := signValue(b64Payload, cfg.HMACSecret)
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(b64Payload)
	if err != nil {
		return nil
	}
	var paths []string
	if err := json.Unmarshal(payload, &paths); err != nil {
		return nil
	}
	result := make(map[string]bool, len(paths))
	for _, p := range paths {
		result[p] = true
	}
	return result
}

func setUnlockedCookie(w http.ResponseWriter, paths map[string]bool, cfg Config) {
	list := make([]string, 0, len(paths))
	for p := range paths {
		list = append(list, p)
	}
	payload, _ := json.Marshal(list)
	b64 := base64.RawURLEncoding.EncodeToString(payload)
	sig := signValue(b64, cfg.HMACSecret)
	http.SetCookie(w, &http.Cookie{
		Name:     "vault_unlocked",
		Value:    b64 + "." + sig,
		Path:     "/",
		MaxAge:   86400, // 24 hours
		SameSite: http.SameSiteLaxMode,
		HttpOnly: true,
	})
}

// --- Handlers ---

func handleAuth(cfg Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		authEnabled := cfg.AuthUser != "" && cfg.AuthPass != ""
		authenticated := isAuthenticated(r, cfg)
		writeJSON(w, http.StatusOK, map[string]bool{
			"authEnabled":   authEnabled,
			"authenticated": authenticated,
		})
	}
}

func handleLogin(cfg Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
			return
		}

		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
			return
		}

		if cfg.AuthUser == "" || cfg.AuthPass == "" {
			writeJSON(w, http.StatusForbidden, ErrorResponse{Error: "authentication not configured"})
			return
		}

		if req.Username != cfg.AuthUser || req.Password != cfg.AuthPass {
			writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "invalid credentials"})
			return
		}

		sig := signValue(req.Username, cfg.HMACSecret)
		http.SetCookie(w, &http.Cookie{
			Name:     "vault_session",
			Value:    req.Username + "|" + sig,
			Path:     "/",
			MaxAge:   86400 * 7, // 7 days
			SameSite: http.SameSiteLaxMode,
			HttpOnly: true,
		})
		writeJSON(w, http.StatusOK, map[string]bool{"authenticated": true})
	}
}

func handleLogout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:   "vault_session",
			Value:  "",
			Path:   "/",
			MaxAge: -1,
		})
		writeJSON(w, http.StatusOK, map[string]bool{"authenticated": false})
	}
}

func handleUpload(absContentRoot string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
			return
		}

		// 100MB max
		r.ParseMultipartForm(100 << 20)

		file, header, err := r.FormFile("file")
		if err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "no file provided"})
			return
		}
		defer file.Close()

		targetDir := r.FormValue("path")
		if targetDir == "" {
			targetDir = "/"
		}

		targetDir = filepath.Clean("/" + targetDir)
		fullDir := filepath.Join(absContentRoot, targetDir)

		absDir, err := filepath.Abs(fullDir)
		if err != nil || !strings.HasPrefix(absDir, absContentRoot) {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid path"})
			return
		}

		info, err := os.Stat(absDir)
		if err != nil || !info.IsDir() {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "target directory does not exist"})
			return
		}

		// Sanitize filename
		filename := filepath.Base(header.Filename)
		if filename == "." || filename == ".." || strings.HasPrefix(filename, ".") {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid filename"})
			return
		}

		destPath := filepath.Join(absDir, filename)
		dest, err := os.Create(destPath)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "failed to create file"})
			return
		}
		defer dest.Close()

		if _, err := io.Copy(dest, file); err != nil {
			os.Remove(destPath)
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "failed to write file"})
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{
			"name": filename,
			"path": filepath.Join(targetDir, filename),
		})
	}
}

func handleMkdir(absContentRoot string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
			return
		}
		var req struct {
			Path string `json:"path"`
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
			return
		}
		parentDir := filepath.Clean("/" + req.Path)
		name := filepath.Base(req.Name)
		if name == "." || name == ".." || strings.HasPrefix(name, ".") || strings.Contains(req.Name, "/") {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid folder name"})
			return
		}
		absDir := filepath.Join(absContentRoot, parentDir, name)
		if !strings.HasPrefix(absDir, absContentRoot) {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid path"})
			return
		}
		if _, err := os.Stat(absDir); err == nil {
			writeJSON(w, http.StatusConflict, ErrorResponse{Error: "a file or folder with that name already exists"})
			return
		}
		if err := os.Mkdir(absDir, 0755); err != nil {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "failed to create directory"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"path": filepath.Join(parentDir, name)})
	}
}

func handleRename(absContentRoot string, secrets *SecretsStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
			return
		}
		var req struct {
			Path    string `json:"path"`
			NewName string `json:"newName"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
			return
		}
		cleanPath := filepath.Clean("/" + req.Path)
		if cleanPath == "/" {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "cannot rename root"})
			return
		}
		newName := filepath.Base(req.NewName)
		if newName == "." || newName == ".." || strings.HasPrefix(newName, ".") || strings.Contains(newName, "/") {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid name"})
			return
		}
		oldAbs := filepath.Join(absContentRoot, cleanPath)
		if !strings.HasPrefix(oldAbs, absContentRoot) {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid path"})
			return
		}
		if _, err := os.Stat(oldAbs); os.IsNotExist(err) {
			writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "not found"})
			return
		}
		newAbs := filepath.Join(filepath.Dir(oldAbs), newName)
		if _, err := os.Stat(newAbs); err == nil {
			writeJSON(w, http.StatusConflict, ErrorResponse{Error: "a file or folder with that name already exists"})
			return
		}
		if err := os.Rename(oldAbs, newAbs); err != nil {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "rename failed"})
			return
		}
		newCleanPath := filepath.Join(filepath.Dir(cleanPath), newName)
		secrets.RenamePath(cleanPath, newCleanPath)
		writeJSON(w, http.StatusOK, map[string]string{"newPath": newCleanPath})
	}
}

func handleMove(absContentRoot string, secrets *SecretsStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
			return
		}
		var req struct {
			Path string `json:"path"`
			Dest string `json:"dest"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
			return
		}
		cleanPath := filepath.Clean("/" + req.Path)
		if cleanPath == "/" {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "cannot move root"})
			return
		}
		destDir := filepath.Clean("/" + req.Dest)
		oldAbs := filepath.Join(absContentRoot, cleanPath)
		destAbs := filepath.Join(absContentRoot, destDir)
		if !strings.HasPrefix(oldAbs, absContentRoot) || !strings.HasPrefix(destAbs, absContentRoot) {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid path"})
			return
		}
		if _, err := os.Stat(oldAbs); os.IsNotExist(err) {
			writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "source not found"})
			return
		}
		info, err := os.Stat(destAbs)
		if err != nil || !info.IsDir() {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "destination directory does not exist"})
			return
		}
		name := filepath.Base(cleanPath)
		newAbs := filepath.Join(destAbs, name)
		if _, err := os.Stat(newAbs); err == nil {
			writeJSON(w, http.StatusConflict, ErrorResponse{Error: "a file or folder with that name already exists in the destination"})
			return
		}
		if err := os.Rename(oldAbs, newAbs); err != nil {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "move failed"})
			return
		}
		newCleanPath := filepath.Join(destDir, name)
		secrets.RenamePath(cleanPath, newCleanPath)
		writeJSON(w, http.StatusOK, map[string]string{"newPath": newCleanPath})
	}
}

func handleDelete(absContentRoot string, secrets *SecretsStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
			return
		}
		var req struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
			return
		}
		cleanPath := filepath.Clean("/" + req.Path)
		if cleanPath == "/" {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "cannot delete root"})
			return
		}
		absPath := filepath.Join(absContentRoot, cleanPath)
		if !strings.HasPrefix(absPath, absContentRoot) {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid path"})
			return
		}
		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "not found"})
			return
		}
		if err := os.RemoveAll(absPath); err != nil {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "delete failed"})
			return
		}
		secrets.DeletePath(cleanPath)
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	}
}

func handleSecret(absContentRoot string, secrets *SecretsStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
			return
		}

		var req struct {
			Path string `json:"path"`
			Code string `json:"code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
			return
		}

		req.Path = filepath.Clean("/" + req.Path)
		fullPath := filepath.Join(absContentRoot, req.Path)
		absPath, err := filepath.Abs(fullPath)
		if err != nil || !strings.HasPrefix(absPath, absContentRoot) {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid path"})
			return
		}

		info, err := os.Stat(absPath)
		if err != nil {
			writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "file not found"})
			return
		}
		if info.IsDir() {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "cannot set secret on directory"})
			return
		}

		if req.Code == "" {
			if err := secrets.Remove(req.Path); err != nil {
				writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "failed to remove secret"})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"path": req.Path, "hasSecret": false})
		} else {
			if err := secrets.Set(req.Path, req.Code); err != nil {
				writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "failed to set secret"})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"path": req.Path, "hasSecret": true})
		}
	}
}

func handleUnlock(_ string, secrets *SecretsStore, cfg Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
			return
		}

		var req struct {
			Path string `json:"path"`
			Code string `json:"code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
			return
		}

		req.Path = filepath.Clean("/" + req.Path)

		if !secrets.Verify(req.Path, req.Code) {
			writeJSON(w, http.StatusForbidden, ErrorResponse{Error: "incorrect code"})
			return
		}

		// Add to unlocked paths cookie
		unlocked := getUnlockedPaths(r, cfg)
		if unlocked == nil {
			unlocked = make(map[string]bool)
		}
		unlocked[req.Path] = true
		setUnlockedCookie(w, unlocked, cfg)

		writeJSON(w, http.StatusOK, map[string]any{"unlocked": true, "path": req.Path})
	}
}

func handleFiles(absContentRoot string, secrets *SecretsStore, cfg Config) http.HandlerFunc {
	fileServer := http.StripPrefix("/files/", http.FileServer(http.Dir(absContentRoot)))

	return func(w http.ResponseWriter, r *http.Request) {
		// Extract the file path from the URL
		reqPath := strings.TrimPrefix(r.URL.Path, "/files")
		reqPath = filepath.Clean("/" + reqPath)

		// Check if this file has a secret
		if secrets.HasSecret(reqPath) {
			// Authenticated users bypass secret checks
			if !isAuthenticated(r, cfg) {
				// Check unlocked cookie
				unlocked := getUnlockedPaths(r, cfg)
				if !unlocked[reqPath] {
					w.Header().Set("Content-Type", "application/json")
					writeJSON(w, http.StatusForbidden, ErrorResponse{Error: "this file requires a secret code"})
					return
				}
			}
		}

		fileServer.ServeHTTP(w, r)
	}
}

func handleBrowse(absContentRoot string, secrets *SecretsStore) http.HandlerFunc {
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

			entryPath := filepath.Join(reqPath, e.Name())
			files = append(files, FileEntry{
				Name:      e.Name(),
				Path:      entryPath,
				IsDir:     e.IsDir(),
				Size:      fileSize(info),
				ModTime:   info.ModTime(),
				Ext:       strings.ToLower(filepath.Ext(e.Name())),
				HasSecret: secrets.HasSecret(entryPath),
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

