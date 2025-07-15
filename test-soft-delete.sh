#!/bin/bash
# Test script for soft deletion of knowledge files

# Set variables
FILE_ID="test-soft-delete-$(date +%s)"
AGENT_ID="test-agent"

echo "Testing soft deletion of knowledge files"
echo "----------------------------------------"
echo "File ID: $FILE_ID"

# Step 1: Create a test file
echo "Step 1: Creating test file..."
echo "Test content for soft deletion" > /tmp/test-soft-delete.txt

# Step 2: Upload the file
echo "Step 2: Uploading test file..."
curl -X POST \
  -F "file=@/tmp/test-soft-delete.txt" \
  -F "agent_id=$AGENT_ID" \
  -F "file_name=test-soft-delete.txt" \
  "http://localhost:8080/api/knowledge/upload" | jq .

# Step 3: Get the file ID from the registry
echo "Step 3: Getting file ID from registry..."
FILE_ID=$(curl -s "http://localhost:8080/api/knowledge/files" | jq -r '.files[] | select(.name=="test-soft-delete.txt") | .id')
echo "File ID from registry: $FILE_ID"

# Step 4: Verify the file exists in GCS
echo "Step 4: Verifying file exists in GCS..."
gsutil ls "gs://your-bucket-name/files/$FILE_ID/" || echo "File not found in GCS"

# Step 5: Delete the file (soft delete)
echo "Step 5: Soft deleting the file..."
curl -X DELETE "http://localhost:8080/api/knowledge/files/$FILE_ID" | jq .

# Step 6: Verify the file is removed from registry
echo "Step 6: Verifying file is removed from registry..."
curl -s "http://localhost:8080/api/knowledge/files" | jq '.files[] | select(.id=="'$FILE_ID'")'
echo "If no output above, file was successfully removed from registry"

# Step 7: Verify the file still exists in GCS
echo "Step 7: Verifying file still exists in GCS after soft delete..."
gsutil ls "gs://your-bucket-name/files/$FILE_ID/" || echo "File not found in GCS"

echo "----------------------------------------"
echo "Test completed"
