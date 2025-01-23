package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	_ "modernc.org/sqlite"
)

// Setup test DB with initial transfers
func setupTestDBTransfers(dbName string) (*sql.DB, func(), error) {
	// Create an in-memory SQLite database for testing
	name := fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)
	db, err := sql.Open("sqlite", name)
	if err != nil {
		return nil, nil, err
	}

	// Create tables
	err = Migrate(db) // Assume Migrate() creates tables like accounts, transactions, etc.
	if err != nil {
		return nil, nil, err
	}

	// Insert initial data into accounts table
	_, err = db.Exec(`
		INSERT INTO accounts (branch, account_number, account_name, balance, currency)
		VALUES
			('Main', '12345', 'John Doe', 1000, 'USD'),
			('Main', '54321', 'Jane Doe', 500, 'USD');
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

func TestCreateTransfer(t *testing.T) {
	t.Run("ValidTransfer", func(t *testing.T) {
		// Setup test database
		db, cleanup, err := setupTestDBTransfers("transfer_db")
		assert.NoError(t, err)
		defer cleanup()

		// Initialize the handler with the test DB
		handler := &Handler{db: db}

		// Setup Gin router and routes
		r := gin.Default()
		r.POST("/accounts/:accountNumber/transfers", handler.CreateTransfer)

		// Prepare the request body
		reqBody := `{
			"fromAccount": "12345",
			"toAccount": "54321",
			"toBank": "Bank B",
			"amount": 200,
			"currency": "USD",
			"note": "Payment for services"
		}`

		// Test POST /transfer
		req, err := http.NewRequest(http.MethodPost, "/accounts/12345/transfers", strings.NewReader(reqBody))
		assert.NoError(t, err)

		// Record the response
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		// Assert the response
		assert.Equal(t, http.StatusOK, w.Code)

		resp := TransferResponse{}
		err = json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.NotEmpty(t, resp.TransactionID)
		assert.Equal(t, "TRANSFERRED", resp.Status)
		assert.NotEmpty(t, resp.TransferredAt)

		// Verify account balances after transfer
		var balance1, balance2 int64
		err = db.QueryRow("SELECT balance FROM accounts WHERE account_number = '12345'").Scan(&balance1)
		assert.NoError(t, err)
		err = db.QueryRow("SELECT balance FROM accounts WHERE account_number = '54321'").Scan(&balance2)
		assert.NoError(t, err)

		// Check if balances are updated correctly
		assert.Equal(t, int64(800), balance1) // Sender balance should decrease by 200
		assert.Equal(t, int64(700), balance2) // Receiver balance should increase by 200
	})

	t.Run("InsufficientBalance", func(t *testing.T) {
		// Setup test database
		db, cleanup, err := setupTestDBTransfers("insufficient_balance_db")
		assert.NoError(t, err)
		defer cleanup()

		// Initialize the handler with the test DB
		handler := &Handler{db: db}

		// Setup Gin router and routes
		r := gin.Default()
		r.POST("/accounts/:accountNumber/transfers", handler.CreateTransfer)

		// Prepare the request body with insufficient balance
		reqBody := `{
			"fromAccount": "12345",
			"toAccount": "54321",
			"toBank": "Bank B",
			"amount": 2000,
			"currency": "USD",
			"note": "Payment for services"
		}`

		// Test POST /transfer
		req, err := http.NewRequest(http.MethodPost, "/accounts/12345/transfers", strings.NewReader(reqBody))
		assert.NoError(t, err)

		// Record the response
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		// Assert the response
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.JSONEq(t, `{"error":"insufficient balance"}`, w.Body.String())
	})
}
