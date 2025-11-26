-- Examples of dangerous DELETE and TRUNCATE operations
-- These should all trigger validation errors

-- DELETE without WHERE clause
DELETE FROM users;

-- TRUNCATE TABLE
TRUNCATE TABLE sessions;

-- TRUNCATE multiple tables
TRUNCATE TABLE logs, events;
