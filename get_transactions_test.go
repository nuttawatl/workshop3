package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	_ "modernc.org/sqlite"
)

func setupTestDBGetTransactions(dbName string) (*sql.DB, func(), error) {
	// Create an in-memory SQLite database for testing
	name := fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)
	db, err := sql.Open("sqlite", name)
	if err != nil {
		return nil, nil, err
	}

	// Create tables and migrate schema
	err = Migrate(db)
	if err != nil {
		return nil, nil, err
	}

	// Insert test data
	_, err = db.Exec(`
		INSERT INTO accounts (branch, account_number, type, account_name, balance, available_balance, currency)
		VALUES ('Main', '12345', 'Savings', 'John Doe', 1000, 1000, 'USD');
	`)
	if err != nil {
		return nil, nil, err
	}

	_, err = db.Exec(`
		INSERT INTO transactions (transaction_id, account_number, from_account, to_account, to_account_name, to_bank, type, amount, currency, note, transferred_at)
		VALUES ('txn1', '12345', '12345', '54321', 'Jane Doe', 'Bank B', 'Transfer', 100, 'USD', 'Payment for services', '2025-01-01 12:00:00');
		INSERT INTO transactions (transaction_id, account_number, from_account, to_account, to_account_name, to_bank, type, amount, currency, note, transferred_at)
		VALUES ('txn2', '12345', '12345', '54322', 'Jake Doe', 'Bank C', 'Transfer', 200, 'USD', 'Payment for goods', '2025-01-02 13:00:00');
	`)
	if err != nil {
		return nil, nil, err
	}

	// Return DB and cleanup function
	return db, func() {
		// Cleanup after tests
		err := db.Close()
		if err != nil {
			fmt.Println("Error closing DB:", err)
		}
	}, nil
}

func TestGetTransactions(t *testing.T) {
	t.Run("ValidAccount", func(t *testing.T) {
		// Setup test database
		db, cleanup, err := setupTestDBGetTransactions("get_transactions")
		assert.NoError(t, err)
		defer cleanup()

		// Initialize the handler with the test DB
		handler := &Handler{db: db}

		// Setup Gin router and routes
		r := gin.Default()
		r.GET("/account/:accountNumber/transactions", handler.GetTransactions)

		// Test GET /account/12345/transactions (Valid account with transactions)
		req, err := http.NewRequest(http.MethodGet, "/account/12345/transactions", nil)
		assert.NoError(t, err)

		// Record the response
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		// Assert the response
		assert.Equal(t, http.StatusOK, w.Code)
		expected := `[
		{
			"transactionId": "txn2",
			"accountNumber": "12345",
			"fromAccount": "12345",
			"toAccount": "54322",
			"toAccountName": "Jake Doe",
			"toBank": "Bank C",
			"type": "Transfer",
			"amount": 200,
			"currency": "USD",
			"note": "Payment for goods",
			"transferredAt": "2025-01-02T13:00:00Z"
		},
		{
			"transactionId": "txn1",
			"accountNumber": "12345",
			"fromAccount": "12345",
			"toAccount": "54321",
			"toAccountName": "Jane Doe",
			"toBank": "Bank B",
			"type": "Transfer",
			"amount": 100,
			"currency": "USD",
			"note": "Payment for services",
			"transferredAt": "2025-01-01T12:00:00Z"
		}
	]`
		assert.JSONEq(t, expected, w.Body.String())
	})

	t.Run("NoTransactions", func(t *testing.T) {
		// Setup test database
		db, cleanup, err := setupTestDBGetTransactions("no_transactions")
		assert.NoError(t, err)
		defer cleanup()

		// Initialize the handler with the test DB
		handler := &Handler{db: db}

		// Setup Gin router and routes
		r := gin.Default()
		r.GET("/account/:accountNumber/transactions", handler.GetTransactions)

		// Test GET /account/99999/transactions (Account with no transactions)
		req, err := http.NewRequest(http.MethodGet, "/account/99999/transactions", nil)
		assert.NoError(t, err)

		// Record the response
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		// Assert the response
		assert.Equal(t, http.StatusOK, w.Code)
		expected := `[]`
		assert.JSONEq(t, expected, w.Body.String())
	})

	t.Run("DBError", func(t *testing.T) {
		// Setup test database
		db, cleanup, err := setupTestDBGetTransactions("dberror")
		assert.NoError(t, err)
		defer cleanup()

		// Simulate a DB error by closing the connection before making the request
		db.Close()

		// Initialize the handler with the test DB
		handler := &Handler{db: db}

		// Setup Gin router and routes
		r := gin.Default()
		r.GET("/account/:accountNumber/transactions", handler.GetTransactions)

		// Test GET /account/12345/transactions (Simulating DB error)
		req, err := http.NewRequest(http.MethodGet, "/account/12345/transactions", nil)
		assert.NoError(t, err)

		// Record the response
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		// Assert the response
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		expected := `{"error":"unable to get transactions: sql: database is closed"}`
		assert.JSONEq(t, expected, w.Body.String())
	})
}
