# Lockplane — v0 Design Doc

## Summary
Lockplane is a thin, Postgres-first control plane that lets humans and AIs **plan, apply, and audit**
schema and data changes with **zero ambiguity**. The user API is an **MCP server** exposing a tiny set
of safe operators. Application traffic uses stock Postgres and optional PostgREST/PostGraphile APIs.

Lockplane ensures every database change is **deterministic, explainable, and reproducible** by
tracking a canonical catalog manifest, computing a cryptographic hash for each version, and
recording a ledger of all applied plans.

---

## Problem

- SQL migrations are often ambiguous and non-reproducible.
- AI or automation tools cannot safely reason about state or rollback.
- Developers want Firebase-like ease with relational guarantees.
- Users of tools like Lovable and V0 should not need to learn all the intricacies of database
  performance and safety

---

## Principles

- **Postgres is the source of truth** — no proprietary DB engine or forks.
- **Determinism everywhere** — each catalog state has a unique hash.
- **Human + AI co-editable** — DSL and APIs are machine-readable and human-readable.
- **Explainability first** — query plans, RLS policies, and changes can all be inspected.

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

## Migration Strategy

Two supported modes:

**A) pgroll (zero-downtime):**

**B) Transactional (simple dev):**

---

## Safety Model

- Strict SQL allow-list (via parser).
- Shadow DB dry-run validation.
- Advisory lock per apply.
- Two-phase confirmation for destructive ops.
- Durable execution with timeouts, retries, and rollbacks.
- Human- and machine-friendly error messages.


