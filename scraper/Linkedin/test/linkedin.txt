package Linkedin

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
	
	"github.com/chromedp/chromedp"
)


func LinkedInHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Job Listings (Fetch and store jobs in CSV)
	fmt.Println("🚀 Starting Job Listings automation...")

	// Set up chromedp with Chromium executable path (optional)
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", false), // Disable headless mode (optional)
		chromedp.Flag("executable-path", "/snap/bin/chromium"), // Replace with actual path if needed
	)

	// Create a new context with the specified options
	allocatorCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	// Create a new chromedp context using the allocator context
	ctx, cancel := chromedp.NewContext(allocatorCtx)
	defer cancel()

	jobTitles := []string{
	"Software Engineer", "Data Scientist", "Product Manager",
	"DevOps Engineer", "Cybersecurity Analyst", "Cloud Engineer",
	"Machine Learning Engineer", "Frontend Developer", "Backend Developer", "QA Engineer",
	
	// New Titles
	"Software Engineering Intern", "Data Engineer", "Full Stack Developer",
	"AI Engineer", "Mobile App Developer", "Game Developer",
	"Embedded Software Engineer", "Blockchain Developer", "NLP Engineer",
	"Big Data Engineer",
	}

	location := "Berlin, Germany"
	dateSincePosted := ""

	// Fetch and store jobs in the CSV
	if err := fetchAndStoreJobs(ctx, jobTitles, location, dateSincePosted); err != nil {
		http.Error(w, fmt.Sprintf("❌ Error fetching job listings: %v", err), http.StatusInternalServerError)
		return
	}
	fmt.Println("✅ Job listings saved to CSV.")

	// 2. Login to LinkedIn (Automation for applying to jobs)
	fmt.Println("🚀 Starting LinkedIn job application automation...")

	// Start Chrome (Chromium) with remote debugging
	/*if err := StartChrome(); err != nil {
		log.Fatalf("❌ Failed to start Chrome: %v", err)
	}
	fmt.Println("✅ Chrome launched successfully.")
	*/

	// Wait for Chrome to start and be ready for remote debugging
	time.Sleep(5 * time.Second)

	// Load job links from CSV file
	jobLinks, err := LoadJobLinks("../../storage/linkedin_jobs.csv")
	if err != nil {
		log.Fatalf("❌ Failed to load job links: %v", err)
	}
	fmt.Printf("✅ Loaded %d job titles with links.\n", len(jobLinks))

	// Connect to the already running Chrome instance using the remote debugger
	allocatorCtx, cancel = chromedp.NewRemoteAllocator(
		context.Background(),
		"http://localhost:9222", // Connect to the existing Chrome instance via remote debugging
	)
	defer cancel()

	// Create a new chromedp context with the allocator context
	ctx, cancel = chromedp.NewContext(allocatorCtx)
	defer cancel()

	// Process all job links correctly (apply to jobs)
	if err := ProcessJobLinks(ctx, jobLinks); err != nil {
		log.Fatalf("❌ Error processing job links: %v", err)
	}
	fmt.Println("✅ Job application automation completed.")

	// 3. Upload to Azure SQL (Upload CSV to Azure SQL database)
	fmt.Println("📡 Starting to upload CSV to Azure SQL...")

	err = UploadCSVToAzure()
	if err != nil {
		http.Error(w, fmt.Sprintf("❌ Upload failed: %v", err), http.StatusInternalServerError)
		return
	}

	fmt.Println("✅ CSV successfully uploaded to Azure SQL.")

	// Send a success message indicating the process is complete
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Job listings fetched, LinkedIn jobs applied, and CSV uploaded to Azure SQL successfully."})

	cancel()
}
