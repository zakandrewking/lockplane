// Demo: After schema (with new posts table and age column)
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
				{
					name:     "age"
					type:     "integer"
					nullable: true
				},
			]
			indexes: [
				{
					name:    "idx_users_email"
					columns: ["email"]
					unique:  true
				},
			]
		},
		{
			name: "posts"
			columns: [
				{
					name:           "id"
					type:           "integer"
					nullable:       false
					is_primary_key: true
				},
				{
					name:     "user_id"
					type:     "integer"
					nullable: false
				},
				{
					name:     "title"
					type:     "text"
					nullable: false
				},
				{
					name:     "content"
					type:     "text"
					nullable: true
				},
			]
			indexes: []
		},
	]
}
