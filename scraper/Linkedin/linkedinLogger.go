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
	//"errors"
	
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
	log.Printf("✅ Loaded %d job titles with links.\n", len(jobLinks))

	totalLinks := 0
	for title, jobs := range jobLinks {
		log.Printf("🔹 %s (%d jobs)\n", title, len(jobs))
		totalLinks += len(jobs)
	}
	
	log.Printf("📊 Total job links loaded: %d\n", totalLinks)

	// Create ChromeDP allocator context
	allocatorCtx, cancelAllocator := chromedp.NewRemoteAllocator(context.Background(), "http://localhost:9222")
	defer cancelAllocator()

	// Create root ChromeDP context
	_, cancelCtx := chromedp.NewContext(allocatorCtx)
	defer cancelCtx()
	
	for title, jobs := range jobLinks {
		fmt.Printf("📌 Processing jobs for: %s\n", title)
	
		for _, job := range jobs {
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("⚠️ Recovered from panic while processing job %s: %v\n", job.ID, r)
					}
				}()
	
				jobIDStr := job.ID
				fmt.Println("🔗 Processing:", job.Link)
	
				// ✅ Mark job as "being processed" immediately
				updateQuery := `
					UPDATE linkedin_jobs 
					SET processed = TRUE 
					WHERE id = ?
				`
				_, err := db.Exec(updateQuery, jobIDStr)
				if err != nil {
					log.Printf("❌ Failed to update job %s as processed: %v\n", jobIDStr, err)
					// Optional: return here if you want to skip on failure
				}
	
				// 🎯 Create new Chrome context
				jobCtxBase, cancelBase := chromedp.NewContext(allocatorCtx)
				defer cancelBase()
	
				jobCtx, cancelJob := context.WithTimeout(jobCtxBase, 40*time.Second)
				defer cancelJob()
	
				startTime := time.Now()
	
				initialTabs, err := chromedp.Targets(jobCtx)
				if err != nil {
					log.Printf("⚠️ Skipping job %s - failed to get tabs: %v\n", jobIDStr, err)
					return
				}
	
				existingTabs := make(map[target.ID]struct{})
				for _, t := range initialTabs {
					existingTabs[t.TargetID] = struct{}{}
				}
	
				if err := navigateAndClickApply(jobCtx, db, jobIDStr, job.Link); err != nil {
					log.Printf("⚠️ Skipping job %s - navigation/apply failed: %v\n", jobIDStr, err)
				} else {
					if err := captureAndCloseNewTab(jobCtx, db, jobIDStr, existingTabs); err != nil {
						log.Printf("⚠️ Skipping capture for job %s - error: %v\n", jobIDStr, err)
					}
				}
	
				fmt.Printf("⏱️ Job %s completed in %s\n", jobIDStr, time.Since(startTime))
			}()
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
			&job.JobDescription,  // stored as job_description in DB, mapped to Description field
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
