// scraper/terminate.go
package scraper

import (
	"fmt"
	"os/exec"
)

func TerminateBrowser() error {
	// Assuming you're using chromium or any browser, kill it
	// This is an example for terminating Chromium; adjust as needed for your use case
	cmd := exec.Command("pkill", "chromium") // For Unix-like systems, this kills the Chromium process
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to terminate browser: %v", err)
	}
	return nil
}
