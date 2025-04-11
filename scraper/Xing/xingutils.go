package Xing

import (
	"context"
	"database/sql"
	//"encoding/json"
	"fmt"
	"log"
	//"net/http"
	//"os/exec"
	//"runtime"
	//"strconv"
	"time"
	"strings"
	
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)


// JobTemp struct for temporary processing (matches database columns)
type JobTemp struct {
	ID    string `json:"id"`    // Matches id TEXT in linkedin_jobs
	Title string `json:"title"` // Matches title TEXT
	Link  string `json:"link"`  // Matches link TEXT
}

// LoadJobLinksFromDB fetches job links from linkedin_jobs
func LoadJobLinksFromDB(db *sql.DB) (map[string][]JobTemp, error) {
	rows, err := db.Query("SELECT id, title, link FROM xing_jobs")
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
type FailedJob struct {
	ID      int    `json:"id"`      // Matches INTEGER PRIMARY KEY AUTOINCREMENT
	JobID   string `json:"job_id"`  // Matches job_id TEXT
	JobLink string `json:"job_link"` // Matches job_link TEXT
}

// StoreFailedJob stores a failed job in the database
func StoreFailedJob(db *sql.DB, jobID, jobLink, reason string) error {
	_, err := db.Exec(`
        INSERT INTO xing_failed_jobs (job_id, job_link, reason) 
        VALUES (?, ?, ?)`, jobID, jobLink, reason,
	)
	if err != nil {
		log.Printf("❌ Failed to store failed job in DB: %v\n", err)
		return err
	}

	fmt.Printf("⚠️ Stored failed job: %s -> %s (Reason: %s)\n", jobID, jobLink, reason)
	return nil
}

func navigateAndClickApply(ctx context.Context, db *sql.DB, jobID string, jobLink string) error {
	// Step 1: Navigate to the job page
	err := chromedp.Run(ctx,
		chromedp.Navigate(jobLink),
		chromedp.WaitVisible(`div.main-actions__ActionsContainer-sc-68c89ebb-0`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
	)
	if err != nil {
		log.Printf("❌ Failed to navigate or wait for container: %s -> %v\n", jobID, err)
		StoreFailedJob(db, jobID, jobLink, "Navigation or container wait failed")
		return err
	}

	// Step 2: Try clicking the button normally
	chromedp.Run(ctx,
		chromedp.Click(`div.main-actions__ActionsContainer-sc-68c89ebb-0 button[data-testid="apply-button"]`, chromedp.NodeVisible),
		chromedp.Sleep(5*time.Second),
	)

	// if err != nil {
	// 	log.Printf("⚠️ Normal click failed, trying JS click for jobID %s\n", jobID)

	// 	// Step 3: Fallback — click via JavaScript if normal click fails
	// 	jsClick := `
	// 		(() => {
	// 			const container = document.querySelector('div.main-actions__ActionsContainer-sc-68c89ebb-0');
	// 			if (container) {
	// 				const button = container.querySelector('button[data-testid="apply-button"]');
	// 				if (button) button.click();
	// 			}
	// 		})()
	// 	`
	// 	err = chromedp.Run(ctx,
	// 		chromedp.Evaluate(jsClick, nil),
	// 		chromedp.Sleep(5*time.Second),
	// 	)
	// 	if err != nil {
	// 		log.Printf("❌ JS click also failed for jobID %s: %v\n", jobID, err)
	// 		StoreFailedJob(db, jobID, jobLink, "Apply button JS click failed")
	// 		return err
	// 	}
	// }

	return nil
}


func captureAndCloseNewTab(ctx context.Context, db *sql.DB, jobID string, existingTabs map[target.ID]struct{}) error {
	var newTabID target.ID
	seen := make(map[string]bool)

	newTabs, err := chromedp.Targets(ctx)
	if err != nil {
		log.Printf("❌ Failed to get updated open tabs: %v\n", err)
		return err
	}

	for _, t := range newTabs {
		if _, exists := existingTabs[t.TargetID]; !exists && t.Type == "page" && t.URL != "" && !strings.Contains(t.URL, "xing.com") {
			cleanURL := strings.TrimSpace(t.URL)
			if cleanURL == "" || seen[cleanURL] {
				continue
			}
			seen[cleanURL] = true

			if err := StoreApplicationLink(db, jobID, cleanURL); err != nil {
				log.Printf("❌ Error storing application link in DB: %v\n", err)
			} else {
				fmt.Println("✅ Captured and stored application page:", cleanURL)
			}

			newTabID = t.TargetID
			break
		}
	}

	if newTabID != "" {
		tabCtx, cancel := chromedp.NewContext(ctx, chromedp.WithTargetID(newTabID))
		defer cancel()

		err := chromedp.Run(tabCtx, chromedp.ActionFunc(func(ctx context.Context) error {
			return target.CloseTarget(newTabID).Do(ctx)
		}))
		if err != nil {
			log.Printf("❌ Error closing tab: %v\n", err)
			return err
		} else {
			fmt.Println("✅ Successfully closed extra tab:", newTabID)
		}
	}

	return nil
}




type Joblinks struct {
	ID    int    `json:"id"`    // Matches INTEGER PRIMARY KEY AUTOINCREMENT
	JobID string `json:"job_id"` // Matches job_id TEXT (foreign key)
	Link  string `json:"link"`   // Matches job_link TEXT
}

// StoreApplicationLink stores the application link in the database
func StoreApplicationLink(db *sql.DB, jobID, link string) error {
	_, err := db.Exec(`
        INSERT INTO xing_job_application_links (job_id, job_link) 
        VALUES (?, ?)`, jobID, link, // Fixed incorrect table & column name
	)
	if err != nil {
		log.Printf("❌ Failed to store application link in DB: %v\n", err)
		return err
	}

	fmt.Printf("✅ Stored application link: %s\n", link)
	return nil
}
