-- Examples of dangerous DROP operations
-- These should all trigger validation errors

-- DROP TABLE
DROP TABLE users;

-- DROP TABLE with CASCADE
DROP TABLE posts CASCADE;

-- DROP COLUMN
ALTER TABLE users DROP COLUMN email;
ALTER TABLE products DROP COLUMN price;
