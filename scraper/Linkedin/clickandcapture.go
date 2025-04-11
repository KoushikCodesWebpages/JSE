package Linkedin

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	//"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/cdproto/target"
)

// StopChrome closes the Chromium browser process
func StopChrome() {
	if chromeCmd != nil {
		fmt.Println("🛑 Closing Chromium...")
		if err := chromeCmd.Process.Kill(); err != nil {
			fmt.Printf("⚠️ Failed to close Chromium: %v\n", err)
		} else {
			fmt.Println("✅ Chromium closed successfully.")
		}
	}
}

func captureAndCloseNewTab(ctx context.Context, db *sql.DB, jobID string, existingTabs map[target.ID]struct{}) error {
	var newTabID target.ID
	var cleanURL string

	// Get all open tabs
	newTabs, err := chromedp.Targets(ctx)
	if err != nil {
		log.Printf("❌ Failed to get updated open tabs: %v\n", err)
		return err
	}

	// Find new tab that is NOT a LinkedIn page
	for _, t := range newTabs {
		if _, exists := existingTabs[t.TargetID]; !exists && t.Type == "page" && t.URL != "" && !strings.Contains(t.URL, "linkedin.com") {
			cleanURL = strings.TrimSpace(t.URL)
			if cleanURL == "" {
				continue
			}

			newTabID = t.TargetID
			break
		}
	}

	if newTabID == "" {
		log.Printf("⚠️ No new non-LinkedIn tab found.")
		return nil
	}

	// Create a context bound to the new tab
	tabCtx, cancel := chromedp.NewContext(ctx, chromedp.WithTargetID(newTabID))
	defer cancel()

	fmt.Println("🔍 Processing external application page:", cleanURL)

	// ✅ Store application link
	if err := StoreApplicationLink(db, jobID, cleanURL); err != nil {
		log.Printf("❌ Error storing application link in DB: %v\n", err)
	} else {
		fmt.Println("✅ Captured and stored application page:", cleanURL)
	}

	// 🧠 Scrape job description from the open tab

	// 🛑 Close the new tab
	err = chromedp.Run(tabCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		return target.CloseTarget(newTabID).Do(ctx)
	}))
	if err != nil {
		log.Printf("❌ Error closing tab: %v\n", err)
	} else {
		fmt.Println("✅ Successfully closed extra tab:", newTabID)
	}

	return nil
}

