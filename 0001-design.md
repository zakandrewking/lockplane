# Lockplane — v0 Design Doc

## Summary
Lockplane is a thin, Postgres-first control plane that lets humans and AIs **plan, apply, and audit**
schema and data changes with **zero ambiguity**. The user API is an **MCP server** exposing a tiny set
of safe operators. Application traffic uses stock Postgres and optional PostgREST/PostGraphile APIs.

Lockplane ensures every database change is **deterministic, explainable, and reproducible** by
tracking a canonical catalog manifest, computing a cryptographic hash for each version, and
recording a ledger of all applied plans.

---

## Work Plan
- Anchor sprint charter around MCP-first surface and Postgres determinism goals so every deliverable ties back to the Summary and Principles sections.
- **M1 – DSL & Planner**: lock DSL syntax and schema coverage, build diff engine plus transactional executor, wire shadow-DB dry runs, and publish golden tests with a CLI demo.
- **M2 – pgroll Integration**: prototype dual-schema flow, script pgroll handoff, add rollback and timeout guards from the safety model, and document migration playbooks.
- **M3 – RLS & Policy Matrix**: implement `policy_evaluate` plus policy export, extend DSL RLS blocks, generate matrix reports, and dogfood with sample tenants.
- **M4 – Observability & Drift**: emit structured receipts, integrate OTLP hooks, schedule periodic drift diffs, and surface ledger hash verification inside the MCP server.
- Resolve open questions by running design spikes on enum/domain rollout, concurrent plan merging, and audit metadata, then feed outcomes into the relevant milestones before freeze.

---

## Problem
- SQL migrations are often ambiguous and non-reproducible.
- AI or automation tools cannot safely reason about DDL state or rollback.
- ORMs and serverless platforms obscure the actual database structure.
- Developers want Firebase/Supabase-like ease with relational guarantees.

---

## Principles
- **Postgres is the source of truth** — no proprietary DB engine or forks.
- **Determinism everywhere** — each catalog state has a unique hash.
- **Human + AI co-editable** — DSL and APIs are machine-readable and human-readable.
- **Explainability first** — query plans, RLS policies, and changes can all be inspected.

---

## MVP Scope
- Works with Postgres 14–16, with optional pgroll for zero-downtime changes.
- MCP server exposing 7 tools: `introspect_catalog`, `explain_sql`, `diff_catalog`,
  `apply_plan`, `apply_sql`, `policy_evaluate`, and `set_context`.
- DSL v0 (YAML/JSON) covering: tables, columns, PK/FK/unique/check, indexes, simple RLS,
  grants, and extensions.
- Catalog manifest with hash-chained ledger.
- Shadow DB for dry-runs and verification.

Out of scope: cross-database orchestration, sharding, online type rewrites, complex policy analysis.

---

## Architecture Overview
- **Postgres Core**: actual data store and single source of truth.
- **Lockplane Core**: introspector, planner/compiler, executor, and ledger.
- **MCP Server**: exposes Lockplane tools for AI and automation.
- **DSL Engine**: parses and serializes schema definitions.
- **Optional Gateway**: injects `app.user_id` and `app.tenant_id` for RLS contexts.

```text
          +--------------------------+
          |        MCP Client        |
          +-----------+--------------+
                      |
                      v
          +--------------------------+
          |   Lockplane MCP Server   |
          +-----------+--------------+
                      |
                      v
          +--------------------------+
          |     Lockplane Core       |
          | (Introspect, Plan, Exec) |
          +-----------+--------------+
                      |
                      v
              +---------------+
              |   Postgres    |
              +---------------+
```

---

## Interfaces (MCP Tools)

| Tool | Purpose | Output |
|------|----------|---------|
| `introspect_catalog` | Read schema → JSON manifest | `{ catalog, catalog_hash }` |
| `diff_catalog` | Compare DSL vs live schema | `{ plan, predicted_hash }` |
| `apply_plan` | Execute prebuilt plan | `{ resulting_hash, receipts[] }` |
| `apply_sql` | Safe SQL op (allow-listed) | `{ resulting_hash, receipt }` |
| `explain_sql` | Query plan insight | `{ plan_json }` |
| `policy_evaluate` | RLS check for given row | `{ predicate, verdict }` |
| `set_context` | Set role/user/tenant GUCs | `{ ok: true }` |

---

## DSL Example

```yaml
version: 1
extensions:
  - name: pgcrypto
    required: false
schemas:
  public:
    tables:
      users:
        id: users
        columns:
          - name: id
            type: uuid
            pk: true
            default: gen_random_uuid()
          - name: email
            type: text
            not_null: true
            unique: true
        grants:
          - role: app_reader
            privilege: select
          - role: app_writer
            privilege: insert_update
      todos:
        id: todos
        columns:
          - name: id
            type: uuid
            pk: true
            default: gen_random_uuid()
          - name: owner_id
            type: uuid
            not_null: true
            references:
              table: users
              columns: [id]
          - name: title
            type: text
          - name: done
            type: boolean
            default: false
        indexes:
          - name: ix_todos_owner
            columns: [owner_id]
        rls:
          enabled: true
          policies:
            - name: tenant_isolation
              for: all
              using: "tenant_id() = tenant_id"
```

---

## Migration Strategy

Two supported modes:

**A) pgroll (zero-downtime):**
1. `start` → creates dual schema
2. deploy new app (`SET search_path TO new_schema`)
3. `complete` → contract old schema

**B) Transactional (simple dev):**
- All safe DDL in one transaction
- Non-transactional ops (indexes, enums) run separately

---

## Safety Model
- Strict SQL allow-list (via parser).
- Shadow DB dry-run validation.
- Advisory lock per apply.
- Two-phase confirmation for destructive ops.
- Timeouts and retries for non-TX steps.

---

## Observability
- Each operation emits a JSON receipt: `{ input, steps, timing, hash }`
- Query plans via `EXPLAIN (FORMAT JSON)`.
- Policy matrix exported for all roles/tables.
- OpenTelemetry integration planned for v1.

---

## Security
- Roles: `lp_migrator`, `lp_runtime_read`, `lp_runtime_write`.
- Runtime isolation via RLS GUCs (`app.user_id`, `app.tenant_id`).
- No superuser privileges required.

---

## Milestones

| Milestone | Goals |
|------------|-------|
| **M0** | Introspect + Explain working (✅) |
| **M1** | DSL + diff planner, tx executor |
| **M2** | pgroll integration |
| **M3** | RLS evaluation + policy matrix |
| **M4** | Observability + drift detection |

---

## Open Questions
- Should enum and domain changes use custom online patterns or rely on pgroll?
- How do we merge concurrent human + AI plans?
- What’s the minimum metadata required in receipts for audit compliance?

---

## Tagline
**Lockplane — deterministic database change management for humans and AIs, on stock Postgres.**
