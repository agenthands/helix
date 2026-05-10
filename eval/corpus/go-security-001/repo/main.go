package main

import (
	"database/sql"
	"fmt"
)

// RunQuery executes a SELECT query for the given userID.
// WARNING: This uses string concatenation — SQL injection risk. Fix it.
func RunQuery(db *sql.DB, userID string) (*sql.Rows, error) {
	// VULNERABLE: do not concatenate user input into SQL.
	query := "SELECT id, name FROM users WHERE id = '" + userID + "'"
	return db.Query(query)
}

func main() {
	// placeholder: real usage would pass a *sql.DB.
	fmt.Println("RunQuery defined — see function body for SQL injection risk")
}
