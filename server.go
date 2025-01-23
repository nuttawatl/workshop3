package main

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"demo/config"
	"demo/firebase"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	_ "modernc.org/sqlite"
)

type Account struct {
	Branch           string `json:"branch"`
	AccountNumber    string `json:"number"`
	AccountType      string `json:"type"`
	AccountName      string `json:"name"`
	Balance          int64  `json:"currentBalance"`
	AvailableBalance int64  `json:"availableBalance"`
	Currency         string `json:"currency"`
}

type Transaction struct {
	TransactionID string    `json:"transactionId"`
	AccountNumber string    `json:"accountNumber"`
	FromAccount   string    `json:"fromAccount"`
	ToAccount     string    `json:"toAccount"`
	ToAccountName string    `json:"toAccountName"`
	ToBank        string    `json:"toBank"`
	Type          string    `json:"type"`
	Amount        int64     `json:"amount"`
	Currency      string    `json:"currency"`
	Note          string    `json:"note"`
	TransferredAt time.Time `json:"transferredAt"`
}

type Schedule struct {
	ScheduleID    string    `json:"scheduleId"`
	FromAccount   string    `json:"fromAccount"`
	ToAccount     string    `json:"toAccount"`
	ToAccountName string    `json:"toAccountName"`
	ToBank        string    `json:"toBank"`
	Amount        int64     `json:"amount"`
	Note          string    `json:"note"`
	ScheduleDate  time.Time `json:"date"`
}

// Request & Response Structs
type TransferRequest struct {
	FromAccount string `json:"fromAccount" binding:"required"`
	ToAccount   string `json:"toAccount" binding:"required"`
	ToBank      string `json:"toBank" binding:"required"`
	Amount      int64  `json:"amount" binding:"required"`
	Currency    string `json:"currency" binding:"required"`
	Note        string `json:"note"`
}

type TransferResponse struct {
	TransactionID string `json:"transactionId"`
	Status        string `json:"status"`
	TransferredAt string `json:"transferredAt"`
}

type ScheduleRequest struct {
	FromAccount string `json:"fromAccount" binding:"required"`
	ToAccount   string `json:"toAccount" binding:"required"`
	ToBank      string `json:"toBank" binding:"required"`
	Amount      int64  `json:"amount" binding:"required"`
	Currency    string `json:"currency" binding:"required"`
	Note        string `json:"note"`
	Schedule    string `json:"schedule" binding:"required"` // "ONCE" or "MONTHLY"
	StartDate   string `json:"startDate" binding:"required"`
	EndDate     string `json:"endDate"`
}

type ScheduleResponse struct {
	ScheduleID   string `json:"scheduleId"`
	Status       string `json:"status"`
	NextRunDate  string `json:"nextRunDate"`
	EndDate      string `json:"endDate"`
	ScheduleType string `json:"scheduleType"`
}

type Handler struct {
	db *sql.DB
}

// Utility function to handle errors consistently
func handleError(c *gin.Context, err error, msg string) {
	if err != nil {
		log.Printf("Error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": msg})
	}
}

// Query for account details
func (h *Handler) getAccount(accountNo string) (*Account, error) {
	var account Account
	err := h.db.QueryRow(`
		SELECT branch, account_number, type, account_name, balance, available_balance, currency
		FROM accounts
		WHERE account_number = ?`, accountNo).Scan(
		&account.Branch, &account.AccountNumber, &account.AccountType, &account.AccountName,
		&account.Balance, &account.AvailableBalance, &account.Currency,
	)
	return &account, err
}

// GetBalance handler
func (h *Handler) GetBalance(c *gin.Context) {
	accountNo := c.Param("accountNumber")
	account, err := h.getAccount(accountNo)
	if err != nil {
		handleError(c, err, "unable to get account balance")
		return
	}

	c.JSON(http.StatusOK, account)
}

