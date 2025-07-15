#!/bin/bash

# Service URL
SERVICE_URL="https://wavie-claude-proxy-s-455488113475.us-central1.run.app"

# File IDs to delete
FILE_IDS=(
  "7bc2b0a0-a0c7-438b-9c73-05d3377694c7"
  "61dec940-413f-4fbd-82c0-afb691a5541e"
  "9f136bca-221f-45c2-a188-0d80b7c3155e"
  "a5083790-fe80-46fa-883e-bbab4618d399"
  "8d7e2766-af41-4064-bde7-2d59614edfa1"
)

echo "Starting batch deletion of all files..."
echo "----------------------------------------"

# Delete each file
for file_id in "${FILE_IDS[@]}"; do
  echo "Deleting file ID: $file_id"
  
  # Call the delete API
  response=$(curl -s -X POST "$SERVICE_URL/api/knowledge/files/delete" \
    -H "Content-Type: application/json" \
    -d "{\"ID\": \"$file_id\"}")
  
  # Check if deletion was successful
  if echo "$response" | grep -q "\"success\":true"; then
    echo "✅ Successfully deleted file: $file_id"
  else
    echo "❌ Failed to delete file: $file_id"
    echo "Response: $response"
  fi
  
  echo "----------------------------------------"
  
  # Small delay between requests to avoid overwhelming the server
  sleep 1
done

echo "Batch deletion completed."
echo "Verifying all files are removed from registry..."

# Get the current list of files
files_response=$(curl -s "$SERVICE_URL/api/knowledge/files")
remaining_files=$(echo "$files_response" | grep -o "\"id\":\"[^\"]*\"" | wc -l)

echo "Remaining files in registry: $remaining_files"

if [ "$remaining_files" -eq 0 ]; then
  echo "✅ All files successfully removed from registry!"
else
  echo "⚠️ Some files may still remain in the registry."
  echo "Remaining files:"
  echo "$files_response" | jq '.files[].id'
fi
