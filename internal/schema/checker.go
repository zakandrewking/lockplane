package schema

import (
	"fmt"
)

func CheckSchema(path string) (reportJson string, err error) {
	// step 1, no db, parse the sql
	_, err = LoadSchema(path)
	if err != nil {
		return "", fmt.Errorf("could not load schema: %v", err)
	}

	// step 2, enrich the parser output

	// step 3, with db, run a diff and validate the results
	// if db is not available, include a warning
	// TODO surface the warning in vscode
	return "", nil
}
