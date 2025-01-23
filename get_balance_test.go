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

func setupTestDB() (*sql.DB, func(), error) {
	// Create an in-memory SQLite database for testing
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, nil, err
	}

	// Create tables
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
	`)
	if err != nil {
		return nil, nil, err
	}

	return db, func() {
		// Cleanup after tests
		err := db.Close()
		if err != nil {
			fmt.Println("Error closing DB:", err)
		}
	}, nil
}

func TestGetBalance(t *testing.T) {
	t.Run("ValidAccount", func(t *testing.T) {
		// Setup test database
		db, cleanup, err := setupTestDB()
		assert.NoError(t, err)
		defer cleanup()

		// Initialize the handler with the test DB
		handler := &Handler{db: db}

		// Setup Gin router and routes
		r := gin.Default()
		r.GET("/account/:accountNumber/balance", handler.GetBalance)

		// Test GET /account/:accountNumber/balance
		req, err := http.NewRequest(http.MethodGet, "/account/12345/balance", nil)
		assert.NoError(t, err)

		// Record the response
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		// Assert the response
		assert.Equal(t, http.StatusOK, w.Code)
		expected := `{"branch":"Main","number":"12345","type":"Savings","name":"John Doe","currentBalance":1000,"availableBalance":1000,"currency":"USD"}`
		assert.JSONEq(t, expected, w.Body.String())
	})
}

func TestGetAllTransactions(t *testing.T) {
	t.Run("ValidAccount", func(t *testing.T) {
		// Setup test database
		db, cleanup, err := setupTestDB()
		assert.NoError(t, err)
		defer cleanup()

		// Initialize the handler with the test DB
		handler := &Handler{db: db}

		// Setup Gin router and routes
		r := gin.Default()
		r.GET("/transactions", handler.GetAllTransactions)

		// Test GET /transactions
		req, err := http.NewRequest(http.MethodGet, "/transactions", nil)
		assert.NoError(t, err)

		// Record the response
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		// Assert the response
		assert.Equal(t, http.StatusOK, w.Code)
		expected := `[{"transactionId":"txn1","accountNumber":"12345","fromAccount":"12345","toAccount":"54321","toAccountName":"Jane Doe","toBank":"Bank B","type":"Transfer","amount":100,"currency":"USD","note":"Payment for services","transferredAt":"2025-01-01T12:00:00Z"}]`
		assert.JSONEq(t, expected, w.Body.String())
	})
}
