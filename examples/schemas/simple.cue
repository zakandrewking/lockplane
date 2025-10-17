// Example: Simple PostgreSQL schema showing minimal syntax
// Use this as a starting point for your own schemas

package simple

import postgres "github.com/lockplane/lockplane/schema/postgres"

postgres.#Schema & {
	tables: [users]
}

users: postgres.#Table & {
	name: "users"
	columns: [
		postgres.#ID,
		postgres.#Email,
		postgres.#CreatedAt,
	]
}
