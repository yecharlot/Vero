package vero

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Store persists Vero data. When SUPABASE_URL + SUPABASE_KEY are set,
// durable data lives in Supabase (PostgREST). Sessions stay in-memory.
// Otherwise falls back to local JSON files (dev only; ephemeral on Render).
type Store struct {
	mu       sync.RWMutex
	dir      string
	sb       *SupabaseClient
	users    map[string]User
	sessions map[string]Session
	byID     map[string]Business
	bySlug   map[string]string
	products map[string][]Product
	reviews  map[string][]Review
	stats    map[string]Stats
}

func NewStore(dataDir string) *Store {
	if dataDir == "" {
		dataDir = "vero_data"
	}
	dir := filepath.Join(dataDir, "vero")
	_ = os.MkdirAll(dir, 0755)
	s := &Store{
		dir:      dir,
		sessions: map[string]Session{},
		users:    map[string]User{},
		byID:     map[string]Business{},
		bySlug:   map[string]string{},
		products: map[string][]Product{},
		reviews:  map[string][]Review{},
		stats:    map[string]Stats{},
	}
	if sb := SupabaseFromEnv(); sb != nil {
		s.sb = sb
		if err := sb.Ping(); err != nil {
			log.Printf("⚠️ Supabase: %v", err)
		} else {
			log.Printf("✅ Supabase storage listo")
		}
		return s
	}
	s.load()
	log.Printf("⚠️ Usando almacenamiento local (efímero): %s", dir)
	return s
}

func (s *Store) UsingSupabase() bool { return s.sb != nil }

func (s *Store) path(name string) string { return filepath.Join(s.dir, name) }

func (s *Store) load() {
	s.loadJSON("users.json", &s.users)
	s.loadJSON("sessions.json", &s.sessions)
	s.loadJSON("businesses.json", &s.byID)
	s.loadJSON("products.json", &s.products)
	s.loadJSON("reviews.json", &s.reviews)
	s.loadJSON("stats.json", &s.stats)
	s.bySlug = map[string]string{}
	for id, b := range s.byID {
		s.bySlug[strings.ToLower(b.Slug)] = id
	}
}

func (s *Store) loadJSON(name string, dest interface{}) {
	b, err := os.ReadFile(s.path(name))
	if err != nil {
		return
	}
	_ = json.Unmarshal(b, dest)
}

func (s *Store) saveJSON(name string, v interface{}) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(name), b, 0600)
}

