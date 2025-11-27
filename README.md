# Lockplane

Easy postgres schema management.

## 1. Install

```bash
go install github.com/lockplane/lockplane
```

## 2. Create a config file

```toml
[environments]
[environments.local]
database_url = "postgresql://postgres:postgres@localhost:5432/postgres"
```

## 3. Apply changes

```bash
npx lockplane apply
```
