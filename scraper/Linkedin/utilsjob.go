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
	"io"
	"net/http"
	"github.com/joho/godotenv"
)


// Load .env on init
func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("❌ Error loading .env file")
	}
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
// Struct to hold the API response from Ollama
type OllamaResponse struct {
	Response string `json:"response"`
}

// Struct for the job summary details
type FlexibleJobSummary struct {
	JobType     string   `json:"job_type"`
	Skills      []string `json:"skills"`
	Description string   `json:"description"`
}

// Unmarshal the JSON response from Ollama
func (f *FlexibleJobSummary) UnmarshalJSON(data []byte) error {
	type Alias FlexibleJobSummary
	aux := &struct {
		Skills interface{} `json:"skills"`
		*Alias
	}{
		Alias: (*Alias)(f),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Normalize the `skills` field
	switch v := aux.Skills.(type) {
	case []interface{}:
		for _, s := range v {
			if str, ok := s.(string); ok {
				f.Skills = append(f.Skills, str)
			}
		}
	case map[string]interface{}:
		for k := range v {
			f.Skills = append(f.Skills, k)
		}
	case string:
		f.Skills = []string{v}
	default:
		f.Skills = []string{}
	}

	return nil
}

// Extract structured summary using Ollama API locally
func extractStructuredSummary(jobDescription string) (string, error) {
	// Call the local Ollama API
	output, err := callOllamaAPI(jobDescription)
	if err != nil || output == "" {
		log.Printf("⚠️ Failed to summarize job description: %v\n", err)
		return "", err
	}
	return output, nil
}

// Function to call Ollama API
func callOllamaAPI(jobDescription string) (string, error) {
	// Construct the prompt for Ollama
	prompt := fmt.Sprintf(`Extract job_type, skills, and description from this job posting: "%s". Return JSON.`, jobDescription)

	// Prepare the request payload
	payload := map[string]interface{}{
		"model":   "phi",       // Use the phi model or adjust if you are using a different model
		"prompt":  prompt,
		"stream":  false,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request payload: %v", err)
	}

	// Send the request to the local Ollama API
	req, err := http.NewRequest("POST", "http://localhost:11434/api/generate", bytes.NewBuffer(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama request failed: %v", err)
	}
	defer resp.Body.Close()

	// Read and process the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	// Parse the response body to extract the generated JSON text
	var response OllamaResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return "", fmt.Errorf("failed to parse Ollama response: %v", err)
	}

	// The response from Ollama is expected to be in the format we need directly
	return response.Response, nil
}
// Store the summary as the job description and mark job as processed
func storeJobDescription(db *sql.DB, jobID, jobLink, summary string) error {
	var parsed FlexibleJobSummary
	err := json.Unmarshal([]byte(summary), &parsed)
	if err != nil {
		return fmt.Errorf("failed to parse structured summary: %v", err)
	}

	skillsCSV := strings.Join(parsed.Skills, ", ")

	insertQuery := `
		INSERT OR IGNORE INTO linkedin_job_description 
		(job_id, job_link, job_description, job_type, skills) 
		VALUES (?, ?, ?, ?, ?)
	`
	_, err = db.Exec(insertQuery, jobID, jobLink, parsed.Description, parsed.JobType, skillsCSV)
	if err != nil {
		return fmt.Errorf("failed to insert job description: %v", err)
	}

	return nil
}
