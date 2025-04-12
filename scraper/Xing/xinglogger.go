package Xing

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/cdproto/target"
)

// Global to track the Chromium process
var chromeCmd *exec.Cmd

// LoginXingHandler opens Xing with an authenticated profile and waits for main menu/dashboard
func LoginXingHandler(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	fmt.Println("🚀 Launching Xing via Chromium...")

	const (
		chromePath           = "chromium" // or "google-chrome"
		remoteDebuggingPort  = "9222"
		supportedOS          = "linux"
		profileDir           = "Profile 2"
		startupWaitDuration  = 5 * time.Second
	)
	XingUrl := "https://www.Xing.com/"
	if runtime.GOOS != supportedOS {
		http.Error(w, "Unsupported OS (only Linux is currently supported)", http.StatusInternalServerError)
		return
	}

	// Start Chromium without any default URL
	chromeCmd = exec.Command(chromePath,
		"--remote-debugging-port="+remoteDebuggingPort,
		"--profile-directory="+profileDir,
		"--new-window",
		XingUrl,
	)
	if err := chromeCmd.Start(); err != nil {
		log.Printf("❌ Failed to start Chromium: %v\n", err)
		http.Error(w, "Failed to start Chromium", http.StatusInternalServerError)
		return
	}
	fmt.Println("✅ Chrome launched successfully.")
	time.Sleep(startupWaitDuration)

	// Load job links from the database
	jobLinks, err := LoadJobLinksFromDB(db)
	if err != nil {
		log.Printf("❌ Failed to load job links: %v\n", err)
		http.Error(w, "Failed to load job links", http.StatusInternalServerError)
		return
	}
	fmt.Printf("✅ Loaded %d job titles with links.\n", len(jobLinks))

	totalLinks := 0
	for title, jobs := range jobLinks {
		log.Printf("🔹 %s (%d jobs)\n", title, len(jobs))
		totalLinks += len(jobs)
	}
	
	log.Printf("📊 Total job links loaded: %d\n", totalLinks)

	// Create a ChromeDP context
	allocatorCtx, cancelAllocator := chromedp.NewRemoteAllocator(context.Background(), "http://localhost:"+remoteDebuggingPort)
	defer cancelAllocator()


	_, cancelCtx := chromedp.NewContext(allocatorCtx)
	defer cancelCtx()
	
	// Optional: check that browser is responsive

	// Process all job links
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

	// Send JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Xing jobs processed successfully",
	})
}

type XingJobLinkDTO struct {
	ID     int    `json:"id"`
	JobID  string `json:"job_id"`
	JobLink string `json:"job_link"`
}

func ViewXingJobs(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, job_id, job_link FROM xing_job_application_links")
	if err != nil {
		http.Error(w, "Failed to fetch jobs", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var jobs []XingJobLinkDTO
	for rows.Next() {
		var job XingJobLinkDTO
		if err := rows.Scan(&job.ID, &job.JobID, &job.JobLink); err != nil {
			http.Error(w, "Failed to scan job", http.StatusInternalServerError)
			return
		}
		jobs = append(jobs, job)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobs)
}


type XingJobDescription struct {
	ID             int    `json:"id"`
	JobID          string `json:"job_id"`
	JobLink        string `json:"job_link"`
	JobDescription string `json:"job_description"`
	JobType        string `json:"job_type"`
	Skills         string `json:"skills"`
}


func ViewXingJobDescriptions(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, job_id, job_link, job_description, job_type, skills FROM xing_job_description")
	if err != nil {
		http.Error(w, "Failed to fetch job descriptions", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var descriptions []XingJobDescription
	for rows.Next() {
		var desc XingJobDescription
		if err := rows.Scan(&desc.ID, &desc.JobID, &desc.JobLink, &desc.JobDescription, &desc.JobType, &desc.Skills); err != nil {
			http.Error(w, "Failed to scan job description", http.StatusInternalServerError)
			return
		}
		descriptions = append(descriptions, desc)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(descriptions)
}
