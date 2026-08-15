package vero

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// SupabaseClient talks to PostgREST with the service role key (bypasses RLS).
type SupabaseClient struct {
	base   string
	key    string
	client *http.Client
}

func NewSupabaseClient(baseURL, key string) *SupabaseClient {
	baseURL = strings.TrimRight(baseURL, "/")
	return &SupabaseClient{
		base:   baseURL + "/rest/v1",
		key:    key,
		client: &http.Client{Timeout: 20 * time.Second},
	}
}

func SupabaseFromEnv() *SupabaseClient {
	u := os.Getenv("SUPABASE_URL")
	k := os.Getenv("SUPABASE_KEY")
	if u == "" || k == "" {
		return nil
	}
	return NewSupabaseClient(u, k)
}

func (c *SupabaseClient) headers(extra map[string]string) http.Header {
	h := http.Header{}
	h.Set("apikey", c.key)
	h.Set("Authorization", "Bearer "+c.key)
	h.Set("Content-Type", "application/json")
	h.Set("Prefer", "return=representation")
	for k, v := range extra {
		h.Set(k, v)
	}
	return h
}

func (c *SupabaseClient) do(method, path string, body interface{}, extraHeaders map[string]string) ([]byte, int, error) {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.base+path, rdr)
	if err != nil {
		return nil, 0, err
	}
	req.Header = c.headers(extraHeaders)
	res, err := c.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, res.StatusCode, err
	}
	if res.StatusCode >= 400 {
		return data, res.StatusCode, fmt.Errorf("supabase %s %s: %d %s", method, path, res.StatusCode, string(data))
	}
	return data, res.StatusCode, nil
}

// --- Users ---

type sbUser struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	Name         string    `json:"name"`
	PasswordHash string    `json:"password_hash"`
	Active       bool      `json:"active"`
	CreatedAt    time.Time `json:"created_at"`
}

func (c *SupabaseClient) UpsertUser(u User) error {
	row := sbUser{
		ID: u.ID, Email: u.Email, Name: u.Name,
		PasswordHash: u.PasswordHash, Active: u.Active, CreatedAt: u.CreatedAt,
	}
	_, _, err := c.do("POST", "/vero_users", row, map[string]string{
		"Prefer": "resolution=merge-duplicates,return=minimal",
	})
	return err
}

func (c *SupabaseClient) GetUserByEmail(email string) (User, bool, error) {
	q := "/vero_users?email=eq." + url.QueryEscape(strings.ToLower(email)) + "&limit=1"
	data, _, err := c.do("GET", q, nil, nil)
	if err != nil {
		return User{}, false, err
	}
	var rows []sbUser
	if err := json.Unmarshal(data, &rows); err != nil || len(rows) == 0 {
		return User{}, false, nil
	}
	r := rows[0]
	return User{
		ID: r.ID, Email: r.Email, Name: r.Name,
		PasswordHash: r.PasswordHash, Active: r.Active, CreatedAt: r.CreatedAt,
	}, true, nil
}

func (c *SupabaseClient) GetUserByID(id string) (User, bool, error) {
	q := "/vero_users?id=eq." + url.QueryEscape(id) + "&limit=1"
	data, _, err := c.do("GET", q, nil, nil)
	if err != nil {
		return User{}, false, err
	}
	var rows []sbUser
	if err := json.Unmarshal(data, &rows); err != nil || len(rows) == 0 {
		return User{}, false, nil
	}
	r := rows[0]
	return User{
		ID: r.ID, Email: r.Email, Name: r.Name,
		PasswordHash: r.PasswordHash, Active: r.Active, CreatedAt: r.CreatedAt,
	}, true, nil
}

func (c *SupabaseClient) DeleteUser(id string) error {
	_, _, err := c.do("DELETE", "/vero_users?id=eq."+url.QueryEscape(id), nil, map[string]string{"Prefer": "return=minimal"})
	return err
}

// --- Businesses ---

