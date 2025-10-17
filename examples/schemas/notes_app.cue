// Example: Note-taking app schema
// This shows basic table definitions with relationships

package notesapp

import "github.com/lockplane/lockplane/schema"

// Full schema export
schema.#Schema & {
	tables: [users, notes, tags, note_tags]
}

// Users table
users: schema.#Table & {
	name: "users"
	columns: [
		{
			name: "id"
			type: "integer"
			nullable: false
			is_primary_key: true
			default: "nextval('users_id_seq'::regclass)"
		},
		schema.#Email,
		schema.#CreatedAt,
	]
	indexes: [
		{
			name: "users_email_key"
			columns: ["email"]
			unique: true
		},
	]
}

// Notes table
notes: schema.#Table & {
	name: "notes"
	columns: [
		{
			name: "id"
			type: "integer"
			nullable: false
			is_primary_key: true
			default: "nextval('notes_id_seq'::regclass)"
		},
		{
			name: "user_id"
			type: "integer"
			nullable: false
		},
		{
			name: "title"
			type: "text"
			nullable: false
		},
		{
			name: "content"
			type: "text"
			nullable: true
		},
		schema.#CreatedAt,
		schema.#UpdatedAt,
	]
	indexes: [
		{
			name: "idx_notes_user_id"
			columns: ["user_id"]
			unique: false
		},
	]
}

// Tags table
tags: schema.#Table & {
	name: "tags"
	columns: [
		{
			name: "id"
			type: "integer"
			nullable: false
			is_primary_key: true
			default: "nextval('tags_id_seq'::regclass)"
		},
		{
			name: "name"
			type: "text"
			nullable: false
		},
	]
	indexes: [
		{
			name: "tags_name_key"
			columns: ["name"]
			unique: true
		},
	]
}

// Note-Tags junction table (many-to-many)
note_tags: schema.#Table & {
	name: "note_tags"
	columns: [
		{
			name: "note_id"
			type: "integer"
			nullable: false
			is_primary_key: true
		},
		{
			name: "tag_id"
			type: "integer"
			nullable: false
			is_primary_key: true
		},
	]
	indexes: [
		{
			name: "idx_note_tags_tag_id"
			columns: ["tag_id"]
			unique: false
		},
	]
}
