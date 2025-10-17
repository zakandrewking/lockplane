// Schema fixture for add_table test
package add_table

import "github.com/lockplane/lockplane/schema"

schema.#Schema & {
	tables: [
		{
			name: "users"
			columns: [
				{
					name:           "id"
					type:           "integer"
					nullable:       false
					is_primary_key: true
				},
			]
			indexes: []
		}
	]
}
