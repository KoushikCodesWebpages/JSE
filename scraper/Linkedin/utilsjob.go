package Linkedin

import (
	"context"
	"fmt"
	"log"
	"time"
	"database/sql"
	"strings"
	//"path/filepath"
	"github.com/chromedp/chromedp"
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"github.com/joho/godotenv"
	"os"
)


// Load .env on init
func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("❌ Error loading .env file")
	}
}

var (
	HF_API_URL = os.Getenv("HF_API_URL")
	HF_API_KEY = os.Getenv("HF_API_KEY")
)
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
func navigateAndClickApply(ctx context.Context, db *sql.DB, jobID string, jobLink string) error {
	// 1. Navigate to the job posting
	err := chromedp.Run(ctx,
		chromedp.Navigate(jobLink),
		chromedp.Sleep(5*time.Second),
	)
	if err != nil {
		log.Printf("❌ Failed to navigate to job: %s -> %v\n", jobID, err)
		StoreFailedJob(db, jobID, jobLink, "Navigation failed")
		return err
	}

	// 2. Extract raw job description
	var rawDescription string
	err = chromedp.Run(ctx,
		chromedp.Text(`#job-details`, &rawDescription, chromedp.NodeVisible, chromedp.ByID),
	)
	if err != nil || strings.TrimSpace(rawDescription) == "" {
		log.Printf("❌ Failed to extract job description for jobID %s: %v\n", jobID, err)
		StoreFailedJob(db, jobID, jobLink, "Description not found")
		return err
	}

	// 3. Use Hugging Face to summarize and structure the description
	summary, err := extractStructuredSummary(rawDescription)
	if err != nil {
		log.Printf("⚠️ Failed to summarize job description for jobID %s: %v\n", jobID, err)
		// Optional fallback:
		// summary = rawDescription
	}

	// 4. Store the job description (structured or raw)
	err = storeJobDescription(db, jobID, jobLink, strings.TrimSpace(summary))
	if err != nil {
		log.Printf("❌ Failed to store job description for jobID %s: %v\n", jobID, err)
		return err
	}

	// 5. Attempt to click Apply button AFTER extraction
	err = chromedp.Run(ctx,
		chromedp.Click(`div.jobs-apply-button--top-card button`, chromedp.NodeVisible),
		chromedp.Sleep(3*time.Second),
	)
	if err != nil {
		log.Printf("⚠️ Apply button not found for jobID %s: %v\n", jobID, err)
		// Not critical — description already stored
	}

	log.Printf("✅ Job %s processed and Apply attempted", jobID)
	return nil
}




type HuggingFaceResponse struct {
	GeneratedText string `json:"generated_text"`
}




func extractStructuredSummary(jobDescription string) (string, error) {
	// Load API credentials from environment
	apiURL := os.Getenv("HF_API_URL")
	apiKey := os.Getenv("HF_API_KEY")

	// Sanity check
	if apiURL == "" || apiKey == "" {
		return "", fmt.Errorf("missing Hugging Face API credentials or URL")
	}

	// Prompt construction
	prompt := fmt.Sprintf(`Extract skills, jobtype that is given in the description, dont create new ones, and summarize the whole details into a summarized description and return in JSON format with fields: 

	- "job_type (remote, part time, full time,unknown)"
	- "skills"
	- "description"

	Description: "%s"`, jobDescription)

	// Request payload
	payload := map[string]interface{}{
		"inputs": prompt,
		"parameters": map[string]interface{}{
			"max_length":  1000,
			"temperature": 0.3,
		},
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request payload: %v", err)
	}

	// HTTP request
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	// HTTP client and response
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("hugging face request failed: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	// Parse Hugging Face response
	var result []HuggingFaceResponse
	err = json.Unmarshal(body, &result)
	if err != nil || len(result) == 0 {
		return "", fmt.Errorf("failed to parse API response: %v", err)
	}

	output := result[0].GeneratedText

	// Extract JSON block from output text
	jsonStart := strings.Index(output, "{")
	jsonEnd := strings.LastIndex(output, "}")
	if jsonStart != -1 && jsonEnd != -1 && jsonEnd > jsonStart {
		output = output[jsonStart : jsonEnd+1]
	}

	return strings.TrimSpace(output), nil
}

// Store the summary as the job description and mark job as processed
func storeJobDescription(db *sql.DB, jobID, jobLink, summary string) error {
	// Parse the structured summary JSON
	var parsed struct {
		JobType     string   `json:"job_type"`
		Skills      []string `json:"skills"`
		Description string   `json:"description"`
	}

	err := json.Unmarshal([]byte(summary), &parsed)
	if err != nil {
		return fmt.Errorf("failed to parse structured summary: %v", err)
	}

	// Convert skills array to comma-separated string
	skillsCSV := strings.Join(parsed.Skills, ", ")

	// 1. Insert structured job description into DB
	insertQuery := `
		INSERT OR IGNORE INTO linkedin_job_description 
		(job_id, job_link, job_description, job_type, skills) 
		VALUES (?, ?, ?, ?, ?)
	`
	_, err = db.Exec(insertQuery, jobID, jobLink, parsed.Description, parsed.JobType, skillsCSV)
	if err != nil {
		return fmt.Errorf("failed to insert job description: %v", err)
	}

	// 2. Mark job as processed in linkedin_jobs
	updateQuery := `
		UPDATE linkedin_jobs 
		SET processed = FALSE 
		WHERE id = ?
	`
	_, err = db.Exec(updateQuery, jobID)
	if err != nil {
		return fmt.Errorf("failed to update job as processed: %v", err)
	}

	return nil
}
