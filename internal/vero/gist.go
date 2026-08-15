package vero

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Gist sync keeps a durable copy of Vero state on GitHub Gists.
// Env:
//   GITHUB_TOKEN  — classic token with "gist" scope
//   VERO_GIST_ID  — existing gist id (optional; created on first save if empty and token set)
//
// Not a production database: fine for MVP/single-instance; concurrent writers can race.

const gistAPI = "https://api.github.com/gists"
const gistStateFile = "vero_state.json"

type gistSnapshot struct {
	Users      map[string]User      `json:"users"`
	Sessions   map[string]Session   `json:"sessions"`
	Businesses map[string]Business  `json:"businesses"`
	Products   map[string][]Product `json:"products"`
	Reviews    map[string][]Review  `json:"reviews"`
	Stats      map[string]Stats     `json:"stats"`
	SavedAt    time.Time            `json:"saved_at"`
}

func gistEnabled() bool {
	return os.Getenv("GITHUB_TOKEN") != ""
}

func gistID() string {
	return os.Getenv("VERO_GIST_ID")
}

func setGistIDEnv(id string) {
	_ = os.Setenv("VERO_GIST_ID", id)
}

func (s *Store) snapshot() gistSnapshot {
	return gistSnapshot{
		Users:      s.users,
		Sessions:   s.sessions,
		Businesses: s.byID,
		Products:   s.products,
		Reviews:    s.reviews,
		Stats:      s.stats,
		SavedAt:    time.Now().UTC(),
	}
}

func (s *Store) applySnapshot(snap gistSnapshot) {
	if snap.Users != nil {
		s.users = snap.Users
	}
	if snap.Sessions != nil {
		s.sessions = snap.Sessions
	}
	if snap.Businesses != nil {
		s.byID = snap.Businesses
	}
	if snap.Products != nil {
		s.products = snap.Products
	}
	if snap.Reviews != nil {
		s.reviews = snap.Reviews
	}
	if snap.Stats != nil {
		s.stats = snap.Stats
	}
	s.bySlug = map[string]string{}
	for id, b := range s.byID {
		s.bySlug[toLowerSlug(b.Slug)] = id
	}
}

func toLowerSlug(slug string) string {
	b := make([]byte, len(slug))
	for i := 0; i < len(slug); i++ {
		c := slug[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

func (s *Store) loadFromGist() {
	if !gistEnabled() {
		return
	}
	id := gistID()
	if id == "" {
		return
	}
	req, err := http.NewRequest(http.MethodGet, gistAPI+"/"+id, nil)
	if err != nil {
		return
	}
	req.Header.Set("Authorization", "Bearer "+os.Getenv("GITHUB_TOKEN"))
	req.Header.Set("Accept", "application/vnd.github+json")
	client := &http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		fmt.Println("⚠️ Vero gist load:", err)
		return
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		body, _ := io.ReadAll(res.Body)
		fmt.Printf("⚠️ Vero gist load HTTP %d: %s\n", res.StatusCode, string(body))
		return
	}
	var payload struct {
		Files map[string]struct {
			Content string `json:"content"`
		} `json:"files"`
	}
	if json.NewDecoder(res.Body).Decode(&payload) != nil {
		return
	}
	f, ok := payload.Files[gistStateFile]
	if !ok || f.Content == "" {
		return
	}
	var snap gistSnapshot
	if json.Unmarshal([]byte(f.Content), &snap) != nil {
		return
	}
	s.mu.Lock()
	s.applySnapshot(snap)
	s.mu.Unlock()
	fmt.Println("✅ Vero state loaded from Gist", id)
}

func (s *Store) saveToGist() {
	if !gistEnabled() {
		return
	}
	s.mu.RLock()
	snap := s.snapshot()
	s.mu.RUnlock()
	raw, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return
	}
	id := gistID()
	token := os.Getenv("GITHUB_TOKEN")
	client := &http.Client{Timeout: 25 * time.Second}

	if id == "" {
		body := map[string]interface{}{
			"description": "Vero commercial identity state (auto)",
			"public":      false,
			"files": map[string]map[string]string{
				gistStateFile: {"content": string(raw)},
			},
		}
		b, _ := json.Marshal(body)
		req, _ := http.NewRequest(http.MethodPost, gistAPI, bytes.NewReader(b))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("Content-Type", "application/json")
		res, err := client.Do(req)
		if err != nil {
			fmt.Println("⚠️ Vero gist create:", err)
			return
		}
		defer res.Body.Close()
		var out struct {
			ID  string `json:"id"`
			HTML string `json:"html_url"`
		}
		_ = json.NewDecoder(res.Body).Decode(&out)
		if out.ID != "" {
			setGistIDEnv(out.ID)
			// persist id to disk so restarts keep it when env not set
			_ = os.WriteFile(s.path("gist_id.txt"), []byte(out.ID), 0600)
			fmt.Println("✅ Vero Gist created:", out.ID, out.HTML)
			fmt.Println("   Set VERO_GIST_ID=" + out.ID + " on Render to keep the same gist")
		} else {
			bodyBytes, _ := io.ReadAll(res.Body)
			fmt.Printf("⚠️ Vero gist create HTTP %d\n", res.StatusCode)
			_ = bodyBytes
		}
		return
	}

	body := map[string]interface{}{
		"files": map[string]map[string]string{
			gistStateFile: {"content": string(raw)},
		},
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPatch, gistAPI+"/"+id, bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		fmt.Println("⚠️ Vero gist save:", err)
		return
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		msg, _ := io.ReadAll(res.Body)
		fmt.Printf("⚠️ Vero gist save HTTP %d: %s\n", res.StatusCode, string(msg))
		return
	}
}
