// Expected output for basic schema test
package basic

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
					default:        "nextval('users_id_seq'::regclass)"
					is_primary_key: true
				},
				{
					name:           "name"
					type:           "text"
					nullable:       false
					is_primary_key: false
				},
				{
					name:           "email"
					type:           "text"
					nullable:       true
					is_primary_key: false
				},
			]
			indexes: [
				{
					name:    "users_email_key"
					columns: []
					unique:  true
				},
				{
					name:    "users_pkey"
					columns: []
					unique:  true
				},
			]
		},
	]
}
