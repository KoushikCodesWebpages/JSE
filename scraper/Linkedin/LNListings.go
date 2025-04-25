package Linkedin

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
	"strconv"

	"github.com/chromedp/chromedp"
	"github.com/google/uuid"
)

// Job struct
type Job struct {
	UUID        string `json:"uuid"`
	JobID       int64  `json:"jobId"` // Added jobId
	Title       string `json:"title"`
	Company     string `json:"company"`
	Location    string `json:"location"`
	PostedDate  string `json:"postedDate"`
	Link        string `json:"link"`
	IsEasyApply bool   `json:"isEasyApply"`
	Processed   bool   `json:"processed"`
}

// JobResponse struct for API response
type JobResponse struct {
	ID         string `json:"id"`
	JobID       int64  `json:"jobId"` // Added jobId
	Title      string `json:"title"`
	Company    string `json:"company"`
	Location   string `json:"location"`
	PostedDate string `json:"postedDate"`
	Link       string `json:"link"`
	Processed  bool   `json:"processed"`
}


// Utility function: Construct LinkedIn job search URL
func constructSearchUrl(keywords, location, dateSincePosted string) string {
	return fmt.Sprintf(
		"https://www.linkedin.com/jobs/search?keywords=%s&location=%s&f_TPR=%s&position=1&pageNum=0",
		strings.ReplaceAll(keywords, " ", "%20"),
		strings.ReplaceAll(location, " ", "%20"),
		dateSincePosted,
	)
}

// Utility function: Set up Chromedp context
func setupChromedpContext() (context.Context, context.CancelFunc) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", false), // Disable headless mode (optional)
		chromedp.Flag("executable-path", "/snap/bin/chromium"),
		chromedp.Flag("window-size", "800,600"),
	)

	allocatorCtx, allocatorCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, ctxCancel := chromedp.NewContext(allocatorCtx)

	return ctx, func() {
		ctxCancel()
		allocatorCancel()
	}
}


// Check if job exists & insert if not
var ctr int = 1
func insertJobIfNotExists(db *sql.DB, job Job) error {

    // Attempt to insert the job into the database
    _, err := db.Exec(`
        INSERT INTO linkedin_jobs (id, jobid, title, company, location, posted_date, link, processed)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
        uuid.New().String(), strconv.FormatInt(job.JobID, 10), job.Title, job.Company, job.Location, job.PostedDate, job.Link, false,
    )

    if err != nil {
        if strings.Contains(err.Error(), "UNIQUE constraint failed") {
            return fmt.Errorf("❌ Job already exists: %v", err)
        }
        return fmt.Errorf("❌ Failed to insert job: %v", err)
    }

    fmt.Printf("✅ Inserted new job: (%d)\n", ctr)
    ctr++
    return nil
}

func extractJobID(link string) int64 {
	// Log the incoming link for debugging

	// Find the index of the last hyphen in the URL, which precedes the job ID
	lastHyphenIndex := strings.LastIndex(link, "-")
	if lastHyphenIndex == -1 {
		fmt.Printf("No hyphen found, returning 0")
		return 0
	}
	// Extract the job ID (everything after the last hyphen and before the query string or end of URL)
	jobID := link[lastHyphenIndex+1:]

	// Check if the URL contains query parameters, and if so, trim them
	if strings.Contains(jobID, "?") {
		jobID = strings.Split(jobID, "?")[0]
	}

	// Convert the job ID string to int64
	num, err := strconv.ParseInt(jobID, 10, 64)
	if err != nil {
		// Log the error and return 0 if conversion fails
		return 0
	}
	return num
}

// Fetch and store jobs in SQLite
func fetchAndStoreJobs(ctx context.Context, db *sql.DB, jobTitles []string, location, dateSincePosted string) error {
	for _, title := range jobTitles {
		searchURL := constructSearchUrl(title, location, dateSincePosted)
		var jobs []Job

		err := chromedp.Run(ctx,
			chromedp.Navigate(searchURL),
			chromedp.Sleep(5*time.Second),
			chromedp.WaitVisible(`.jobs-search__results-list`, chromedp.ByQuery),
			chromedp.Evaluate(`Array.from(document.querySelectorAll('.jobs-search__results-list li')).map(el => ({
				title: "`+title+`",
				link: el.querySelector('.base-card__full-link')?.href || '',
				company: el.querySelector('.base-search-card__subtitle')?.innerText.trim() || 'Unknown',
				location: el.querySelector('.job-search-card__location')?.innerText.trim() || 'Unknown',
				postedDate: el.querySelector('time')?.getAttribute('datetime') || 'Unknown',
				isEasyApply: el.querySelector('.jobs-apply-button--top-card') !== null
			}))`, &jobs),
		)

		if err != nil {
			fmt.Printf("❌ Failed to fetch jobs for %s: %v\n", title, err)
			continue
		}

		count := 0
		for _, job := range jobs {
			if job.Link != "" && !job.IsEasyApply {
				// Extract jobid from the link
				job.JobID = extractJobID(job.Link)

				err := insertJobIfNotExists(db, job)
				if err != nil {
					fmt.Println(err)
					continue
				}
				count++
				if count >= 50 {
					break
				}
			}
		}

		fmt.Printf("✅ Stored %d new jobs for %s (excluding Easy Apply and duplicates)\n", count, title)
	}
	chromedp.Cancel(ctx)
	fmt.Println("📂 Job listings stored in database.")
	return nil
}


// LinkedinJobListingsHandler handles job scraping and storing for LinkedIn.
func LinkedinJobListingsHandler(ctx context.Context, db *sql.DB) error {
	// Set up a chromedp context with cancel
	chromeCtx, cancel := setupChromedpContext()
	defer cancel()

	jobTitles := []string{
		"Logistics Coordinator",
		"Supply Chain Specialist",
		"Procurement Specialist",
		"Financial Analyst",
		"Accounts Specialist",
		"Management Accountant",
		"Operations Manager ",
		"Project Coordinator ",
		"Supply Chain Analyst",
		"Mechanical Design Engineer",
		"Automation Engineer",
		"Production Engineer",
		"Logistics Chain Analyst",
		"Supply Chain Consultant",
		"Procurement Analyst",
		"Sports Marketing Manager",
		"Business Development Manager",
		"Sponsorship Manager",
		"Business Analyst",
		"Relationship Manager",
		}

	location := "Berlin, Germany"
	dateSincePosted := ""

	// Perform scraping and store in DB
	return fetchAndStoreJobs(chromeCtx, db, jobTitles, location, dateSincePosted)
}


