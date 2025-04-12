package Linkedin

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	//"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/cdproto/target"
)

// StopChrome closes the Chromium browser process
func StopChrome() {
	if chromeCmd != nil {
		fmt.Println("🛑 Closing Chromium...")
		if err := chromeCmd.Process.Kill(); err != nil {
			fmt.Printf("⚠️ Failed to close Chromium: %v\n", err)
		} else {
			fmt.Println("✅ Chromium closed successfully.")
		}
	}
}

func captureAndCloseNewTab(ctx context.Context, db *sql.DB, jobID string, existingTabs map[target.ID]struct{}) error {
	var newTabID target.ID
	var cleanURL string

	// Get all open tabs
	newTabs, err := chromedp.Targets(ctx)
	if err != nil {
		log.Printf("❌ Failed to get updated open tabs: %v\n", err)
		return err
	}

	// Find new tab that is NOT a LinkedIn page
	for _, t := range newTabs {
		if _, exists := existingTabs[t.TargetID]; !exists && t.Type == "page" && t.URL != "" && !strings.Contains(t.URL, "linkedin.com") {
			cleanURL = strings.TrimSpace(t.URL)
			if cleanURL == "" {
				continue
			}

			newTabID = t.TargetID
			break
		}
	}

	if newTabID == "" {
		log.Printf("⚠️ No new non-LinkedIn tab found.")
		return nil
	}

	// Create a context bound to the new tab
	tabCtx, cancel := chromedp.NewContext(ctx, chromedp.WithTargetID(newTabID))
	defer cancel()

	fmt.Println("🔍 Processing external application page:", cleanURL)

	// ✅ Store application link
	if err := StoreApplicationLink(db, jobID, cleanURL); err != nil {
		log.Printf("❌ Error storing application link in DB: %v\n", err)
	} else {
		fmt.Println("✅ Captured and stored application page:", cleanURL)
	}

	// 🧠 Scrape job description from the open tab

	// 🛑 Close the new tab
	err = chromedp.Run(tabCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		return target.CloseTarget(newTabID).Do(ctx)
	}))
	if err != nil {
		log.Printf("❌ Error closing tab: %v\n", err)
	} else {
		fmt.Println("✅ Successfully closed extra tab:", newTabID)
	}

	return nil
}


/// JobTemp struct for temporary processing (matches database columns)
type JobTemp struct {
	ID    string `json:"id"`    // Matches id TEXT in linkedin_jobs
	Title string `json:"title"` // Matches title TEXT
	Link  string `json:"link"`  // Matches link TEXT
}

// LoadJobLinksFromDB fetches job links from linkedin_jobs
func LoadJobLinksFromDB(db *sql.DB) (map[string][]JobTemp, error) {
	// Only load unprocessed jobs
	rows, err := db.Query(`
		SELECT id, title, link 
		FROM linkedin_jobs 
		WHERE processed IS NULL OR processed = FALSE
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to load job links: %w", err)
	}
	defer rows.Close()

	jobLinks := make(map[string][]JobTemp)

	for rows.Next() {
		var job JobTemp
		if err := rows.Scan(&job.ID, &job.Title, &job.Link); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		jobLinks[job.Title] = append(jobLinks[job.Title], job)
	}

	return jobLinks, nil
}


type Joblinks struct {
	ID    int    `json:"id"`    // Matches INTEGER PRIMARY KEY AUTOINCREMENT
	JobID string `json:"job_id"` // Matches job_id TEXT (foreign key)
	Link  string `json:"link"`   // Matches job_link TEXT
}

// StoreApplicationLink stores the application link in the database
func StoreApplicationLink(db *sql.DB, jobID, link string) error {
	_, err := db.Exec(`
        INSERT INTO linkedin_job_application_links (job_id, job_link) 
        VALUES (?, ?)`, jobID, link, // Fixed incorrect table & column name
	)
	if err != nil {
		log.Printf("❌ Failed to store application link in DB: %v\n", err)
		return err
	}

	fmt.Printf("✅ Stored application link: %s\n", link)
	return nil
}

type FailedJob struct {
	ID      int    `json:"id"`      // Matches INTEGER PRIMARY KEY AUTOINCREMENT
	JobID   string `json:"job_id"`  // Matches job_id TEXT
	JobLink string `json:"job_link"` // Matches job_link TEXT
}

// StoreFailedJob stores a failed job in the database
func StoreFailedJob(db *sql.DB, jobID, jobLink, reason string) error {
	_, err := db.Exec(`
        INSERT INTO linkedin_failed_jobs (job_id, job_link, reason) 
        VALUES (?, ?, ?)`, jobID, jobLink, reason,
	)
	if err != nil {
		log.Printf("❌ Failed to store failed job in DB: %v\n", err)
		return err
	}

	fmt.Printf("⚠️ Stored failed job: %s -> %s (Reason: %s)\n", jobID, jobLink, reason)
	return nil
}
