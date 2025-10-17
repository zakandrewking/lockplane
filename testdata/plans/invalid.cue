// Migration plan: Invalid SQL for testing error handling
package plans

import "github.com/lockplane/lockplane/schema"

schema.#Plan & {
	steps: [
		{
			description: "This should fail - invalid SQL"
			sql:         "INVALID SQL STATEMENT HERE"
		},
	]
}
