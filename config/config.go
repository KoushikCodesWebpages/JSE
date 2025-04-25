package config

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

// InitializeDatabase creates the SQLite database with LinkedIn and Xing tables
func InitializeDatabase() (*sql.DB, error) {
	dbFile := "JSE.db"
	dbExists := fileExists(dbFile)

	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		return nil, err
	}

	if dbExists {
		// fmt.Println("📂 Existing DB found, dropping selected tables...")

		tablesToDrop := []string{
			// "linkedin_jobs",
			// "xing_jobs",
			// "linkedin_job_application_links",
			// "xing_job_application_links",
			// "linkedin_job_description",
			// "xing_job_description",

			// "linkedin_failed_jobs",
			// "xing_failed_jobs",
		}

		for _, table := range tablesToDrop {
			dropQuery := fmt.Sprintf("DROP TABLE IF EXISTS %s;", table)
			if _, err := db.Exec(dropQuery); err != nil {
				return nil, fmt.Errorf("❌ Failed to drop table %s: %v", table, err)
			}
			fmt.Printf("🗑️ Dropped table: %s\n", table)
		}
	} else {
		fmt.Println("🆕 No existing DB found. Creating from scratch...")
	}

	// SQL statements for table creation
		createLinkedInJobsTable := `
		CREATE TABLE IF NOT EXISTS linkedin_jobs (
			id TEXT PRIMARY KEY,
			jobid TEXT UNIQUE,  
			title TEXT,
			company TEXT,
			location TEXT,
			posted_date TEXT,
			link TEXT UNIQUE,  
			processed BOOLEAN
		);`

		createXingJobsTable := `
		CREATE TABLE IF NOT EXISTS xing_jobs (
			id TEXT PRIMARY KEY,
			jobid TEXT UNIQUE,  		
			title TEXT,
			company TEXT,
			location TEXT,
			posted_date TEXT,
			link TEXT UNIQUE,  
			processed BOOLEAN
		);`

		createLinkedInJobApplicationLinksTable := `
		CREATE TABLE IF NOT EXISTS linkedin_job_application_links (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			job_id TEXT,
			job_link TEXT UNIQUE,
			FOREIGN KEY (job_id) REFERENCES linkedin_jobs(id) ON DELETE CASCADE
		);`

		createXingJobApplicationLinksTable := `
		CREATE TABLE IF NOT EXISTS xing_job_application_links (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			job_id TEXT,
			job_link TEXT,
			UNIQUE(job_id, job_link),
			FOREIGN KEY (job_id) REFERENCES xing_jobs(id) ON DELETE CASCADE
		);`

		createLinkedInJobDescTable := `
		CREATE TABLE IF NOT EXISTS linkedin_job_description (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			job_id TEXT UNIQUE,
			job_link TEXT UNIQUE,
			job_description TEXT,
			job_type TEXT,
			skills TEXT,
			FOREIGN KEY (job_id) REFERENCES linkedin_job_application_links(id) ON DELETE CASCADE,
			FOREIGN KEY (job_link) REFERENCES linkedin_job_application_links(job_link) ON DELETE CASCADE
		);`
		
		createXingJobDescTable := `
		CREATE TABLE IF NOT EXISTS xing_job_description (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			job_id TEXT UNIQUE,
			job_link TEXT UNIQUE,
			job_description TEXT,
			job_type TEXT,
			skills TEXT,
			FOREIGN KEY (job_id) REFERENCES xing_job_application_links(id) ON DELETE CASCADE,
			FOREIGN KEY (job_link) REFERENCES xing_job_application_links(job_link) ON DELETE CASCADE
		);`
		










	createLinkedInFailedJobsTable := `
	CREATE TABLE IF NOT EXISTS linkedin_failed_jobs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		job_id TEXT,
		job_link TEXT UNIQUE,
		FOREIGN KEY (job_id) REFERENCES linkedin_jobs(id) ON DELETE CASCADE
	);`

	createXingFailedJobsTable := `
	CREATE TABLE IF NOT EXISTS xing_failed_jobs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		job_id TEXT,
		job_link TEXT UNIQUE,
		FOREIGN KEY (job_id) REFERENCES xing_jobs(id) ON DELETE CASCADE
	);`



	// Execute table creation queries
	for _, query := range []string{
		createLinkedInJobsTable,
		createXingJobsTable,
		createLinkedInFailedJobsTable,
		createXingFailedJobsTable,
		createLinkedInJobApplicationLinksTable,
		createXingJobApplicationLinksTable,
		createLinkedInJobDescTable,
		createXingJobDescTable,
	} {
		if _, err = db.Exec(query); err != nil {
			return nil, fmt.Errorf("❌ Failed to create table: %v", err)
		}
	}

	fmt.Println("✅ Database initialized successfully with separate LinkedIn & Xing tables")
	return db, nil
}

// fileExists checks if the DB file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