// Query for all transactions
func (h *Handler) getAllTransactions() ([]Transaction, error) {
	rows, err := h.db.Query(`
		SELECT transaction_id, account_number, from_account, to_account, to_account_name, to_bank,
			type, amount, currency, note, transferred_at
		FROM transactions ORDER BY transferred_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []Transaction
	for rows.Next() {
		var txn Transaction
		var transferredAt string
		err := rows.Scan(&txn.TransactionID, &txn.AccountNumber, &txn.FromAccount,
			&txn.ToAccount, &txn.ToAccountName, &txn.ToBank, &txn.Type, &txn.Amount,
			&txn.Currency, &txn.Note, &transferredAt)
		if err != nil {
			return nil, err
		}
		at, err := time.Parse("2006-01-02 15:04:05", transferredAt)
		if err != nil {
			return nil, err
		}
		txn.TransferredAt = at
		transactions = append(transactions, txn)
	}
	return transactions, nil
}

// GetAllTransactions handler
func (h *Handler) GetAllTransactions(c *gin.Context) {
	transactions, err := h.getAllTransactions()
	if err != nil {
		handleError(c, err, "unable to get transactions")
		return
	}

	c.JSON(http.StatusOK, transactions)
}

func (h *Handler) GetTransactions(c *gin.Context) {
	accountNo := c.Param("accountNumber")
	transactions, err := h.fetchTransactions(accountNo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, transactions)
}

// fetchTransactions fetches the transactions from the database and parses the transferred_at time
func (h *Handler) fetchTransactions(accountNo string) ([]Transaction, error) {
	rows, err := h.db.Query(`
        SELECT transaction_id, account_number, from_account, to_account, to_account_name, to_bank, type, amount, currency, note, transferred_at
        FROM transactions
        WHERE account_number = $1
        ORDER BY transferred_at DESC
    `, accountNo)
	if err != nil {
		return nil, fmt.Errorf("unable to get transactions: %w", err)
	}
	defer rows.Close()

	transactions := []Transaction{}
	for rows.Next() {
		var txn Transaction
		var transferredAt string
		if err := rows.Scan(&txn.TransactionID, &txn.AccountNumber, &txn.FromAccount, &txn.ToAccount, &txn.ToAccountName, &txn.ToBank, &txn.Type, &txn.Amount, &txn.Currency, &txn.Note, &transferredAt); err != nil {
			return nil, fmt.Errorf("unable to scan transaction: %w", err)
		}
		if err := parseTransferredAt(&txn, transferredAt); err != nil {
			return nil, fmt.Errorf("unable to parse transferred_at: %w", err)
		}
		transactions = append(transactions, txn)
	}
	return transactions, nil
}

// parseTransferredAt parses the transferred_at time and sets it on the transaction
func parseTransferredAt(txn *Transaction, transferredAt string) error {
	at, err := time.Parse("2006-01-02 15:04:05", transferredAt)
	if err != nil {
		return fmt.Errorf("invalid time format: %w", err)
	}
	txn.TransferredAt = at
	return nil
}

func (h *Handler) GetSchedules(c *gin.Context) {
	accountNo := c.Param("accountNumber")
	// query only  status = 'SCHEDULED'
	rows, err := h.db.Query(`
        SELECT schedule_id, from_account, to_account, to_account_name, to_bank, amount, note, schedule_date
        FROM schedules
        WHERE from_account = $1
        AND status = 'SCHEDULED'
        ORDER BY schedule_date ASC
        `, accountNo)
	if err != nil {
		log.Println(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to get schedules"})
		return
	}
	defer rows.Close()

	schedules := []Schedule{}
	// TODO: Implement the logic to fetch schedules from the database
	for rows.Next() {
		var sch Schedule
		var scheduleDate string
		if err := rows.Scan(&sch.ScheduleID, &sch.FromAccount, &sch.ToAccount, &sch.ToAccountName, &sch.ToBank, &sch.Amount, &sch.Note, &scheduleDate); err != nil {
			log.Println(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to get schedules"})
			return
		}

		layout := "2006-01-02" // Example format (adjust to match your actual date format)
		parsedDate, err := time.Parse(layout, scheduleDate)
		if err != nil {
			log.Printf("Failed to parse date %s: %v", scheduleDate, err)
			continue
		}
		sch.ScheduleDate = parsedDate
		schedules = append(schedules, sch)
	}

	c.JSON(http.StatusOK, schedules)
}

// Utility function to handle errors consistently
func handleScheduleError(c *gin.Context, err error, msg string) {
	if err != nil {
		log.Printf("Error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": msg})
	}
}

// Helper function to validate schedule dates
func validateScheduleDates(startDate, endDate string) error {
	format := "2006-01-02 15:04:05"
	if _, err := time.Parse(format, startDate); err != nil {
		return fmt.Errorf("invalid start date")
	}

	if endDate != "" {
		if _, err := time.Parse(format, endDate); err != nil {
			return fmt.Errorf("invalid end date")
		}
	}

	return nil
}

// Helper function to get account name by account number
func (h *Handler) getAccountName(accountNo string) (string, error) {
	var accountName string
	err := h.db.QueryRow(`
        SELECT account_name
        FROM accounts
        WHERE account_number = $1`, accountNo).Scan(&accountName)
	return accountName, err
}

// CreateSchedules handler
func (h *Handler) CreateSchedules(c *gin.Context) {
	fromAccount := c.Param("accountNumber")
	var req ScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleScheduleError(c, err, "invalid request body")
		return
	}

	// schedule type must be "ONCE" or "MONTHLY" only if not return error
	if req.Schedule != "ONCE" && req.Schedule != "MONTHLY" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid schedule type"})
		return
	}

	// Validate schedule dates
	if err := validateScheduleDates(req.StartDate, req.EndDate); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get recipient account name
	toAccountName, err := h.getAccountName(req.ToAccount)
	if err != nil {
		handleScheduleError(c, err, "unable to retrieve recipient account name")
		return
	}

	// Create schedule entry in the database
	schID := scheduleID()
	status := "SCHEDULED"
	_, err = h.db.Exec(`
		   INSERT INTO schedules (schedule_id, from_account, to_account, to_account_name, to_bank, amount, currency, note, status, schedule, schedule_date, end_date)
		   VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12);`,
		schID, fromAccount, req.ToAccount, toAccountName, req.ToBank, req.Amount, req.Currency, req.Note, status, req.Schedule, req.StartDate, req.EndDate)
	if err != nil {
		handleScheduleError(c, err, "unable to schedule transfer")
		return
	}
	// Send the response with schedule details
	resp := ScheduleResponse{
		ScheduleID:   schID,
		Status:       status,
		NextRunDate:  req.StartDate,
		EndDate:      req.EndDate,
		ScheduleType: req.Schedule,
	}

	c.JSON(http.StatusOK, resp)
}

func transactionID() string {
	rd := rand.New(rand.NewSource(time.Now().UnixNano()))
	return fmt.Sprintf("TXN%v", rd.Intn(1000000000))
}

// Utility function to handle errors consistently
func handleTransferError(c *gin.Context, err error, msg string) {
	if err != nil {
		log.Printf("Error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": msg})
	}
}

// Helper function to validate account balance
func (h *Handler) validateAccountBalance(accountNo string, amount int64) (int64, error) {
	var balance int64
	err := h.db.QueryRow(`
        SELECT balance
        FROM accounts
        WHERE account_number = $1`, accountNo).Scan(&balance)
	return balance, err
}

// Helper function to create a transaction
func (h *Handler) createTransaction(tx *sql.Tx, txID, fromAccount, toAccount, toAccountName, toBank, currency, note string, amount int64, stamp string) error {
	_, err := tx.Exec(`
        INSERT INTO transactions (transaction_id, account_number, from_account, to_account, to_account_name, to_bank, amount, currency, type, note, transferred_at)
        VALUES
            ($1, $2, $2, $3, $4, $5, $6, $7, $8, $9, $10),
            ($1, $3, $2, $3, $4, $5, $11, $7, $12, $9, $10)
        `,
		txID, fromAccount, toAccount, toAccountName, toBank, -amount, currency, "Transfer out", note, stamp, amount, "Transfer in")
	return err
}

// CreateTransfer handler
func (h *Handler) CreateTransfer(c *gin.Context) {
	fromAccount := c.Param("accountNumber")
	var req TransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate account balance
	balance, err := h.validateAccountBalance(fromAccount, req.Amount)
	if err != nil {
		handleTransferError(c, err, "unable to check account balance")
		return
	}
	if balance < req.Amount {
		c.JSON(http.StatusBadRequest, gin.H{"error": "insufficient balance"})
		return
	}

	// Get recipient account name
	toAccountName, err := h.getAccountName(req.ToAccount)
	if err != nil {
		handleTransferError(c, err, "unable to retrieve recipient account name")
		return
	}

	// Begin transaction
	tx, err := h.db.Begin()
	if err != nil {
		handleTransferError(c, err, "unable to create transfer")
		return
	}

	txID := transactionID()
	stamp := time.Now().Format("2006-01-02 15:04:05")

	// Create the transaction entries
	if err := h.createTransaction(tx, txID, fromAccount, req.ToAccount, toAccountName, req.ToBank, req.Currency, req.Note, req.Amount, stamp); err != nil {
		handleTransferError(c, err, "unable to create transaction")
		return
	}

	// Update the balance in the sender account
	_, err = tx.Exec(`
        UPDATE accounts
        SET balance = balance - $1
        WHERE account_number = $2`,
		req.Amount, fromAccount)
	if err != nil {
		handleTransferError(c, err, "unable to update sender balance")
		return
	}

	// Update the balance in the recipient account
	_, err = tx.Exec(`
        UPDATE accounts
        SET balance = balance + $1
        WHERE account_number = $2`,
		req.Amount, req.ToAccount)
	if err != nil {
		handleTransferError(c, err, "unable to update recipient balance")
		return
	}

	// Commit the transaction
	err = tx.Commit()
	if err != nil {
		handleTransferError(c, err, "unable to commit transfer")
		return
	}

	// Send the response with transaction details
	resp := TransferResponse{
		TransactionID: txID,
		Status:        "TRANSFERRED",
		TransferredAt: stamp,
	}

	c.JSON(http.StatusOK, resp)
}

func scheduleID() string {
	rd := rand.New(rand.NewSource(time.Now().UnixNano()))
	return fmt.Sprintf("SCH%v", rd.Intn(1000000000))
}

func main() {
	bankDB := "./bank.sqlite"
	// reset database
	err := os.Remove(bankDB)
	if err != nil {
		log.Println(err)
	}

	db, err := sql.Open("sqlite", bankDB)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	err = Migrate(db)
	if err != nil {
		log.Fatal(err)
	}

	err = Seed(db)
	if err != nil {
		log.Fatal(err)
	}

	conf := config.C()

	h := &Handler{db: db}

	port := "8080"
	if conf.PORT != "" {
		port = conf.PORT
	}

	router := setupRouter(h)
	// Start server
	router.Run(":" + port)
}

func setupRouter(h *Handler) *gin.Engine {
	router := gin.Default()
	router.Use(cors.Default())
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
	})

	// Health Check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Routes
	router.GET("/accounts/:accountNumber/balances", h.GetBalance)
	router.GET("/accounts/:accountNumber/transactions", h.GetTransactions)
	router.GET("/accounts/:accountNumber/schedules", h.GetSchedules)
	router.GET("/transactions", h.GetAllTransactions)

	router.POST("/accounts/:accountNumber/transfers", h.CreateTransfer)
	router.POST("/accounts/:accountNumber/schedules", h.CreateSchedules)

	router.GET("/features", func(c *gin.Context) {
		c.JSON(http.StatusOK, firebase.AllConfigs())
	})

	return router
}

func Migrate(db *sql.DB) error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS accounts (
            branch TEXT NOT NULL DEFAULT '',
            account_number TEXT PRIMARY KEY,
            type TEXT NOT NULL DEFAULT '',
            account_name TEXT NOT NULL DEFAULT '',
            balance INTEGER NOT NULL DEFAULT 0,
            available_balance INTEGER NOT NULL DEFAULT 0,
            currency TEXT NOT NULL DEFAULT 'THB'
        )`,
		`CREATE TABLE IF NOT EXISTS transactions (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            transaction_id TEXT NOT NULL,
            account_number TEXT NOT NULL,
            from_account TEXT NOT NULL,
            to_account TEXT NOT NULL,
            to_account_name TEXT NOT NULL DEFAULT '',
            to_bank TEXT,
            type TEXT NOT NULL DEFAULT '',
            amount INTEGER NOT NULL,
            currency TEXT NOT NULL,
            note TEXT,
            transferred_at TEXT NOT NULL
        )`,
		`CREATE TABLE IF NOT EXISTS schedules (
            schedule_id TEXT PRIMARY KEY,
            from_account TEXT NOT NULL,
            to_account TEXT NOT NULL,
            to_account_name TEXT NOT NULL DEFAULT '',
            to_bank TEXT,
            type TEXT NOT NULL DEFAULT '',
            amount INTEGER NOT NULL,
            currency TEXT NOT NULL,
            note TEXT,
            status TEXT DEFAULT 'scheduled',
            schedule TEXT NOT NULL,
            schedule_date TEXT NOT NULL,
            end_date TEXT
        )`,
	}

	for _, migration := range migrations {
		_, err := db.Exec(migration)
		if err != nil {
			return err
		}
	}

	return nil
}

