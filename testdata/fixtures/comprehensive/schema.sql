-- Comprehensive test with various column types and constraints
CREATE TABLE test_table (
  id SERIAL PRIMARY KEY,
  name TEXT NOT NULL,
  email TEXT UNIQUE,
  age INTEGER,
  active BOOLEAN DEFAULT true,
  created_at TIMESTAMP DEFAULT NOW()
);

-- Additional index
CREATE INDEX idx_test_name ON test_table(name);
