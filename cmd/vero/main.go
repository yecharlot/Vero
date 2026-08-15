package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	qrcode "github.com/skip2/go-qrcode"

	"github.com/yecharlot/Vero/internal/vero"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseKey := os.Getenv("SUPABASE_KEY")
	if supabaseURL != "" && supabaseKey != "" {
		log.Printf("🚀 Vero con SUPABASE: %s", supabaseURL)
	} else {
		log.Printf("⚠️ SUPABASE_URL/KEY no configurados — usando almacenamiento local")
	}
	dataDir := os.Getenv("VERO_DATA_DIR")
	if dataDir == "" {
		dataDir = "vero_data"
	}
	_ = os.MkdirAll(dataDir, 0755)

	svc := vero.NewService(dataDir)
	mux := http.NewServeMux()
	h := &handlers{svc: svc}
	h.mount(mux)

	// ensure UI on disk for static fallback
	_ = os.MkdirAll("static", 0755)
	if len(vero.AppHTML) > 0 {
		_ = os.WriteFile(filepath.Join("static", "index.html"), vero.AppHTML, 0644)
	}

	addr := ":" + port
	log.Printf("✅ Vero listening on %s (data: %s)", addr, dataDir)
	log.Printf(" App: http://localhost%s/", addr)
	log.Printf(" Profile: http://localhost%s/z/<slug>", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

type handlers struct {
	svc *vero.Service
}

func (h *handlers) mount(mux *http.ServeMux) {
	mux.HandleFunc("/api/vero/auth/register", h.register)
	mux.HandleFunc("/api/vero/auth/login", h.login)
	mux.HandleFunc("/api/vero/auth/logout", h.logout)
	mux.HandleFunc("/api/vero/auth/me", h.me)
	mux.HandleFunc("/api/vero/account", h.deleteAccount)
	mux.HandleFunc("/api/vero/businesses", h.businesses)
	mux.HandleFunc("/api/vero/businesses/", h.businessByID)
	mux.HandleFunc("/api/vero/public/", h.publicAPI)
	mux.HandleFunc("/api/vero/track/", h.track)
	mux.HandleFunc("/api/vero/qr/", h.qr)
	mux.HandleFunc("/z/", h.publicPage)
	mux.HandleFunc("/w/vero.app.ans", h.app)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"status": "ok", "app": "vero"})
	})
	mux.HandleFunc("/", h.app)
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func (h *handlers) session(r *http.Request) (*vero.Session, int, string) {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return nil, 401, "authorization required"
	}
	sess, ok := h.svc.Session(strings.TrimSpace(auth[7:]))
	if !ok {
		return nil, 401, "session expired"
	}
	return sess, 0, ""
}

func (h *handlers) app(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/w/vero.app.ans" {
		http.NotFound(w, r)
		return
	}
	html := vero.AppHTML
	if data, err := os.ReadFile(filepath.Join("static", "index.html")); err == nil && len(data) > 0 {
		html = data
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(html)
}

func (h *handlers) publicPage(w http.ResponseWriter, r *http.Request) {
	slug := strings.Trim(strings.TrimPrefix(r.URL.Path, "/z/"), "/")
	if slug == "" {
		http.NotFound(w, r)
		return
	}
	html := string(vero.AppHTML)
	boot := fmt.Sprintf(`<script>sessionStorage.setItem("vero_boot_slug",%q);if(!location.hash)location.hash="#/p/%s";</script>`, slug, slug)
	if i := strings.LastIndex(html, "</body>"); i >= 0 {
		html = html[:i] + boot + html[i:]
	} else {
		html += boot
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(html))
}

func (h *handlers) register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var req struct {
		Email, Password, Name string
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid json"})
		return
	}
	u, sess, err := h.svc.Register(req.Email, req.Password, req.Name)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 201, map[string]interface{}{"user": u, "token": sess.Token, "session": sess})
}

func (h *handlers) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var req struct {
		Email, Password string
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid json"})
		return
	}
	u, sess, err := h.svc.Login(req.Email, req.Password)
	if err != nil {
		writeJSON(w, 401, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"user": u, "token": sess.Token, "session": sess})
}

func (h *handlers) logout(w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		h.svc.Logout(strings.TrimSpace(auth[7:]))
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (h *handlers) me(w http.ResponseWriter, r *http.Request) {
	sess, code, msg := h.session(r)
	if code != 0 {
		writeJSON(w, code, map[string]string{"error": msg})
		return
	}
	writeJSON(w, 200, map[string]interface{}{
		"session":    sess,
		"businesses": h.svc.Store().ListByOwner(sess.UserID),
	})
}

func (h *handlers) deleteAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	sess, code, msg := h.session(r)
	if code != 0 {
		writeJSON(w, code, map[string]string{"error": msg})
		return
	}
	if err := h.svc.DeleteAccount(sess.UserID, sess.Token); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "deleted"})
}

