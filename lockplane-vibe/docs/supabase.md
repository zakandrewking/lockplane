---
layout: default
title: Using Lockplane with Supabase
nav_order: 4
has_children: true
---

# Using Lockplane with Supabase

Supabase projects run on managed PostgreSQL. Lockplane gives you declarative schema control, safety checks, and reversible plans that complement Supabase's migration tooling. Run `lockplane init --supabase --yes` inside your Supabase repo to scaffold `supabase/schema/`, `.env.supabase`, and `schema_path = "supabase/schema"` automatically.

## Multi-Schema Support

Supabase projects use multiple PostgreSQL schemas to organize functionality:
- **`public`** - Your application tables
- **`storage`** - Supabase Storage buckets and objects
- **`auth`** - Authentication tables and functions

Lockplane supports managing tables and policies across all these schemas.

### Configuration

Add the schemas you want to manage to `lockplane.toml`:

```toml
default_environment = "local"
schema_path = "supabase/schema"
dialect = "postgres"
schemas = ["public", "storage", "auth"]  # Manage multiple schemas

[environments.local]
description = "Local Supabase development"
```

### Example: Storage Schema with RLS Policies

```sql
-- supabase/schema/storage_objects.lp.sql
CREATE TABLE storage.objects (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  bucket_id TEXT NOT NULL,
  name TEXT NOT NULL,
  owner UUID,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Enable RLS
ALTER TABLE storage.objects ENABLE ROW LEVEL SECURITY;

-- Public read access for specific bucket
CREATE POLICY "Public Access - Select" ON storage.objects
    FOR SELECT
    USING (bucket_id = 'public');

-- Owner has full access
CREATE POLICY "Owner Access - All" ON storage.objects
    FOR ALL
    USING (auth.uid() = owner);
```

When you run `lockplane introspect` or `lockplane apply`, Lockplane will:
- Query all specified schemas (`public`, `storage`, `auth`)
- Introspect RLS policies and include them in the schema
- Generate schema-qualified DDL (e.g., `CREATE TABLE storage.objects ...`)
- Manage policy changes (create, modify, drop)

### Row Level Security (RLS) Support

Lockplane fully supports RLS policies, introspecting:
- Policy name and type (PERMISSIVE or RESTRICTIVE)
- Command (SELECT, INSERT, UPDATE, DELETE, ALL)
- Roles the policy applies to
- USING clause (for SELECT, UPDATE, DELETE)
- WITH CHECK clause (for INSERT, UPDATE)

Policy changes are included in migration plans just like table changes.

## Choose Your Guide

**Starting fresh?**
→ [Using Lockplane with a New Supabase Project](supabase-new-project.md)

**Already have tables and data?**
→ [Using Lockplane with an Existing Supabase Project](supabase-existing-project.md)
