package scraper

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"job_scraper/scraper/Linkedin"
	// "job_scraper/scraper/Xing"

)



// Construct LinkedIn job search URL
// Insert job into the specified table if it doesn't already exist

func JobListingsHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	ctx := context.Background()

	//Run Xing scraper
	// if err := Xing.XingJobListingsHandler(ctx, db); err != nil {
	// 	http.Error(w, "Xing error: "+err.Error(), http.StatusInternalServerError)
	// 	return
	// }

	// Run LinkedIn scraper
	if err := Linkedin.LinkedinJobListingsHandler(ctx, db); err != nil {
		http.Error(w, "LinkedIn error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Respond with success
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Jobs stored from LinkedIn and Xing"})
}

// JobResponse struct for API response
type JobResponse struct {
	ID         string `json:"id"`
	JobID      string `json:"jobId"`
	Title      string `json:"title"`
	Company    string `json:"company"`
	Location   string `json:"location"`
	PostedDate string `json:"postedDate"`
	Link       string `json:"link"`
	Processed  bool   `json:"processed"`
	Sent       bool   `json:"sent"`
}

// ViewJobsHandler fetches jobs from both LinkedIn and Xing databases
func ViewJobsHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	// Fetch LinkedIn jobs
	linkedinRows, err := db.Query("SELECT id, jobid, title, company, location, posted_date, link, processed, sent FROM linkedin_jobs ORDER BY posted_date DESC")
	if err != nil {
		http.Error(w, fmt.Sprintf("Error fetching LinkedIn jobs: %v", err), http.StatusInternalServerError)
		return
	}
	defer linkedinRows.Close()

	var linkedinJobs []JobResponse
	for linkedinRows.Next() {
		var job JobResponse
		var sent sql.NullBool

		err := linkedinRows.Scan(
			&job.ID,
			&job.JobID,
			&job.Title,
			&job.Company,
			&job.Location,
			&job.PostedDate,
			&job.Link,
			&job.Processed,
			&sent,
		)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error scanning LinkedIn row: %v", err), http.StatusInternalServerError)
			return
		}
		job.Sent = sent.Valid && sent.Bool
		linkedinJobs = append(linkedinJobs, job)
	}

	// Fetch Xing jobs
	xingRows, err := db.Query("SELECT id, jobid, title, company, location, posted_date, link, processed, sent FROM xing_jobs ORDER BY posted_date DESC")
	if err != nil {
		http.Error(w, fmt.Sprintf("Error fetching Xing jobs: %v", err), http.StatusInternalServerError)
		return
	}
	defer xingRows.Close()

	var xingJobs []JobResponse
	for xingRows.Next() {
		var job JobResponse
		var sent sql.NullBool

		err := xingRows.Scan(
			&job.ID,
			&job.JobID,
			&job.Title,
			&job.Company,
			&job.Location,
			&job.PostedDate,
			&job.Link,
			&job.Processed,
			&sent,
		)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error scanning Xing row: %v", err), http.StatusInternalServerError)
			return
		}
		job.Sent = sent.Valid && sent.Bool
		xingJobs = append(xingJobs, job)
	}

	// Get job counts
	var linkedinCount, xingCount int
	err = db.QueryRow("SELECT COUNT(*) FROM linkedin_jobs").Scan(&linkedinCount)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error fetching LinkedIn job count: %v", err), http.StatusInternalServerError)
		return
	}
	err = db.QueryRow("SELECT COUNT(*) FROM xing_jobs").Scan(&xingCount)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error fetching Xing job count: %v", err), http.StatusInternalServerError)
		return
	}

	// Convert to JSON and return response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"linkedin": map[string]interface{}{"count": linkedinCount, "jobs": linkedinJobs},
		"xing":     map[string]interface{}{"count": xingCount, "jobs": xingJobs},
	})
}
