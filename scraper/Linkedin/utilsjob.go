package Linkedin

import (
	"context"
	"fmt"
	"log"
	"time"
	"database/sql"
	"strings"
	//"path/filepath"
	 "regexp"
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
func cleanJobDescription(raw string) string {
    // Normalize line breaks and trim spaces
    lines := strings.Split(raw, "\n")
    var cleaned []string

    for _, line := range lines {
        line = strings.TrimSpace(line)
        if line == "" {
            continue
        }
        cleaned = append(cleaned, line)
    }

    // Merge the cleaned lines into a paragraph-like structure
    result := strings.Join(cleaned, "\n")

    // Fix common issues like "..", excessive spaces, etc.
    result = regexp.MustCompile(`\.\.+`).ReplaceAllString(result, ".")
    result = regexp.MustCompile(`\s+`).ReplaceAllString(result, " ")

    return result
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


	cleanedDescription := cleanJobDescription(rawDescription)
	// log.Printf("📄 Cleaned Description:\n%s\n", cleanedDescription)

	// 3. Use Hugging Face to summarize and structure the description
	summary, err := extractStructuredSummary(cleanedDescription)
	if err != nil {
		log.Printf("⚠️ Failed to summarize job description for jobID %s: %v\n", jobID, err)
	}

	log.Printf("📦 Summary:\n%s\n", summary)

	
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

func extractStructuredSummary(jobDescription string) (string, error) {
	output, err := callOllamaAPI(jobDescription)
	if err != nil || output == "" {
		log.Printf("⚠️ Ollama call failed: %v\n", err)
		return "", err
	}

	// log.Printf("📤 Ollama Raw Output:\n%s\n", output)

	start := strings.Index(output, "{")
	end := strings.LastIndex(output, "}")
	if start != -1 && end != -1 && end > start {
		jsonPart := output[start : end+1]
		// log.Printf("🧪 Extracted JSON:\n%s\n", jsonPart)
		return jsonPart, nil
	}


	// Manual fallback
	summary := FlexibleJobSummary{
		JobType:     extractAfter(output, "Job Type:", "\n"),
		Skills:      splitSkills(extractAfter(output, "Skills Required:", "\n")),
		Description: extractAfter(output, "Description:", ""),
	}

	jsonBytes, err := json.Marshal(summary)
	if err != nil {
		return "", fmt.Errorf("failed to marshal fallback JSON: %v", err)
	}
 
	return string(jsonBytes), nil
}

func callOllamaAPI(jobDescription string) (string, error) {
	prompt := fmt.Sprintf(`
	Extract and return the following from this job posting as JSON:
	{
	  "job_type": "One word like Remote, On-site, or Hybrid",
	  "skills": ["List at least 5 key technical skills or tools"],
	  "description": "Professional summary of the role in full sentences(20 lines or 500 words)"
	}
	
	Only return valid JSON. No extra text.
	
	Job posting:
	"%s"
	`, jobDescription)
	


	payload := map[string]interface{}{
		"model":  "mistral",
		"prompt": prompt,
		"stream": false,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request payload: %v", err)
	}

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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	var response OllamaResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return "", fmt.Errorf("failed to parse Ollama response: %v", err)
	}

	return response.Response, nil
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

func extractAfter(text, key, end string) string {
	idx := strings.Index(text, key)
	if idx == -1 {
		return ""
	}
	start := idx + len(key)
	if end == "" {
		return strings.TrimSpace(text[start:])
	}
	endIdx := strings.Index(text[start:], end)
	if endIdx == -1 {
		return strings.TrimSpace(text[start:])
	}
	return strings.TrimSpace(text[start : start+endIdx])
}

func splitSkills(raw string) []string {
	raw = strings.ReplaceAll(raw, "•", "")
	parts := strings.Split(raw, ",")
	var skills []string
	for _, s := range parts {
		skill := strings.TrimSpace(s)
		if skill != "" {
			skills = append(skills, skill)
		}
	}
	return skills
}
// Store the summary as the job description and mark job as processed
func storeJobDescription(db *sql.DB, jobID, jobLink, summary string) error {
	var parsed FlexibleJobSummary
	err := json.Unmarshal([]byte(summary), &parsed)
	if err != nil {
		log.Printf("❌ Unmarshal failed for summary:\n%s\n", summary)
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