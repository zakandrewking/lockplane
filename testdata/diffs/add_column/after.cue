// Schema fixture for add_column test
package add_column

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
					nullable:       true
					is_primary_key: false
				},
			]
			indexes: []
		}
	]
}
