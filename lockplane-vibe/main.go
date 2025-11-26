package main

import (
	_ "github.com/lib/pq"
	_ "github.com/tursodatabase/libsql-client-go/libsql"
	_ "modernc.org/sqlite"

	"github.com/lockplane/lockplane/cmd"
)

func main() {
	cmd.Execute()
}
