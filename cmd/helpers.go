package cmd

import "fmt"

// printConfigNotFound prints a helpful message when lockplane.toml is not found
func printConfigNotFound() {
	fmt.Println(`lockplane.toml not found. Create one that looks like:

[environments.local]
postgres_url = "postgresql://postgres:postgres@localhost:5432/postgres"`)
}