type sbBusiness struct {
	ID                string    `json:"id"`
	VeroID            string    `json:"vero_id"`
	Slug              string    `json:"slug"`
	Name              string    `json:"name"`
	OwnerUserID       string    `json:"owner_user_id"`
	Phone             string    `json:"phone,omitempty"`
	WhatsApp          string    `json:"whatsapp,omitempty"`
	Category          string    `json:"category,omitempty"`
	Country           string    `json:"country,omitempty"`
	City              string    `json:"city,omitempty"`
	Zone              string    `json:"zone,omitempty"`
	Hours             string    `json:"hours,omitempty"`
	Bio               string    `json:"bio,omitempty"`
	LogoURL           string    `json:"logo_url,omitempty"`
	Plan              string    `json:"plan"`
	VerificationLevel int       `json:"verification_level"`
	Published         bool      `json:"published"`
	Score             int       `json:"score"`
	ReviewCount       int       `json:"review_count"`
	RatingAvg         float64   `json:"rating_avg"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func businessToSB(b Business) sbBusiness {
	return sbBusiness{
		ID: b.ID, VeroID: b.ZyrID, Slug: b.Slug, Name: b.Name,
		OwnerUserID: b.OwnerUserID, Phone: b.Phone, WhatsApp: b.WhatsApp,
		Category: b.Category, Country: b.Country, City: b.City, Zone: b.Zone,
		Hours: b.Hours, Bio: b.Bio, LogoURL: b.LogoURL, Plan: b.Plan,
		VerificationLevel: b.VerificationLevel, Published: b.Published,
		Score: b.Score, ReviewCount: b.ReviewCount, RatingAvg: b.RatingAvg,
		CreatedAt: b.CreatedAt, UpdatedAt: b.UpdatedAt,
	}
}

func sbToBusiness(r sbBusiness) Business {
	return Business{
		ID: r.ID, ZyrID: r.VeroID, Slug: r.Slug, Name: r.Name,
		OwnerUserID: r.OwnerUserID, Phone: r.Phone, WhatsApp: r.WhatsApp,
		Category: r.Category, Country: r.Country, City: r.City, Zone: r.Zone,
		Hours: r.Hours, Bio: r.Bio, LogoURL: r.LogoURL, Plan: r.Plan,
		VerificationLevel: r.VerificationLevel, Published: r.Published,
		Score: r.Score, ReviewCount: r.ReviewCount, RatingAvg: r.RatingAvg,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func (c *SupabaseClient) UpsertBusiness(b Business) error {
	_, _, err := c.do("POST", "/vero_businesses", businessToSB(b), map[string]string{
		"Prefer": "resolution=merge-duplicates,return=minimal",
	})
	return err
}

func (c *SupabaseClient) GetBusiness(id string) (Business, bool, error) {
	q := "/vero_businesses?id=eq." + url.QueryEscape(id) + "&limit=1"
	data, _, err := c.do("GET", q, nil, nil)
	if err != nil {
		return Business{}, false, err
	}
	var rows []sbBusiness
	if err := json.Unmarshal(data, &rows); err != nil || len(rows) == 0 {
		return Business{}, false, nil
	}
	return sbToBusiness(rows[0]), true, nil
}

func (c *SupabaseClient) GetBusinessBySlug(slug string) (Business, bool, error) {
	q := "/vero_businesses?slug=eq." + url.QueryEscape(strings.ToLower(slug)) + "&limit=1"
	data, _, err := c.do("GET", q, nil, nil)
	if err != nil {
		return Business{}, false, err
	}
	var rows []sbBusiness
	if err := json.Unmarshal(data, &rows); err != nil || len(rows) == 0 {
		return Business{}, false, nil
	}
	return sbToBusiness(rows[0]), true, nil
}

func (c *SupabaseClient) ListBusinessesByOwner(userID string) ([]Business, error) {
	q := "/vero_businesses?owner_user_id=eq." + url.QueryEscape(userID) + "&order=created_at.desc"
	data, _, err := c.do("GET", q, nil, nil)
	if err != nil {
		return nil, err
	}
	var rows []sbBusiness
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, err
	}
	out := make([]Business, 0, len(rows))
	for _, r := range rows {
		out = append(out, sbToBusiness(r))
	}
	return out, nil
}

func (c *SupabaseClient) DeleteBusiness(id string) error {
	_, _, err := c.do("DELETE", "/vero_businesses?id=eq."+url.QueryEscape(id), nil, map[string]string{"Prefer": "return=minimal"})
	return err
}

// --- Products ---

type sbProduct struct {
	ID         string    `json:"id"`
	BusinessID string    `json:"business_id"`
	Title      string    `json:"title"`
	Price      float64   `json:"price"`
	Currency   string    `json:"currency"`
	PhotoURL   string    `json:"photo_url,omitempty"`
	Active     bool      `json:"active"`
	SortOrder  int       `json:"sort_order"`
	CreatedAt  time.Time `json:"created_at"`
}

func (c *SupabaseClient) SetProducts(businessID string, list []Product) error {
	// replace all products for business
	_, _, err := c.do("DELETE", "/vero_products?business_id=eq."+url.QueryEscape(businessID), nil, map[string]string{"Prefer": "return=minimal"})
	if err != nil {
		return err
	}
	if len(list) == 0 {
		return nil
	}
	rows := make([]sbProduct, 0, len(list))
	for _, p := range list {
		rows = append(rows, sbProduct{
			ID: p.ID, BusinessID: p.BusinessID, Title: p.Title,
			Price: p.Price, Currency: p.Currency, PhotoURL: p.PhotoURL,
			Active: p.Active, SortOrder: p.Sort, CreatedAt: p.CreatedAt,
		})
	}
	_, _, err = c.do("POST", "/vero_products", rows, map[string]string{"Prefer": "return=minimal"})
	return err
}

func (c *SupabaseClient) GetProducts(businessID string) ([]Product, error) {
	q := "/vero_products?business_id=eq." + url.QueryEscape(businessID) + "&order=sort_order.asc,created_at.asc"
	data, _, err := c.do("GET", q, nil, nil)
	if err != nil {
		return nil, err
	}
	var rows []sbProduct
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, err
	}
	out := make([]Product, 0, len(rows))
	for _, r := range rows {
		out = append(out, Product{
			ID: r.ID, BusinessID: r.BusinessID, Title: r.Title,
			Price: r.Price, Currency: r.Currency, PhotoURL: r.PhotoURL,
			Active: r.Active, Sort: r.SortOrder, CreatedAt: r.CreatedAt,
		})
	}
	return out, nil
}

// --- Reviews ---

type sbReview struct {
	ID                string    `json:"id"`
	BusinessID        string    `json:"business_id"`
	Rating            int       `json:"rating"`
	Comment           string    `json:"comment,omitempty"`
	VerifiedOperation bool      `json:"verified_operation"`
	Status            string    `json:"status"`
	CreatedAt         time.Time `json:"created_at"`
}

func (c *SupabaseClient) AddReview(r Review) error {
	row := sbReview{
		ID: r.ID, BusinessID: r.BusinessID, Rating: r.Rating,
		Comment: r.Comment, VerifiedOperation: r.VerifiedOperation,
		Status: r.Status, CreatedAt: r.CreatedAt,
	}
	_, _, err := c.do("POST", "/vero_reviews", row, map[string]string{"Prefer": "return=minimal"})
	return err
}

func (c *SupabaseClient) GetReviews(businessID string) ([]Review, error) {
	q := "/vero_reviews?business_id=eq." + url.QueryEscape(businessID) + "&order=created_at.asc"
	data, _, err := c.do("GET", q, nil, nil)
	if err != nil {
		return nil, err
	}
	var rows []sbReview
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, err
	}
	out := make([]Review, 0, len(rows))
	for _, r := range rows {
		out = append(out, Review{
			ID: r.ID, BusinessID: r.BusinessID, Rating: r.Rating,
			Comment: r.Comment, VerifiedOperation: r.VerifiedOperation,
			Status: r.Status, CreatedAt: r.CreatedAt,
		})
	}
	return out, nil
}

// --- Stats ---

type sbStats struct {
	BusinessID     string `json:"business_id"`
	ProfileViews   int    `json:"profile_views"`
	CatalogViews   int    `json:"catalog_views"`
	ProductViews   int    `json:"product_views"`
	WhatsAppClicks int    `json:"whatsapp_clicks"`
	QRScans        int    `json:"qr_scans"`
	Shares         int    `json:"shares"`
}

func (c *SupabaseClient) GetStats(businessID string) (Stats, error) {
	q := "/vero_stats?business_id=eq." + url.QueryEscape(businessID) + "&limit=1"
	data, _, err := c.do("GET", q, nil, nil)
	if err != nil {
		return Stats{}, err
	}
	var rows []sbStats
	if err := json.Unmarshal(data, &rows); err != nil || len(rows) == 0 {
		return Stats{}, nil
	}
	r := rows[0]
	return Stats{
		ProfileViews: r.ProfileViews, CatalogViews: r.CatalogViews,
		ProductViews: r.ProductViews, WhatsAppClicks: r.WhatsAppClicks,
		QRScans: r.QRScans, Shares: r.Shares,
	}, nil
}

func (c *SupabaseClient) IncrStat(businessID, kind string) error {
	st, err := c.GetStats(businessID)
	if err != nil {
		return err
	}
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
	row := sbStats{
		BusinessID: businessID, ProfileViews: st.ProfileViews,
		CatalogViews: st.CatalogViews, ProductViews: st.ProductViews,
		WhatsAppClicks: st.WhatsAppClicks, QRScans: st.QRScans, Shares: st.Shares,
	}
	_, _, err = c.do("POST", "/vero_stats", row, map[string]string{
		"Prefer": "resolution=merge-duplicates,return=minimal",
	})
	return err
}

// Ping checks connectivity / schema readiness.
func (c *SupabaseClient) Ping() error {
	_, code, err := c.do("GET", "/vero_businesses?select=id&limit=1", nil, nil)
	if err != nil && code == 0 {
		return err
	}
	if code == 404 || (err != nil && strings.Contains(err.Error(), "PGRST205")) {
		return fmt.Errorf("tablas no creadas en Supabase (ejecuta el SQL de esquema)")
	}
	return nil
}
