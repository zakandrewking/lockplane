// Example: Simple SQLite schema
// This demonstrates SQLite-specific types and conventions

package simple_sqlite

import sqlite "github.com/lockplane/lockplane/schema/sqlite"

sqlite.#Schema & {
	tables: [users]
}

users: sqlite.#Table & {
	name: "users"
	columns: [
		sqlite.#ID,
		sqlite.#Email,
		sqlite.#CreatedAt,
	]
}
