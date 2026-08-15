-- Vero · esquema Supabase (ejecutar UNA vez en SQL Editor)
-- IDs texto para compatibilidad con el backend actual (biz-xxx, usr-xxx)

CREATE TABLE IF NOT EXISTS vero_users (
    id TEXT PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    name TEXT,
    password_hash TEXT NOT NULL,
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS vero_businesses (
    id TEXT PRIMARY KEY,
    vero_id TEXT UNIQUE NOT NULL,
    slug TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    owner_user_id TEXT REFERENCES vero_users(id) ON DELETE CASCADE,
    phone TEXT,
    whatsapp TEXT,
    category TEXT,
    country TEXT,
    city TEXT,
    zone TEXT,
    hours TEXT,
    bio TEXT,
    logo_url TEXT,
    plan TEXT DEFAULT 'free',
    verification_level INTEGER DEFAULT 0,
    published BOOLEAN DEFAULT true,
    score INTEGER DEFAULT 0,
    review_count INTEGER DEFAULT 0,
    rating_avg DECIMAL(3,2) DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS vero_products (
    id TEXT PRIMARY KEY,
    business_id TEXT REFERENCES vero_businesses(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    price DECIMAL(10,2),
    currency TEXT DEFAULT 'USD',
    photo_url TEXT,
    active BOOLEAN DEFAULT true,
    sort_order INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS vero_reviews (
    id TEXT PRIMARY KEY,
    business_id TEXT REFERENCES vero_businesses(id) ON DELETE CASCADE,
    rating INTEGER CHECK (rating >= 1 AND rating <= 5),
    comment TEXT,
    verified_operation BOOLEAN DEFAULT false,
    status TEXT DEFAULT 'visible',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS vero_stats (
    business_id TEXT PRIMARY KEY REFERENCES vero_businesses(id) ON DELETE CASCADE,
    profile_views INTEGER DEFAULT 0,
    catalog_views INTEGER DEFAULT 0,
    product_views INTEGER DEFAULT 0,
    whatsapp_clicks INTEGER DEFAULT 0,
    qr_scans INTEGER DEFAULT 0,
    shares INTEGER DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_businesses_slug ON vero_businesses(slug);
CREATE INDEX IF NOT EXISTS idx_businesses_city ON vero_businesses(city);
CREATE INDEX IF NOT EXISTS idx_products_business ON vero_products(business_id);
CREATE INDEX IF NOT EXISTS idx_reviews_business ON vero_reviews(business_id);

ALTER TABLE vero_users ENABLE ROW LEVEL SECURITY;
ALTER TABLE vero_businesses ENABLE ROW LEVEL SECURITY;
ALTER TABLE vero_products ENABLE ROW LEVEL SECURITY;
ALTER TABLE vero_reviews ENABLE ROW LEVEL SECURITY;
ALTER TABLE vero_stats ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS "Public read businesses" ON vero_businesses;
DROP POLICY IF EXISTS "Public read products" ON vero_products;
DROP POLICY IF EXISTS "Public read reviews" ON vero_reviews;
DROP POLICY IF EXISTS "Public read stats" ON vero_stats;

CREATE POLICY "Public read businesses" ON vero_businesses FOR SELECT USING (true);
CREATE POLICY "Public read products" ON vero_products FOR SELECT USING (true);
CREATE POLICY "Public read reviews" ON vero_reviews FOR SELECT USING (status = 'visible');
CREATE POLICY "Public read stats" ON vero_stats FOR SELECT USING (true);

-- service_role bypasses RLS; app uses SUPABASE_KEY (secret)
