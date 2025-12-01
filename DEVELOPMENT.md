# Lockplane Development

```bash
# install
go install .

# test
go test ./...

# test with postgres
POSTGRES_URL=postgres://lockplane:lockplane@localhost:5432/lockplane go test -v ./...
# make sure tests like TestDriver_GetTables are not skipped
```
