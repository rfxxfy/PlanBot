package database

import (
	"database/sql"
	"log"
)

func closeRows(rows *sql.Rows) {
	if err := rows.Close(); err != nil {
		log.Printf("close rows: %v", err)
	}
}

func rollbackTx(tx *sql.Tx) {
	if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
		log.Printf("rollback tx: %v", err)
	}
}

func closeStmt(stmt *sql.Stmt) {
	if err := stmt.Close(); err != nil {
		log.Printf("close stmt: %v", err)
	}
}
