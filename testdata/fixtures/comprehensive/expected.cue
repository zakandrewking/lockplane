// Expected output for comprehensive schema test
package comprehensive

import "github.com/lockplane/lockplane/schema"

schema.#Schema & {
	tables: [
		{
			name: "test_table"
			columns: [
				{
					name:           "id"
					type:           "integer"
					nullable:       false
					default:        "nextval('test_table_id_seq'::regclass)"
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
				{
					name:           "age"
					type:           "integer"
					nullable:       true
					is_primary_key: false
				},
				{
					name:           "active"
					type:           "boolean"
					nullable:       true
					default:        "true"
					is_primary_key: false
				},
				{
					name:           "created_at"
					type:           "timestamp without time zone"
					nullable:       true
					default:        "now()"
					is_primary_key: false
				},
			]
			indexes: [
				{
					name:    "idx_test_name"
					columns: []
					unique:  false
				},
				{
					name:    "test_table_email_key"
					columns: []
					unique:  true
				},
				{
					name:    "test_table_pkey"
					columns: []
					unique:  true
				},
			]
		},
	]
}
