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

	// Create a ChromeDP context
	allocatorCtx, cancelAllocator := chromedp.NewRemoteAllocator(context.Background(), "http://localhost:"+remoteDebuggingPort)
	defer cancelAllocator()
	ctx, cancelCtx := chromedp.NewContext(allocatorCtx)
	defer cancelCtx()

	// Optional: check that browser is responsive
	if err := chromedp.Run(ctx, chromedp.Navigate("about:blank")); err != nil {
		log.Printf("❌ Failed to navigate to blank page: %v\n", err)
		http.Error(w, "Chromium not responsive", http.StatusInternalServerError)
		return
	}

	// Process all job links
	for title, jobs := range jobLinks {
		fmt.Printf("📌 Processing jobs for: %s\n", title)
		for _, job := range jobs {
			jobIDStr := job.ID
			fmt.Println("🔗 Processing:", job.Link)
	
			// Set 15-second timeout for this job
			jobCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
			defer cancel()
	
			// Capture initial tabs
			initialTabs, err := chromedp.Targets(jobCtx)
			if err != nil {
				log.Printf("❌ Failed to get initial open tabs: %v\n", err)
				StoreFailedJob(db, jobIDStr, job.Link, "Failed to get initial open tabs")
				continue
			}
	
			existingTabs := make(map[target.ID]struct{})
			for _, t := range initialTabs {
				existingTabs[t.TargetID] = struct{}{}
			}
	
			// Navigate and click apply
			err = navigateAndClickApply(jobCtx, db, jobIDStr, job.Link)
			if err != nil {
				continue
			}
	
			// Capture and close any newly opened tab
			err = captureAndCloseNewTab(jobCtx, db, jobIDStr, existingTabs)
			if err != nil {
				StoreFailedJob(db, jobIDStr, job.Link, "Failed to capture and close new tab")
				continue
			}

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


func ViewXingJobs(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, job_id, job_link FROM xing_job_application_links")
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