// Example: Simple schema showing minimal syntax
// Use this as a starting point for your own schemas

package simple

import "github.com/lockplane/lockplane/schema"

schema.#Schema & {
	tables: [users]
}

users: schema.#Table & {
	name: "users"
	columns: [
		schema.#ID,
		schema.#Email,
		schema.#CreatedAt,
	]
}
