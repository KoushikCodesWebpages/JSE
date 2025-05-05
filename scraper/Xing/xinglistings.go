package Xing

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"
	"runtime"
	"os/exec"
	"github.com/chromedp/chromedp"
	"github.com/google/uuid"

)
// Job struct for both LinkedIn and Xing
type Job struct {
	UUID        string `json:"uuid"`
	JobID       string `json:"jobId"` // Changed to string to match xing_jobs table
	Title       string `json:"title"`
	Company     string `json:"company"`
	Location    string `json:"location"`
	PostedDate  string `json:"postedDate"`
	Link        string `json:"link"`
	IsEasyApply bool   `json:"isEasyApply"` // Used only for LinkedIn
	Processed   bool   `json:"processed"`
}

func StartChrome() (*exec.Cmd, error) {
	chromePath := "chromium" // or use full path

	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	cmd := exec.Command(chromePath,
		"--remote-debugging-port=9222",
		"--profile-directory=Profile 6",
		"--window-size=800,600", 
		"https://www.xing.com/",
	)

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start Chromium: %w", err)
	}
	return cmd, nil
}

func waitForChromeDebugPort(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url + "/json/version")
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("chrome debugger not responding at %s", url)
}


func constructXingSearchURL(keywords, location string) string {
	return fmt.Sprintf(
		"https://www.xing.com/jobs/search?keywords=%s&location=%s",
		strings.ReplaceAll(keywords, " ", "%20"),
		strings.ReplaceAll(location, " ", "%20"),
	)
}
func insertJobIfNotExists(db *sql.DB, job Job) error {
    _, err := db.Exec(`
        INSERT INTO xing_jobs (id, jobid, title, company, location, posted_date, link, processed)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
        uuid.New().String(), job.JobID, job.Title, job.Company, job.Location, job.PostedDate, job.Link, false,
    )

    if err != nil {
        if strings.Contains(err.Error(), "UNIQUE constraint failed") {
            return fmt.Errorf("❌ Job already exists: %v", err)
        }
        return fmt.Errorf("❌ Failed to insert job: %v", err)
    }

    fmt.Printf("✅ Inserted new job: %s\n", job.Title)
    return nil
}


func extractXingJobID(link string) string {
	if link == "" {
		return ""
	}
	parts := strings.Split(link, "-")
	lastPart := parts[len(parts)-1]

	// Remove any query params or anchors, just in case
	if idx := strings.IndexAny(lastPart, "?#"); idx != -1 {
		lastPart = lastPart[:idx]
	}

	// Make sure it's numeric
	for _, ch := range lastPart {
		if ch < '0' || ch > '9' {
			return ""
		}
	}

	return lastPart
}

func fetchAndStoreXingJobs(ctx context.Context, db *sql.DB, jobTitles []string, location string) error {
	for _, title := range jobTitles {
		searchURL := constructXingSearchURL(title, location)
		var jobs []Job

		err := chromedp.Run(ctx,
			chromedp.Navigate(searchURL),
			chromedp.Sleep(4*time.Second), // Give the page some time to load

			// Scroll to bottom to trigger lazy loading
			chromedp.ActionFunc(func(ctx context.Context) error {
				return chromedp.Evaluate(`window.scrollTo(0, document.body.scrollHeight);`, nil).Do(ctx)
			}),
			chromedp.Sleep(2*time.Second),
			chromedp.ActionFunc(func(ctx context.Context) error {
				return chromedp.Evaluate(`window.scrollTo(0, document.body.scrollHeight);`, nil).Do(ctx)
			}),
			chromedp.Sleep(2*time.Second),
			chromedp.ActionFunc(func(ctx context.Context) error {
				return chromedp.Evaluate(`window.scrollTo(0, document.body.scrollHeight);`, nil).Do(ctx)
			}),
			chromedp.Sleep(2*time.Second),
			chromedp.ActionFunc(func(ctx context.Context) error {
				return chromedp.Evaluate(`window.scrollTo(0, document.body.scrollHeight);`, nil).Do(ctx)
			}),
			chromedp.Sleep(2*time.Second),
			chromedp.ActionFunc(func(ctx context.Context) error {
				return chromedp.Evaluate(`window.scrollTo(0, document.body.scrollHeight);`, nil).Do(ctx)
			}),
			chromedp.Sleep(2*time.Second),
			chromedp.Sleep(2*time.Second),
			chromedp.ActionFunc(func(ctx context.Context) error {
				return chromedp.Evaluate(`window.scrollTo(0, document.body.scrollHeight);`, nil).Do(ctx)
			}),
			chromedp.Sleep(2*time.Second),
			// Wait for job listings to appear
			chromedp.WaitVisible(`[data-testid="job-search-result"]`, chromedp.ByQuery),
			

			// Extract job data
			chromedp.Evaluate(`Array.from(document.querySelectorAll('[data-testid="job-search-result"]')).map(el => ({
				title: "`+title+`",
				link: 'https://www.xing.com' + (el.getAttribute('href') || ''),
				company: el.querySelector('[data-xds="BodyCopy"].job-teaser-list-item-styles__Company-sc-4c7b5190-7')?.innerText.trim() || 'Unknown',
				location: el.querySelector('[data-xds="BodyCopy"].job-teaser-list-item-styles__City-sc-4c7b5190-6')?.innerText.trim() || 'Unknown',
				postedDate: '',
				isEasyApply: false
			}))`, &jobs),
		)

		if err != nil {
			fmt.Printf("❌ Failed to fetch Xing jobs for %s: %v\n", title, err)
			continue
		}

		count := 0
		for _, job := range jobs {
			if job.Link != "" {
				job.JobID = extractXingJobID(job.Link)
				if err := insertJobIfNotExists(db, job); err != nil {
					fmt.Printf("⚠️ Insert error for job %s: %v\n", job.Link, err)
					continue
				}
				count++
				if count >= 5 {
					break
				}
			}
		}
		fmt.Printf("✅ Stored %d new Xing jobs for %s (excluding duplicates)\n", count, title)
	}
	return nil
}

func XingJobListingsHandler(ctx context.Context, db *sql.DB) error {
	// Start headless Chrome
	cmd, err := StartChrome()
	if err != nil {
		return fmt.Errorf("failed to start Chrome: %w", err)
	}
	defer cmd.Process.Kill()

	// Wait for Chrome to be ready for remote debugging
	err = waitForChromeDebugPort("http://localhost:9222", 10*time.Second)
	if err != nil {
		return fmt.Errorf("chrome debugger not ready: %w", err)
	}

	// Create ChromeDP context connected to the remote instance
	allocatorCtx, cancel := chromedp.NewRemoteAllocator(ctx, "http://localhost:9222")
	defer cancel()

	chromeCtx, cancelChrome := chromedp.NewContext(allocatorCtx)
	defer cancelChrome()

	// Define job titles and location

	jobTitles := []string{
		"Mechanical Engineer",
		// "Design Engineer",
		// "Manufacturing Engineer",
		// "Automation Engineer",
		// "Electrical Engineer",
		// "Power Engineer",
		// "Control Systems Engineer",
		// "Electronics Engineer",
		// "Civil Engineer",
		// "Structural Engineer",
		// "Construction Manager",
		// "Transportation Engineer",

		// "Software Developer",
		// "Frontend Developer",
		// "Backend Developer",
		// "Full Stack Developer",
		// "Data Scientist",
		// "Data Engineer",
		// "Machine Learning Engineer",
		// "Data Analyst",
		// "Cybersecurity Specialist",
		// "Network Security Engineer", 
		// "Penetration Tester",
		// "Incident Response Analyst",
		// "AI/ML Engineer",
		// "Machine Learning Engineer",
		// "AI Research Scientist",
		// "Natural Language Processing Engineer",
		// "Automotive Engineer",
		// "Automotive Systems Engineer",
		// "Vehicle Integration Engineer",
		// "Testing Engineer",
		// "Robotics Engineer",
		// "Robotic Systems Designer",
		// "Robot Programmer",
		// "Automation Engineer",
		// "Environmental Engineer",
		// "Water Resources Engineer",
		// "Waste Management Engineer",
		// "Sustainability Engineer",
		// "Chemical Engineer",
		// "Process Engineer",
		// "Materials Engineer",
		// "Biochemical Engineer",
		// "Industrial Engineer",
		// "Operations Research Analyst",
		// "Manufacturing Engineer",
		// "Supply Chain Engineer",
		// "Construction Engineer",
		// "Site Engineer",
		// "Cost Engineer",
		// "Health & Safety Engineer",
		// "HVAC Engineer",
		// "Heating Engineer",
		// "Ventilation Engineer",
		// "Air Conditioning Engineer",
		// "Mechatronics Engineer",
		// "Automation Engineer",
		// "Control Systems Engineer",
		// "Embedded Systems Engineer",
		// "Telecommunications Engineer",
		// "Network Engineer",
		// "Radio Frequency (RF) Engineer",
		// "Telecom Systems Engineer",
		// "Aerospace Engineer",
		// "Aircraft Systems Engineer",
		// "Spacecraft Engineer",
		// "Flight Test Engineer",
		// "Marine Engineer",
		// "Ship Design Engineer",
		// "Marine Systems Engineer",
		// "Naval Architect",
		// "Geotechnical Engineer",
		// "Soil Engineer",
		// "Rock Mechanics Engineer",
		// "Foundation Engineer",
		// "Nuclear Engineer",
		// "Radiation Protection Engineer",
		// "Reactor Engineer",
		// "Nuclear Safety Engineer",
		// "Project Manager",
		// "IT Project Manager",
		// "Construction Project Manager",
		// "Agile Project Manager",
		// "Product Manager",
		// "Digital Product Manager",
		// "Mobile Product Manager",
		// "E-commerce Product Manager",
		// "Operations Manager",
		// "Supply Chain Operations Manager",
		// "Manufacturing Operations Manager",
		// "Facility Operations Manager",
		// "Supply Chain Manager",
		// "Logistics Manager",
		// "Inventory Manager",
		// "Procurement Manager",
		// "HR Manager",
		// "Talent Acquisition Manager",
		// "Employee Relations Manager",
		// "Compensation and Benefits Manager",
		// "Financial Manager",
		// "Risk Manager",
		// "Treasury Manager",
		// "Compliance Manager",
		// "Marketing Manager",
		// "Brand Manager",
		// "Content Marketing Manager",
		// "Social Media Manager",
		// "Sales Manager",
		// "Regional Sales Manager",
		// "Inside Sales Manager",
		// "Key Account Manager",
		// "IT Manager",
		// "Systems Manager",
		// "Network Manager",
		// "Security Manager",
		// "Business Development Manager",
		// "Partnerships Manager",
		// "Sales Development Manager",
		// "Market Development Manager",
		// "Risk Manager",
		// "Insurance Risk Manager",
		// "Operational Risk Manager",
		// "Credit Risk Manager",
		// "Legal Manager",
		// "Contract Manager",
		// "Compliance Manager",
		// "Corporate Counsel",
		// "Quality Assurance Manager",
		// "Test Manager",
		// "Process Improvement Manager",
		// "Auditing Manager",
		// "Customer Service Manager",
		// "Call Center Manager",
		// "Client Relations Manager",
		// "Technical Support Manager",
		// "Procurement Manager",
		// "Sourcing Manager",
		// "Vendor Manager",
		// "Purchasing Manager",
		// "Compliance Manager",
		// "Regulatory Affairs Manager",
		// "Internal Auditor",
		// "Environmental Compliance Manager",
		// "Change Manager",
		// "Organizational Development Manager",
		// "Transition Manager",
		// "Change Communication Manager",
		// "Corporate Strategy Manager",
		// "Strategic Planning Manager",
		// "Business Analyst",
		// "Mergers and Acquisitions Manager",
		// "Training and Development Manager",
		// "Learning and Development Specialist",
		// "Employee Engagement Manager",
		// "Leadership Development Manager",
		// "Facility Manager",
		// "Building Operations Manager",
		// "Maintenance Manager",
		// "Space Planner",
	}
	location := "Berlin, Germany"

	// Fetch and store Xing jobs
	return fetchAndStoreXingJobs(chromeCtx, db, jobTitles, location)
}

