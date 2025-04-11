package Linkedin

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"os"
	"context"
	"net/url"

	_ "github.com/denisenkom/go-mssqldb"
)

// Database credentials
const (
	server   = "raasdb.database.windows.net"
	port     = 1433
	user     = "RAAS"
	password = `c6"h1_qS)>Jye^eT` // Your actual password
	database = "RAAS"
)

var db *sql.DB

// Initialize the database connection with proper encoding
func initDB() error {
	// ✅ Encode password to prevent special character issues
	encodedPassword := url.QueryEscape(password)

	// ✅ Use proper connection string format
	connString := fmt.Sprintf("sqlserver://%s:%s@%s:%d?database=%s",
		user, encodedPassword, server, port, database)

	// 🔍 Debug: Print connection details (excluding password)
	fmt.Println("🔍 Debug: Attempting to connect with the following details:")
	fmt.Printf("  - Server: %s\n", server)
	fmt.Printf("  - Port: %d\n", port)
	fmt.Printf("  - User: %s\n", user)
	fmt.Printf("  - Database: %s\n", database)

	var err error
	db, err = sql.Open("sqlserver", connString)
	if err != nil {
		return fmt.Errorf("❌ Error creating connection pool: %v", err)
	}

	// 🔍 Debug: Testing database connection
	fmt.Println("🔍 Debug: Pinging the database...")

	ctx := context.Background()
	err = db.PingContext(ctx)
	if err != nil {
		return fmt.Errorf("❌ Database connection failed: %v", err)
	}

	fmt.Println("✅ Connected to Azure SQL successfully!")
	return nil
}

// Upload CSV to Azure SQL
func UploadCSVToAzure() error {
	// Ensure DB is initialized
	if db == nil {
		if err := initDB(); err != nil {
			return err
		}
	}

	// Open CSV file
	filePath := "/JSE/scraper/storage/Linkedin_joblinks.csv"
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("❌ Failed to open CSV file (%s): %v", filePath, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("❌ Failed to read CSV file: %v", err)
	}

	// Ensure the table exists
	checkTableSQL := `
	IF NOT EXISTS (SELECT * FROM sys.objects WHERE object_id = OBJECT_ID(N'JobApplications') AND type in (N'U'))
	CREATE TABLE JobApplications (
		ID INT IDENTITY(1,1) PRIMARY KEY,
		JobTitle NVARCHAR(255),
		ApplyLink NVARCHAR(1000)
	);`
	_, err = db.Exec(checkTableSQL)
	if err != nil {
		return fmt.Errorf("❌ Failed to ensure table exists: %v", err)
	}

	// Skip header row and insert records
	for _, row := range records[1:] {
		if len(row) < 2 {
			continue
		}
		title, link := row[0], row[1]

		_, err := db.Exec("INSERT INTO JobApplications (JobTitle, ApplyLink) VALUES (@p1, @p2)", title, link)
		if err != nil {
			log.Printf("❌ Failed to insert record (%s, %s): %v\n", title, link, err)
		} else {
			fmt.Printf("✅ Inserted: %s -> %s\n", title, link)
		}
	}

	return nil
}

// HTTP Handler for /postdb
func PostDBHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("📡 Received request to upload CSV to Azure SQL.")

	err := UploadCSVToAzure()
	if err != nil {
		http.Error(w, fmt.Sprintf("❌ Upload failed: %v", err), http.StatusInternalServerError)
		return
	}

	fmt.Println("✅ CSV successfully uploaded to Azure SQL.")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("✅ CSV uploaded to Azure SQL successfully."))
}