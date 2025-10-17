// Migration plan: Create posts table
package plans

import "github.com/lockplane/lockplane/schema"

schema.#Plan & {
	steps: [
		{
			description: "Create posts table"
			sql:         "CREATE TABLE posts (id SERIAL PRIMARY KEY, title TEXT NOT NULL, content TEXT, created_at TIMESTAMP DEFAULT NOW())"
		},
	]
}