func (s *Store) PutUser(u User) error {
	if s.sb != nil {
		return s.sb.UpsertUser(u)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users[strings.ToLower(u.Email)] = u
	return s.saveJSON("users.json", s.users)
}

func (s *Store) GetUser(email string) (User, bool) {
	email = strings.ToLower(strings.TrimSpace(email))
	if s.sb != nil {
		u, ok, err := s.sb.GetUserByEmail(email)
		if err != nil {
			log.Printf("supabase GetUser: %v", err)
			return User{}, false
		}
		return u, ok
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[email]
	return u, ok
}

func (s *Store) DeleteUser(userID string) error {
	if s.sb != nil {
		return s.sb.DeleteUser(userID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for email, u := range s.users {
		if u.ID == userID {
			delete(s.users, email)
			break
		}
	}
	for tok, sess := range s.sessions {
		if sess.UserID == userID {
			delete(s.sessions, tok)
		}
	}
	_ = s.saveJSON("users.json", s.users)
	_ = s.saveJSON("sessions.json", s.sessions)
	return nil
}

func (s *Store) PutSession(sess Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sess.Token] = sess
	if s.sb == nil {
		return s.saveJSON("sessions.json", s.sessions)
	}
	return nil
}

func (s *Store) GetSession(token string) (Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[token]
	return sess, ok
}

func (s *Store) DeleteSession(token string) {
	s.mu.Lock()
	delete(s.sessions, token)
	if s.sb == nil {
		_ = s.saveJSON("sessions.json", s.sessions)
	}
	s.mu.Unlock()
}

func (s *Store) PutBusiness(b Business) error {
	if s.sb != nil {
		if other, ok, err := s.sb.GetBusinessBySlug(b.Slug); err == nil && ok && other.ID != b.ID {
			return fmt.Errorf("slug already taken")
		}
		return s.sb.UpsertBusiness(b)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if old, ok := s.byID[b.ID]; ok && old.Slug != b.Slug {
		delete(s.bySlug, strings.ToLower(old.Slug))
	}
	slug := strings.ToLower(b.Slug)
	if other, ok := s.bySlug[slug]; ok && other != b.ID {
		return fmt.Errorf("slug already taken")
	}
	s.byID[b.ID] = b
	s.bySlug[slug] = b.ID
	return s.saveJSON("businesses.json", s.byID)
}

func (s *Store) GetBusiness(id string) (Business, bool) {
	if s.sb != nil {
		b, ok, err := s.sb.GetBusiness(id)
		if err != nil {
			log.Printf("supabase GetBusiness: %v", err)
			return Business{}, false
		}
		return b, ok
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.byID[id]
	return b, ok
}

func (s *Store) GetBySlug(slug string) (Business, bool) {
	if s.sb != nil {
		b, ok, err := s.sb.GetBusinessBySlug(slug)
		if err != nil {
			log.Printf("supabase GetBySlug: %v", err)
			return Business{}, false
		}
		return b, ok
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.bySlug[strings.ToLower(slug)]
	if !ok {
		return Business{}, false
	}
	b, ok := s.byID[id]
	return b, ok
}

func (s *Store) ListByOwner(userID string) []Business {
	if s.sb != nil {
		list, err := s.sb.ListBusinessesByOwner(userID)
		if err != nil {
			log.Printf("supabase ListByOwner: %v", err)
			return nil
		}
		return list
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []Business{}
	for _, b := range s.byID {
		if b.OwnerUserID == userID {
			out = append(out, b)
		}
	}
	return out
}

func (s *Store) DeleteBusiness(id string) error {
	if s.sb != nil {
		_ = s.sb.SetProducts(id, nil)
		return s.sb.DeleteBusiness(id)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if b, ok := s.byID[id]; ok {
		delete(s.bySlug, strings.ToLower(b.Slug))
		delete(s.byID, id)
		delete(s.products, id)
		delete(s.reviews, id)
		delete(s.stats, id)
		_ = s.saveJSON("businesses.json", s.byID)
		_ = s.saveJSON("products.json", s.products)
		_ = s.saveJSON("reviews.json", s.reviews)
		_ = s.saveJSON("stats.json", s.stats)
	}
	return nil
}

func (s *Store) SetProducts(businessID string, list []Product) error {
	if s.sb != nil {
		return s.sb.SetProducts(businessID, list)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.products[businessID] = list
	return s.saveJSON("products.json", s.products)
}

func (s *Store) GetProducts(businessID string) []Product {
	if s.sb != nil {
		list, err := s.sb.GetProducts(businessID)
		if err != nil {
			log.Printf("supabase GetProducts: %v", err)
			return nil
		}
		return list
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Product(nil), s.products[businessID]...)
}

func (s *Store) AddReview(r Review) error {
	if s.sb != nil {
		return s.sb.AddReview(r)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reviews[r.BusinessID] = append(s.reviews[r.BusinessID], r)
	return s.saveJSON("reviews.json", s.reviews)
}

func (s *Store) GetReviews(businessID string) []Review {
	if s.sb != nil {
		list, err := s.sb.GetReviews(businessID)
		if err != nil {
			log.Printf("supabase GetReviews: %v", err)
			return nil
		}
		return list
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Review(nil), s.reviews[businessID]...)
}

func (s *Store) IncrStat(businessID, kind string) {
	if s.sb != nil {
		if err := s.sb.IncrStat(businessID, kind); err != nil {
			log.Printf("supabase IncrStat: %v", err)
		}
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.stats[businessID]
	switch kind {
	case "profile_viewed":
		st.ProfileViews++
	case "catalog_viewed":
		st.CatalogViews++
	case "product_viewed":
		st.ProductViews++
	case "whatsapp_clicked":
		st.WhatsAppClicks++
	case "qr_scanned":
		st.QRScans++
	case "business_shared":
		st.Shares++
	}
	s.stats[businessID] = st
	_ = s.saveJSON("stats.json", s.stats)
}

func (s *Store) GetStats(businessID string) Stats {
	if s.sb != nil {
		st, err := s.sb.GetStats(businessID)
		if err != nil {
			log.Printf("supabase GetStats: %v", err)
			return Stats{}
		}
		return st
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stats[businessID]
}