func Seed(db *sql.DB) error {
	seeds := []string{
		`INSERT INTO accounts (branch, account_number, type, account_name, balance, available_balance, currency)
            VALUES
            ('Kalasin', '111-111-111', 'Savings', 'AnuchitO', 101282250, 101282250, 'THB'),
            ('KhonKean', '222-222-222', 'Savings', 'MaiThai', 96588150, 96588150, 'THB'),
            ('Bangkok', '333-333-333', 'Savings', 'LaumPlearn', 105500, 105500, 'THB'),
            ('Udon', '444-444-444', 'Savings', 'Laumcing', 199800, 199800, 'THB')
        ON CONFLICT (account_number) DO UPDATE
        SET balance = EXCLUDED.balance,
            available_balance = EXCLUDED.available_balance`,

		`INSERT INTO transactions (transaction_id, account_number, from_account, to_account, to_account_name, to_bank, amount, currency, type, note, transferred_at)
        VALUES
            ('TXN123456789', '111-111-111', '111-111-111', '222-222-222', 'MaiThai', 'KTB',  98982500, 'THB', 'Transfer in', 'Lunch', '2024-11-10 14:22:00'),
            ('TXN123456789', '222-222-222', '111-111-111', '222-222-222', 'MaiThai', 'KTB', -98982500, 'THB', 'Transfer out', 'Lunch', '2024-11-10 14:22:00'),
            ('TXN120456799', '111-111-111', '111-111-111', '444-444-444', 'Laumcing', 'KBank', -2300000, 'THB', 'Transfer out', 'Dinner', '2024-12-10 14:22:00'),
            ('TXN120456799', '444-444-444', '111-111-111', '444-444-444', 'Laumcing', 'KBank',  2300000, 'THB', 'Transfer in', 'Dinner', '2024-12-10 14:22:00'),
            ('TXN987634521', '111-111-111', '111-111-111', '333-333-333', 'LaumPlearn', 'SCB',  2499850, 'THB', 'Transfer in', 'Dinner', '2025-01-13 18:00:00'),
            ('TXN987634521', '333-333-333', '111-111-111', '333-333-333', 'LaumPlearn', 'SCB', -2499850, 'THB', 'Transfer out', 'Dinner', '2025-01-13 18:00:00'),
            ('TXN123416629', '111-111-111', '111-111-111', '222-222-222', 'MaiThai', 'KTB', -399900, 'THB', 'Transfer out', 'Breakfast', '2025-01-14 14:22:00'),
            ('TXN123416629', '222-222-222', '111-111-111', '222-222-222', 'MaiThai', 'KTB',  399900, 'THB', 'Transfer in', 'Breakfast', '2025-01-14 14:22:00'),
            ('TXN987654331', '222-222-222', '222-222-222', '333-333-333', 'LaumPlearn', 'SCB', -2394350, 'THB', 'Transfer out', 'Dinner', '2021-09-01 18:00:00'),
            ('TXN987654331', '333-333-333', '222-222-222', '333-333-333', 'LaumPlearn', 'SCB',  2394350, 'THB', 'Transfer in', 'Dinner', '2021-09-01 18:00:00'),
            ('TXN123434267', '111-111-111', '111-111-111', '444-444-444', 'Laumcing',  'KBank', -2499800, 'THB', 'Transfer out', 'Lunch', '2025-01-10 14:22:00'),
            ('TXN123456789', '444-444-444', '111-111-111', '444-444-444', 'Laumcing',  'KBank',  2499800, 'THB', 'Transfer in', 'Lunch', '2025-01-10 14:22:00')
        ON CONFLICT DO NOTHING`,

		`INSERT INTO schedules (schedule_id, from_account, to_account, to_account_name, to_bank, amount, currency, note, schedule, status, schedule_date, end_date)
        VALUES
            ('SCH123456789', '111-111-111', '222-222-222', 'MaiThai', 'KTB', -1899900, 'THB', 'Breakfast', 'ONCE', 'SCHEDULED', '2025-09-01 12:00:00', '2030-09-01 12:00:00'),
            ('SCH987654321', '111-111-111', '333-333-333', 'LaumPlearn', 'SCB', -2499850, 'THB', 'Lunch', 'ONCE', 'SCHEDULED', '2025-09-01 12:00:00', '2030-09-01 12:00:00'),
            ('SCH123434267', '111-111-111', '444-444-444', 'Laumcing', 'KBank', -2398825, 'THB', 'Dinner', 'ONCE', 'SCHEDULED', '2025-09-01 12:00:00', '2030-09-01 12:00:00')
        ON CONFLICT DO NOTHING`,
	}

	for _, seed := range seeds {
		_, err := db.Exec(seed)
		if err != nil {
			return err
		}
	}

	return nil
}
