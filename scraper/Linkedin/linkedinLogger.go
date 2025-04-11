package Linkedin

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	//"strconv"
	"time"
	
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"strings"
)





var chromeCmd *exec.Cmd // Global variable to track the process
func LoginLinkedInHandler(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	fmt.Println("🚀 Starting LinkedIn job application automation...")

	// Start Chrome with remote debugging
	linkedInURL := "https://www.linkedin.com/"
	chromePath := "chromium"

	chromeCmd := exec.Command(chromePath, "--remote-debugging-port=9222", "--profile-directory=Profile 2", linkedInURL)

	if runtime.GOOS != "linux" {
		http.Error(w, "Unsupported OS", http.StatusInternalServerError)
		return
	}

	err := chromeCmd.Start()
	if err != nil {
		log.Printf("❌ Failed to start Chromium: %v\n", err)
		http.Error(w, "Failed to start Chromium", http.StatusInternalServerError)
		return
	}

	fmt.Println("✅ Chrome launched successfully.")
	time.Sleep(5 * time.Second)

	// Load job links from the database
	jobLinks, err := LoadJobLinksFromDB(db)
	if err != nil {
		log.Printf("❌ Failed to load job links: %v\n", err)
		http.Error(w, "Failed to load job links", http.StatusInternalServerError)
		return
	}
	fmt.Printf("✅ Loaded %d job titles with links.\n", len(jobLinks))

	// Create ChromeDP allocator context
	allocatorCtx, cancelAllocator := chromedp.NewRemoteAllocator(context.Background(), "http://localhost:9222")
	defer cancelAllocator()

	// Create root ChromeDP context
	_, cancelCtx := chromedp.NewContext(allocatorCtx)
	defer cancelCtx()

	// Process all job links
	for title, jobs := range jobLinks {
		fmt.Printf("📌 Processing jobs for: %s\n", title)

		for _, job := range jobs {
			jobIDStr := job.ID
			fmt.Println("🔗 Processing:", job.Link)

			// Create isolated ChromeDP context and timeout for this job
			jobCtxBase, cancelBase := chromedp.NewContext(allocatorCtx)
			jobCtx, cancelJob := context.WithTimeout(jobCtxBase, 30*time.Second)

			startTime := time.Now()

			// Capture initial tabs
			initialTabs, err := chromedp.Targets(jobCtx)
			if err != nil {
				log.Printf("❌ Failed to get initial open tabs: %v\n", err)
				StoreFailedJob(db, jobIDStr, job.Link, "Failed to get initial open tabs")
				cancelJob()
				cancelBase()
				continue
			}

			existingTabs := make(map[target.ID]struct{})
			for _, t := range initialTabs {
				existingTabs[t.TargetID] = struct{}{}
			}

			// Process job
			err = navigateAndClickApply(jobCtx, db, jobIDStr, job.Link)
			if err == nil {
				err = captureAndCloseNewTab(jobCtx, db, jobIDStr, existingTabs)
				if err != nil {
					StoreFailedJob(db, jobIDStr, job.Link, "Failed to capture and close new tab")
				}
			}

			fmt.Printf("⏱️ Job %s completed in %s\n", jobIDStr, time.Since(startTime))

			// Clean up
			cancelJob()
			cancelBase()
		}
	}

	// Stop Chromium process
	fmt.Println("🛑 Closing Chromium...")
	if err := chromeCmd.Process.Kill(); err != nil {
		fmt.Printf("⚠️ Failed to close Chromium: %v\n", err)
	} else {
		fmt.Println("✅ Chromium closed successfully.")
	}

	// Send success response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Job links processed successfully"})
}



// Job represents a job listing

func ViewLinkedInJobs(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, job_id, job_link FROM linkedin_job_application_links")
	if err != nil {
		http.Error(w, "Failed to fetch jobs", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var jobs []Joblinks
	for rows.Next() {
		var job Joblinks
		if err := rows.Scan(&job.ID, &job.JobID, &job.Link); err != nil {
			http.Error(w, "Failed to scan job", http.StatusInternalServerError)
			return
		}
		jobs = append(jobs, job)
	}

	// Convert to JSON and return response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobs)
}

func ViewLinkedInFailedJobs(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, job_id, job_link FROM linkedin_failed_jobs")
	if err != nil {
		http.Error(w, "Failed to fetch failed jobs", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var failedJobs []FailedJob
	for rows.Next() {
		var job FailedJob
		if err := rows.Scan(&job.ID, &job.JobID, &job.JobLink); err != nil {
			http.Error(w, "Failed to scan failed job", http.StatusInternalServerError)
			return
		}
		failedJobs = append(failedJobs, job)
	}

	// Convert to JSON and return response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(failedJobs)
}

type JobDescription struct {
	ID             int    `json:"id"`
	JobID          string `json:"job_id"`
	JobLink        string `json:"job_link"`
	JobDescription string `json:"job_description"`
	JobType     string   `json:"job_type"`
	Skills      []string `json:"skills"`
	Description string   `json:"description"`
}

func ViewLinkedInJobDescriptions(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(`
		SELECT id, job_id, job_link, job_description, job_type, skills 
		FROM linkedin_job_description
	`)
	if err != nil {
		http.Error(w, "Failed to fetch job descriptions", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var jobs []JobDescription
	for rows.Next() {
		var job JobDescription
		var skillsStr string

		err := rows.Scan(
			&job.ID,
			&job.JobID,
			&job.JobLink,
			&job.Description,  // stored as job_description in DB, mapped to Description field
			&job.JobType,
			&skillsStr,
		)
		if err != nil {
			http.Error(w, "Failed to scan job description", http.StatusInternalServerError)
			return
		}

		// Convert comma-separated skills into slice
		if skillsStr != "" {
			job.Skills = strings.Split(skillsStr, ", ")
		}

		jobs = append(jobs, job)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobs)
}
