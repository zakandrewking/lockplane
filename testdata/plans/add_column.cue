// Migration plan: Add email column to users table
package plans

import "github.com/lockplane/lockplane/schema"

schema.#Plan & {
	steps: [
		{
			description: "Add email column to users table"
			sql:         "ALTER TABLE users ADD COLUMN email TEXT"
		},
		{
			description: "Add unique constraint to email"
			sql:         "ALTER TABLE users ADD CONSTRAINT users_email_key UNIQUE (email)"
		},
	]
}
