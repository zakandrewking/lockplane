// Demo: Before schema
package demo

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
					name:     "email"
					type:     "text"
					nullable: false
				},
			]
			indexes: []
		},
	]
}
