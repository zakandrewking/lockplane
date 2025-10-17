// Schema fixture for add_index test
package add_index

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
				{
					name:           "email"
					type:           "text"
					nullable:       false
					is_primary_key: false
				},
			]
			indexes: []
		}
	]
}
