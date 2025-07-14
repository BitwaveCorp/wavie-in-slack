# Working cURL Requests for RAG Service

## Document Upload
```bash
curl -X POST -H "Content-Type: application/json" -d '{
  "document_id": "test-vector-search",
  "file_path": "test/test-vector-search.md"
}' https://wavie-rag-svc-455488113475.us-central1.run.app/api/documents
```

Response:
```json
{"status":"success","message":"Document processed successfully. Estimated cost: $0.000004","document_id":"test-vector-search","chunk_count":1}
```

## Document Deletion
```bash
curl -X DELETE https://wavie-rag-svc-455488113475.us-central1.run.app/api/documents/test-vector-search
```

Response:
```json
{"status":"success","message":"Document embeddings deleted successfully","document_id":"test-vector-search"}
```

## Health Check
```bash
curl -s https://wavie-rag-svc-455488113475.us-central1.run.app/health
```

Response:
```json
{"status":"ok","timestamp":"2025-07-14T12:16:56.393281701Z","version":"1.0.0"}
```

## Question Answering (Currently Not Working)
```bash
curl -X POST -H "Content-Type: application/json" -d '{"question": "What is Wavie?"}' https://wavie-rag-svc-455488113475.us-central1.run.app/api/ask
```

Error Response:
```
Failed to find similar chunks: API request failed with status code 501: {
  "error": {
    "code": 501,
    "message": "Operation is not implemented, or supported, or enabled.",
    "status": "UNIMPLEMENTED"
  }
}
```

## File Path Format
When uploading documents, the file path should be in the format:
```
test/filename.md
```

For files extracted from ZIP archives, the document ID is typically:
```
fileID-relativePath
```
