package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"job_scraper/config"
	"job_scraper/scraper"
	"job_scraper/scraper/Linkedin"
	"job_scraper/scraper/Xing"

)

func suppressLogs() {
	log.SetOutput(ioutil.Discard) // Disables all logs
}

func main() {
	// Initialize the database
	db, err := config.InitializeDatabase() // No import needed as it's in the same package
	if err != nil {
		log.Fatalf("❌ Failed to initialize the database: %v", err)
	}
	defer db.Close() // Ensure the database is closed when the program exits

	// Set up a channel to listen for an interrupt signal (Ctrl+C)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Create a new HTTP request multiplexer (mux)
	mux := http.NewServeMux()

	// Define the routes and their handlers
	mux.HandleFunc("/joblistings", func(w http.ResponseWriter, r *http.Request) {
		scraper.JobListingsHandler(w, r, db) 
	})
	
	mux.HandleFunc("/loginlinkedin", func(w http.ResponseWriter, r *http.Request) {
		Linkedin.LoginLinkedInHandler(db,w,r)
	})

	



	mux.HandleFunc("/loginxing", func(w http.ResponseWriter, r *http.Request) {
		Xing.LoginXingHandler(db,w,r)
	})
	mux.HandleFunc("/viewxingjobs", func(w http.ResponseWriter, r *http.Request) {
		Xing.ViewXingJobs(db,w,r)
	})

	


	mux.HandleFunc("/viewlinkedmetadata", func(w http.ResponseWriter, r *http.Request) {
		scraper.ViewJobsHandler(w, r, db)
	})

	mux.HandleFunc("/viewlinkedinjobs", func(w http.ResponseWriter, r *http.Request) {
		Linkedin.ViewLinkedInJobs(db, w, r)
	})

	mux.HandleFunc("/viewlinkedindesc", func(w http.ResponseWriter, r *http.Request) {
		Linkedin.ViewLinkedInJobDescriptions(db, w, r)
	})


	/*
	mux.HandleFunc("/viewlinkedinfailedjobs", func(w http.ResponseWriter, r *http.Request) {
		Linkedin.ViewLinkedInFailedJobs(db, w, r)
	})
		*/

	//mux.HandleFunc("/uploaddb", func(w http.ResponseWriter, r *http.Request) {
	//	Linkedin.PostDBHandler(w, r, db)  // Pass the db to the handler
	//})

	// Enable CORS support
	handler := enableCors(mux)

	// Define the server port
	port := ":8000"
	server := &http.Server{
		Addr:    port,
		Handler: handler,
	}

	// Start the HTTP server in another goroutine
	go func() {
		fmt.Printf("🚀 Server is running on http://localhost%s\n", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ Server failed: %v", err)
		}
	}()

	// Wait for an interrupt signal to gracefully shut down the server and the browser
	<-quit

	// Gracefully shut down the server
	fmt.Println("\n🚪 Shutting down the server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("❌ Server shutdown failed: %v", err)
	}

	// Terminate the browser (chromedp or any other process you are running)
	fmt.Println("🖥️ Terminating browser...")
	if err := scraper.TerminateBrowser(); err != nil {
		log.Printf("❌ Failed to terminate the browser: %v", err)
	} else {
		fmt.Println("✅ Browser terminated successfully.")
	}

	fmt.Println("✅ Server shut down gracefully.")
}

// enableCors adds CORS headers to allow cross-origin requests
func enableCors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allowedOrigin := "http://localhost:3000" // Frontend server URL or adjust if needed
		w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		// Respond to OPTIONS pre-flight request
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// Serve the request to the next handler
		next.ServeHTTP(w, r)
	})
}
