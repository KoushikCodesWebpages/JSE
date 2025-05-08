package scraper

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Job model – no "sent" field
type Job struct {
	JobID          string `json:"job_id" bson:"job_id"`
	Title          string `json:"title" bson:"title"`
	Company        string `json:"company" bson:"company"`
	Location       string `json:"location" bson:"location"`
	PostedDate     string `json:"posted_date" bson:"posted_date"`
	Link           string `json:"link" bson:"link"`
	Processed      bool   `json:"processed" bson:"processed"`
	Source         string `json:"source" bson:"source"`
	JobDescription string `json:"job_description" bson:"job_description"`
	JobType        string `json:"job_type" bson:"job_type"`
	Skills         string `json:"skills" bson:"skills"`
	JobLink        string `json:"job_link" bson:"job_link"`
}

// UploadHandler handles job upload and marking as sent
// UploadHandler handles job upload and marking as sent
// UploadHandler handles job upload and marking as sent
func UploadHandler(w http.ResponseWriter, r *http.Request, sqliteDB *sql.DB) {
	mongoURI := "mongodb+srv://JSE:JSE@cluster0.dnetqn5.mongodb.net/?retryWrites=true&w=majority&appName=Cluster0&tls=true"
	mongoDBName := "JSE"
	mongoCollectionName := "jobs"

	// Set a larger connection pool size
	maxPoolSize := uint64(400) // Set your desired max pool size

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	clientOpts := options.Client().
		ApplyURI(mongoURI).
		SetMaxPoolSize(maxPoolSize) // Increase pool size

	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		http.Error(w, "MongoDB connection failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer client.Disconnect(ctx)

	log.Println("✅ Connected to MongoDB:", mongoDBName)
	collection := client.Database(mongoDBName).Collection(mongoCollectionName)

	jobs, err := CollectJobs(sqliteDB)
	if err != nil {
		http.Error(w, "Failed to collect jobs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	insertedCount := 0

	for _, job := range jobs {
		// Skip if sent == true (No need to upload this job)
		if job.Processed {
			continue
		}

		// Check if job already exists
		var existingJob Job
		err := collection.FindOne(ctx, bson.M{"job_id": job.JobID}).Decode(&existingJob)
		if err == nil {
			// Job already exists, skip it
			log.Printf("Job with job_id %s already exists, skipping insertion", job.JobID)
			continue
		}

		// Insert the job into MongoDB
		_, err = collection.InsertOne(ctx, job)
		if err != nil {
			log.Printf("❌ Failed to insert job %s: %v", job.JobID, err)
			continue
		}

		// Mark the job as sent in SQLite
		table := "linkedin_jobs"
		if job.Source == "Xing" {
			table = "xing_jobs"
		}
		updateQuery := fmt.Sprintf("UPDATE %s SET sent = TRUE WHERE id = ?", table)
		if _, err := sqliteDB.Exec(updateQuery, job.JobID); err != nil {
			log.Printf("❌ Failed to mark job %s as sent: %v", job.JobID, err)
			continue
		}

		insertedCount++
	}

	log.Printf("✅ Inserted %d jobs into MongoDB and marked as sent", insertedCount)

	// Fetch inserted jobs for confirmation
	cursor, err := collection.Find(ctx, bson.D{})
	if err != nil {
		http.Error(w, "Failed to retrieve jobs from MongoDB: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var mongoJobsList []Job
	for cursor.Next(ctx) {
		var job Job
		if err := cursor.Decode(&job); err != nil {
			http.Error(w, "Failed to decode job: "+err.Error(), http.StatusInternalServerError)
			return
		}
		mongoJobsList = append(mongoJobsList, job)
	}

	if err := cursor.Err(); err != nil {
		http.Error(w, "Cursor error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(mongoJobsList); err != nil {
		http.Error(w, "Failed to encode jobs to JSON: "+err.Error(), http.StatusInternalServerError)
	}
}

// CollectJobs fetches jobs from LinkedIn and Xing in SQLite, skipping sent == true
func CollectJobs(db *sql.DB) ([]Job, error) {
	var jobs []Job

	// LinkedIn Jobs
	rowsLinkedIn, err := db.Query(`
		SELECT 
			lj.id, lj.title, lj.company, lj.location, lj.posted_date, lj.link, lj.processed, lj.sent,
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
		var sent sql.NullBool

		err := rowsLinkedIn.Scan(&job.JobID, &job.Title, &job.Company, &job.Location, &job.PostedDate, &job.Link, &job.Processed, &sent,
			&desc, &typ, &skills, &jobLink)
		if err != nil {
			return nil, fmt.Errorf("LinkedIn scan error: %v", err)
		}

		// Skip if sent == true
		if sent.Valid && sent.Bool {
			continue
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
			xj.id, xj.title, xj.company, xj.location, xj.posted_date, xj.link, xj.processed, xj.sent,
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
		var sent sql.NullBool

		err := rowsXing.Scan(&job.JobID, &job.Title, &job.Company, &job.Location, &job.PostedDate, &job.Link, &job.Processed, &sent,
			&desc, &typ, &skills, &jobLink)
		if err != nil {
			return nil, fmt.Errorf("xing scan error: %v", err)
		}

		// Skip if sent == true
		if sent.Valid && sent.Bool {
			continue
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
