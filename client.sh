#!/bin/bash

BASE_URL="http://localhost:5000"

echo "Starting API calls..."

function call_api_and_wait() {
    local endpoint=$1
    local success_message=$2
    echo "Calling $BASE_URL$endpoint..."

    # Call the API and continuously display logs while waiting for success message
    curl -s -N "$BASE_URL$endpoint" | while read -r line; do
        echo "$line"  # Print each line of the output

        # Check if the success message appears in the response
        if [[ "$line" == *"$success_message"* ]]; then
            echo "$success_message"
            break
        fi
    done
}

# Call each API and wait for the success message
call_api_and_wait "/joblistings" "📂 Job listings saved in job_links.csv"
call_api_and_wait "/loginlinkedin" "✅ Job application automation completed."
call_api_and_wait "/uploaddb" "✅ Connected to Azure SQL successfully!"

echo "✅ All API calls completed."