func (h *handlers) businesses(w http.ResponseWriter, r *http.Request) {
	sess, code, msg := h.session(r)
	if code != 0 {
		writeJSON(w, code, map[string]string{"error": msg})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, 200, map[string]interface{}{"businesses": h.svc.Store().ListByOwner(sess.UserID)})
	case http.MethodPost:
		var in vero.CreateBusinessInput
		if json.NewDecoder(r.Body).Decode(&in) != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid json"})
			return
		}
		b, err := h.svc.CreateBusiness(sess.UserID, in)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 201, b)
	default:
		http.Error(w, "method not allowed", 405)
	}
}

func (h *handlers) businessByID(w http.ResponseWriter, r *http.Request) {
	sess, code, msg := h.session(r)
	if code != 0 {
		// allow public review POST without session on .../reviews
		path := strings.TrimPrefix(r.URL.Path, "/api/vero/businesses/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) >= 2 && parts[1] == "reviews" && r.Method == http.MethodPost {
			h.postReview(w, r, parts[0])
			return
		}
		writeJSON(w, code, map[string]string{"error": msg})
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/vero/businesses/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	id := parts[0]

	if len(parts) >= 2 && parts[1] == "products" {
		if len(parts) >= 3 {
			pid := parts[2]
			if r.Method == http.MethodDelete {
				if err := h.svc.DeleteProduct(sess.UserID, id, pid); err != nil {
					writeJSON(w, 400, map[string]string{"error": err.Error()})
					return
				}
				writeJSON(w, 200, map[string]string{"status": "deleted"})
				return
			}
		}
		switch r.Method {
		case http.MethodGet:
			b, ok := h.svc.Store().GetBusiness(id)
			if !ok || b.OwnerUserID != sess.UserID {
				writeJSON(w, 403, map[string]string{"error": "forbidden"})
				return
			}
			writeJSON(w, 200, map[string]interface{}{"products": h.svc.Store().GetProducts(id)})
		case http.MethodPost:
			var in vero.ProductInput
			if json.NewDecoder(r.Body).Decode(&in) != nil {
				writeJSON(w, 400, map[string]string{"error": "invalid json"})
				return
			}
			p, err := h.svc.AddProduct(sess.UserID, id, in)
			if err != nil {
				writeJSON(w, 400, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, 201, p)
		default:
			http.Error(w, "method not allowed", 405)
		}
		return
	}

	if len(parts) >= 2 && parts[1] == "stats" {
		b, ok := h.svc.Store().GetBusiness(id)
		if !ok || b.OwnerUserID != sess.UserID {
			writeJSON(w, 403, map[string]string{"error": "forbidden"})
			return
		}
		writeJSON(w, 200, h.svc.Store().GetStats(id))
		return
	}

	if len(parts) >= 2 && parts[1] == "reviews" && r.Method == http.MethodPost {
		h.postReview(w, r, id)
		return
	}

	switch r.Method {
	case http.MethodGet:
		b, ok := h.svc.Store().GetBusiness(id)
		if !ok || b.OwnerUserID != sess.UserID {
			writeJSON(w, 404, map[string]string{"error": "not found"})
			return
		}
		writeJSON(w, 200, map[string]interface{}{
			"business": b,
			"products": h.svc.Store().GetProducts(id),
			"stats":    h.svc.Store().GetStats(id),
			"reviews":  h.svc.Store().GetReviews(id),
		})
	case http.MethodPut, http.MethodPatch:
		var in vero.CreateBusinessInput
		if json.NewDecoder(r.Body).Decode(&in) != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid json"})
			return
		}
		b, err := h.svc.UpdateBusiness(sess.UserID, id, in)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, b)
	default:
		http.Error(w, "method not allowed", 405)
	}
}

func (h *handlers) postReview(w http.ResponseWriter, r *http.Request, businessID string) {
	var req struct {
		Rating  int    `json:"rating"`
		Comment string `json:"comment"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid json"})
		return
	}
	rev, err := h.svc.AddReview(businessID, req.Rating, req.Comment)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 201, rev)
}

func (h *handlers) publicAPI(w http.ResponseWriter, r *http.Request) {
	slug := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/vero/public/"), "/")
	if strings.HasSuffix(slug, "/reviews") && r.Method == http.MethodPost {
		slug = strings.Trim(strings.TrimSuffix(slug, "/reviews"), "/")
		b, ok := h.svc.Store().GetBySlug(slug)
		if !ok {
			writeJSON(w, 404, map[string]string{"error": "not found"})
			return
		}
		h.postReview(w, r, b.ID)
		return
	}
	prof, err := h.svc.PublicProfile(slug)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, 200, prof)
}

func (h *handlers) track(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/vero/track/"), "/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		writeJSON(w, 400, map[string]string{"error": "slug/event required"})
		return
	}
	h.svc.Track(parts[0], parts[1])
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (h *handlers) qr(w http.ResponseWriter, r *http.Request) {
	slug := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/vero/qr/"), "/")
	if slug == "" {
		http.Error(w, "missing slug", 400)
		return
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if xf := r.Header.Get("X-Forwarded-Proto"); xf != "" {
		scheme = xf
	}
	url := fmt.Sprintf("%s://%s/z/%s", scheme, r.Host, slug)
	png, err := qrcode.Encode(url, qrcode.Medium, 256)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	h.svc.Track(slug, "qr_scanned")
	w.Header().Set("Content-Type", "image/png")
	_, _ = w.Write(png)
}
