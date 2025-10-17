-- Table focused on testing various index types
CREATE TABLE products (
  id SERIAL PRIMARY KEY,
  sku TEXT UNIQUE NOT NULL,
  name TEXT NOT NULL,
  category TEXT,
  price NUMERIC
);

-- Regular index
CREATE INDEX idx_products_category ON products(category);

-- Multi-column index
CREATE INDEX idx_products_category_price ON products(category, price);
