#!/bin/bash
# Test script for soft deletion of knowledge files

# Set variables
FILE_ID="test-soft-delete-$(date +%s)"
AGENT_ID="test-agent"
SERVICE_URL="https://wavie-claude-proxy-s-455488113475.us-central1.run.app"

echo "Testing soft deletion of knowledge files"
echo "----------------------------------------"
echo "File ID: $FILE_ID"

# Step 1: Create a test file and ZIP it
echo "Step 1: Creating test file and ZIP archive..."
echo "Test content for soft deletion" > /tmp/test-content.md
cd /tmp
zip -j test-soft-delete.zip test-content.md
cd -

# Step 2: Upload the file and get the file ID from the response
echo "Step 2: Uploading test file..."
UPLOAD_RESPONSE=$(curl -X POST \
  -F "file=@/tmp/test-soft-delete.zip" \
  -F "agent_ids=$AGENT_ID" \
  -F "name=test-soft-delete" \
  -F "description=Test file for soft deletion" \
  "$SERVICE_URL/api/knowledge/upload")

echo "Upload response: $UPLOAD_RESPONSE"

# Step 3: Extract the file ID from the upload response
echo "Step 3: Extracting file ID from upload response..."
FILE_ID=$(echo $UPLOAD_RESPONSE | jq -r '.file_id')
echo "File ID from upload: $FILE_ID"

# Step 4: Check if file exists in registry before deletion
echo "Step 4: Checking if file exists in registry before deletion..."
FILE_EXISTS=$(curl -s "$SERVICE_URL/api/knowledge/files" | jq '.files[] | select(.id=="'$FILE_ID'")' | wc -l)
if [ "$FILE_EXISTS" -gt 0 ]; then
  echo "File found in registry before deletion"
else
  echo "File NOT found in registry before deletion - something went wrong"
  exit 1
fi

# Step 5: Delete the file (soft delete)
echo "Step 5: Soft deleting the file..."
curl -X POST "$SERVICE_URL/api/knowledge/files/delete" \
  -H "Content-Type: application/json" \
  -d '{"ID": "'$FILE_ID'"}' \
  | jq .

# Step 6: Verify the file is removed from registry
echo "\nStep 6: Verifying file is removed from registry..."
FILE_EXISTS_AFTER=$(curl -s "$SERVICE_URL/api/knowledge/files" | jq '.files[] | select(.id=="'$FILE_ID'")' | wc -l)
if [ "$FILE_EXISTS_AFTER" -eq 0 ]; then
  echo "Success: File was removed from registry"
else
  echo "Error: File still exists in registry after deletion"
  exit 1
fi

echo "----------------------------------------"
echo "Test completed successfully - file was soft deleted (removed from registry)"
echo "The actual files remain in GCS but are inaccessible to the application"
