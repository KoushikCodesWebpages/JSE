package scraper

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	_ "github.com/mattn/go-sqlite3"
)

// Job model
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


// UploadHandler connects to MongoDB, fetches jobs from SQLite, and prints them
func UploadHandler(w http.ResponseWriter, r *http.Request, sqliteDB *sql.DB) {
	// MongoDB hardcoded credentials
	// mongoURI := "mongodb+srv://JSE:JSE@cluster0.dnetqn5.mongodb.net/?retryWrites=true&w=majority&appName=Cluster0&tls=true"
	mongoURI :="mongodb://localhost:27017/JSE"
	mongoDBName := "JSE"
	mongoCollectionName := "jobs"

	// Connect to MongoDB
	clientOpts := options.Client().ApplyURI(mongoURI)
	client, err := mongo.NewClient(clientOpts)
	if err != nil {
		http.Error(w, "MongoDB client creation failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	err = client.Connect(ctx)
	if err != nil {
		http.Error(w, "MongoDB connection failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer client.Disconnect(ctx)

	log.Println("✅ Connected to MongoDB database:", mongoDBName)

	// Fetch jobs from SQLite
	jobs, err := CollectJobs(sqliteDB)
	if err != nil {
		http.Error(w, "Failed to collect jobs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Insert jobs into MongoDB
	collection := client.Database(mongoDBName).Collection(mongoCollectionName)
	var mongoJobs []interface{}
	for _, job := range jobs {
		mongoJobs = append(mongoJobs, job)
	}

	if len(mongoJobs) > 0 {
		_, err := collection.InsertMany(ctx, mongoJobs)
		if err != nil {
			http.Error(w, "Failed to insert jobs into MongoDB: "+err.Error(), http.StatusInternalServerError)
			return
		}
		log.Printf("✅ Inserted %d jobs into MongoDB", len(mongoJobs))
	}

	// Retrieve all jobs from MongoDB
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

	// Output jobs from MongoDB as JSON
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(mongoJobsList)
	if err != nil {
		http.Error(w, "Failed to encode jobs to JSON: "+err.Error(), http.StatusInternalServerError)
		return
	}
}


// CollectJobs fetches LinkedIn and Xing jobs from SQLite
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
