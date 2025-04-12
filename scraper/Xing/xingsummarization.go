package Xing

import (

	"encoding/json"
	"fmt"
	"log"
	"net/http"
	//"runtime"
	//"strconv"
	"os"
	"strings"
	"bytes"
	"io"
	"database/sql"
	"github.com/joho/godotenv"
)

// Load .env on init
func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("❌ Error loading .env file")
	}
}


type HuggingFaceResponse struct {
	GeneratedText string `json:"generated_text"`
}

type HuggingFaceAPI struct {
	URL string
	Key string
}

var huggingFaceAPIs []HuggingFaceAPI

func init() {
	apiKey := os.Getenv("HF_API_KEY")

	// Loop through 10 model URLs: HF_API_URL_1 to HF_API_URL_10
	for i := 1; i <= 10; i++ {
		envVar := fmt.Sprintf("HF_MODEL_%d", i)
		modelURL := os.Getenv(envVar)
		if modelURL != "" {
			huggingFaceAPIs = append(huggingFaceAPIs, HuggingFaceAPI{
				URL: modelURL,
				Key: apiKey,
			})
		}
	}
}

func extractStructuredSummary(jobDescription string) (string, error) {
	for _, api := range huggingFaceAPIs {
		output, err := callHuggingFaceAPI(api.URL, api.Key, jobDescription)
		if err == nil && output != "" {
			return output, nil
		}
		log.Printf("⚠️ Failed with %s: %v", api.URL, err)
	}
	return "", fmt.Errorf("all Hugging Face API calls failed")
}



func callHuggingFaceAPI(apiURL, apiKey, jobDescription string) (string, error) {
	prompt := fmt.Sprintf(`Extract structured job details from the following job description and return in JSON format with fields:: 
- "job_type (remote, part time, full time, unknown)"
- "skills "
- "description"

Description: "%s"`, jobDescription)

	payload := map[string]interface{}{
		"inputs": prompt,
		"parameters": map[string]interface{}{
			"max_length":  1000,
			"temperature": 0.3,
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request payload: %v", err)
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("hugging face request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	var result []HuggingFaceResponse
	err = json.Unmarshal(body, &result)
	if err != nil || len(result) == 0 {
		return "", fmt.Errorf("failed to parse API response: %v", err)
	}

	output := result[0].GeneratedText

	// Extract JSON block from output text
	jsonStart := strings.Index(output, "{")
	jsonEnd := strings.LastIndex(output, "}")
	if jsonStart != -1 && jsonEnd != -1 && jsonEnd > jsonStart {
		output = output[jsonStart : jsonEnd+1]
	}

	return strings.TrimSpace(output), nil
}


type FlexibleJobSummary struct {
	JobType     string   `json:"job_type"`
	Skills      []string `json:"skills"`
	Description string   `json:"description"`
}

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

// Store the summary as the job description and mark job as processed
func storeJobDescription(db *sql.DB, jobID, jobLink, summary string) error {
	var parsed FlexibleJobSummary
	err := json.Unmarshal([]byte(summary), &parsed)
	if err != nil {
		return fmt.Errorf("failed to parse structured summary: %v", err)
	}

	skillsCSV := strings.Join(parsed.Skills, ", ")

	insertQuery := `
		INSERT OR IGNORE INTO xing_job_description 
		(job_id, job_link, job_description, job_type, skills) 
		VALUES (?, ?, ?, ?, ?)
	`
	_, err = db.Exec(insertQuery, jobID, jobLink, parsed.Description, parsed.JobType, skillsCSV)
	if err != nil {
		return fmt.Errorf("failed to insert job description: %v", err)
	}

	return nil
}
