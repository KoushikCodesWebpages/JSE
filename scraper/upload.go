package scraper

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"


	_ "github.com/mattn/go-sqlite3"
	_ "github.com/go-sql-driver/mysql"
)

// Job model used during migration
type Job struct {
	JobID          string
	Title          string
	Company        string
	Location       string
	PostedDate     string
	Link           string
	Processed      bool
	Source         string // LinkedIn or Xing
	JobDescription string
	JobType        string
	Skills         string
	JobLink        string
}

// UploadHandler handles the migration from SQLite to MySQL
// UploadHandler handles the migration from SQLite to MySQL
func UploadHandler(w http.ResponseWriter, r *http.Request, sqliteDB *sql.DB) {
	// Connect to MySQL
	mysqlDSN := fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		"root",
		"idNxcPPQFpRyAsufSVAoPtwhdrNFGfAe",
		"crossover.proxy.rlwy.net",
		57423,
		"railway",
	)
	mysqlDB, err := sql.Open("mysql", mysqlDSN)
	if err != nil {
		http.Error(w, "MySQL connection failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer mysqlDB.Close()

	// Print the table names in MySQL
	rows, err := mysqlDB.Query("SHOW TABLES")
	if err != nil {
		http.Error(w, "Failed to retrieve table names: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Collect all table names
	var tableName string
	var tableNames []string
	for rows.Next() {
		if err := rows.Scan(&tableName); err != nil {
			http.Error(w, "Failed to scan table name: "+err.Error(), http.StatusInternalServerError)
			return
		}
		tableNames = append(tableNames, tableName)
	}

	// Check for errors after iteration
	if err := rows.Err(); err != nil {
		http.Error(w, "Error while iterating over tables: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Print the table names
	log.Printf("MySQL Tables: %v", tableNames)

	// Collect jobs from SQLite
	jobs, err := CollectJobs(sqliteDB)
	if err != nil {
		http.Error(w, "Failed to collect jobs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Upload jobs and count success/failure
	var successCount, failureCount int
	for _, job := range jobs {
		var err error
		switch job.Source {
		case "LinkedIn":
			// Uncomment the following line for actual upload
			err = uploadLinkedInJob(mysqlDB, job)
			log.Printf("Uploading LinkedIn job: %s", job.JobID) // For debugging
		case "Xing":
			// Uncomment the following line for actual upload
			err = uploadXingJob(mysqlDB, job)
			log.Printf("Uploading Xing job: %s", job.JobID) // For debugging
		}
		
		// For demo, simulate success or failure
		if err != nil {
			log.Printf("❌ Failed to upload job %s (%s): %v", job.JobID, job.Source, err)
			failureCount++
		} else {
			successCount++
		}
	}

	// Print total uploaded data
	log.Printf("Upload completed: Success: %d | Failures: %d", successCount, failureCount)

	// Response with the uploaded job count
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"tables":        tableNames,
		"uploaded_jobs": jobs,
		"count":         len(jobs),
		"success_count": successCount,
		"failure_count": failureCount,
	})
}



func CollectJobs(db *sql.DB) ([]Job, error) {
	var jobs []Job

	// LinkedIn Jobs
	rowsLinkedIn, err := db.Query(`
		SELECT 
			lj.id, lj.title, lj.company, lj.location, lj.posted_date, lj.link, lj.processed,
			ljd.job_description, ljd.job_type, ljd.skills,
			ljal.job_link
		FROM linkedin_job_application_links ljal
		LEFT JOIN linkedin_jobs lj ON ljal.job_id = lj.id
		LEFT JOIN linkedin_job_description ljd ON ljal.job_id = ljd.job_id
	`)
	if err != nil {
		return nil, fmt.Errorf("LinkedIn query error: %v", err)
	}
	defer rowsLinkedIn.Close()

	for rowsLinkedIn.Next() {
		var job Job
		var desc, typ, skills, jobLink sql.NullString
		err := rowsLinkedIn.Scan(&job.JobID, &job.Title, &job.Company, &job.Location, &job.PostedDate, &job.Link, &job.Processed,
			&desc, &typ, &skills, &jobLink)
		if err != nil {
			return nil, fmt.Errorf("LinkedIn scan error: %v", err)
		}
		if desc.Valid {
			job.JobDescription = desc.String
		}
		if typ.Valid {
			job.JobType = typ.String
		}
		if skills.Valid {
			job.Skills = skills.String
		}
		if jobLink.Valid {
			job.JobLink = jobLink.String
		}
		job.Source = "LinkedIn"
		jobs = append(jobs, job)
	}

	// Xing Jobs
	rowsXing, err := db.Query(`
		SELECT 
			xjal.job_id, xj.title, xj.company, xj.location, xj.posted_date, xj.link, xj.processed,
			xjd.job_description, xjd.job_type, xjd.skills,
			xjal.job_link
		FROM xing_job_application_links xjal
		LEFT JOIN xing_jobs xj ON xjal.job_id = xj.id
		LEFT JOIN xing_job_description xjd ON xjal.job_id = xjd.job_id
	`)
	if err != nil {
		return nil, fmt.Errorf("xing query error: %v", err)
	}
	defer rowsXing.Close()

	for rowsXing.Next() {
		var job Job
		var desc, typ, skills, jobLink sql.NullString
		err := rowsXing.Scan(&job.JobID, &job.Title, &job.Company, &job.Location, &job.PostedDate, &job.Link, &job.Processed,
			&desc, &typ, &skills, &jobLink)
		if err != nil {
			return nil, fmt.Errorf("xing scan error: %v", err)
		}
		if desc.Valid {
			job.JobDescription = desc.String
		}
		if typ.Valid {
			job.JobType = typ.String
		}
		if skills.Valid {
			job.Skills = skills.String
		}
		if jobLink.Valid {
			job.JobLink = jobLink.String
		}
		job.Source = "Xing"
		jobs = append(jobs, job)
	}

	return jobs, nil
}

func uploadLinkedInJob(db *sql.DB, job Job) error {
	// Insert job metadata into `linked_in_job_meta_data`, excluding the ID field
	_, err := db.Exec(`
		INSERT IGNORE INTO linked_in_job_meta_data (id, job_id, title, company, location, posted_date, link, processed)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		job.JobID, job.JobID, job.Title, job.Company, job.Location, job.PostedDate, job.Link, job.Processed)
	if err != nil {
		return fmt.Errorf("linkedin_jobs_meta_data insert: %w", err)
	}

	// Insert job description into `linked_in_job_descriptions`
	_, err = db.Exec(`
		INSERT IGNORE INTO linked_in_job_descriptions (job_id, job_link, job_description, job_type, skills)
		VALUES (?, ?, ?, ?, ?)`,
		job.JobID, job.JobLink, job.JobDescription, job.JobType, job.Skills)
	if err != nil {
		return fmt.Errorf("linkedin_job_description insert: %w", err)
	}

	// Insert job application link into `linked_in_job_application_links`
	_, err = db.Exec(`
		INSERT IGNORE INTO linked_in_job_application_links (job_id, job_link)
		VALUES (?, ?)`,
		job.JobID, job.JobLink)
	if err != nil {
		return fmt.Errorf("linkedin_job_application_links insert: %w", err)
	}

	return nil
}

// Upload Xing job
func uploadXingJob(db *sql.DB, job Job) error {
	_, err := db.Exec(`
		INSERT IGNORE INTO xing_jobs (id, title, company, location, posted_date, link, processed)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		job.JobID, job.Title, job.Company, job.Location, job.PostedDate, job.Link, job.Processed)
	if err != nil {
		return fmt.Errorf("xing_jobs insert: %w", err)
	}

	_, err = db.Exec(`
		INSERT IGNORE INTO xing_job_description (job_id, job_description, job_type, skills)
		VALUES (?, ?, ?, ?)`,
		job.JobID, job.JobDescription, job.JobType, job.Skills)
	if err != nil {
		return fmt.Errorf("xing_job_description insert: %w", err)
	}

	_, err = db.Exec(`
		INSERT IGNORE INTO xing_job_application_links (job_id, job_link)
		VALUES (?, ?)`,
		job.JobID, job.JobLink)
	if err != nil {
		return fmt.Errorf("xing_job_application_links insert: %w", err)
	}
	return nil
}