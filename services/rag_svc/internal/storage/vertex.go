package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"cloud.google.com/go/aiplatform/apiv1/aiplatformpb"
	"cloud.google.com/go/firestore"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/iterator"
)

// VectorStore handles vector storage and retrieval
type VectorStore struct {
	projectID         string
	location          string
	firestoreClient   *firestore.Client
	httpClient        *http.Client
	indexPath         string
	indexEndpointPath string
	deployedIndexID   string
}

// NewVectorStore creates a new vector store
func NewVectorStore(ctx context.Context, projectID, location, indexID, indexEndpointID, deployedIndexID string) (*VectorStore, error) {
	// Log the configuration parameters
	log.Printf("Initializing VectorStore with:\n" +
		"  Project ID: %s\n" +
		"  Location: %s\n" +
		"  Index ID: %s\n" +
		"  Index Endpoint ID: %s\n" +
		"  Deployed Index ID: %s",
		projectID, location, indexID, indexEndpointID, deployedIndexID)
	
	// Use the correct deployed index ID if the provided one doesn't match the expected format
	if deployedIndexID != "wavie_embeds_1752410897069" {
		log.Printf("WARNING: Provided deployed index ID '%s' doesn't match the expected ID. Using 'wavie_embeds_1752410897069' instead.", deployedIndexID)
		deployedIndexID = "wavie_embeds_1752410897069"
	}
	firestoreClient, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to create Firestore client: %v", err)
	}

	// Create HTTP client with Google API credentials
	credentials, err := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return nil, fmt.Errorf("failed to get default credentials: %v", err)
	}

	tokenSource := credentials.TokenSource
	httpClient := oauth2.NewClient(ctx, tokenSource)

	// Create resource paths
	indexPath := fmt.Sprintf("projects/%s/locations/%s/indexes/%s", projectID, location, indexID)
	indexEndpointPath := fmt.Sprintf("projects/%s/locations/%s/indexEndpoints/%s", projectID, location, indexEndpointID)

	return &VectorStore{
		projectID:         projectID,
		location:          location,
		firestoreClient:   firestoreClient,
		httpClient:        httpClient,
		indexPath:         indexPath,
		indexEndpointPath: indexEndpointPath,
		deployedIndexID:   deployedIndexID,
	}, nil
}

// Close closes the vector store
func (s *VectorStore) Close() error {
	// Only the Firestore client needs to be closed
	if s.firestoreClient != nil {
		return s.firestoreClient.Close()
	}

	// Note: httpClient doesn't need to be closed
	return nil
}

// StoreEmbeddings stores document chunks and their embeddings
func (s *VectorStore) StoreEmbeddings(ctx context.Context, documentID string, chunks []string, embeddings [][]float32) error {
	// 1. Store text chunks in Firestore
	batch := s.firestoreClient.Batch()

	for i, chunk := range chunks {
		chunkID := fmt.Sprintf("%s-%d", documentID, i)
		ref := s.firestoreClient.Collection("document_chunks").Doc(chunkID)

		batch.Set(ref, map[string]interface{}{
			"document_id": documentID,
			"chunk_index": i,
			"content":     chunk,
			"created_at":  time.Now(),
		})
	}

	_, err := batch.Commit(ctx)
	if err != nil {
		return fmt.Errorf("failed to store chunks in Firestore: %v", err)
	}

	// 2. Store vectors in Vertex AI Vector Search
	var datapoints []*aiplatformpb.IndexDatapoint

	for i, embedding := range embeddings {
		chunkID := fmt.Sprintf("%s-%d", documentID, i)

		// We're storing metadata in Firestore, so we don't need it in the datapoint
		// But keeping this code commented for reference if needed in the future
		/*
			metadata, err := structpb.NewStruct(map[string]interface{}{
				"document_id": documentID,
				"chunk_index": i,
			})
			if err != nil {
				return fmt.Errorf("failed to create metadata: %v", err)
			}
		*/

		// Create datapoint with embedding
		datapoint := &aiplatformpb.IndexDatapoint{
			DatapointId:   chunkID,
			FeatureVector: embedding,
		}

		datapoints = append(datapoints, datapoint)
	}

	// Create the request URL for UpsertDatapoints
	url := fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/%s:upsertDatapoints",
		s.location, s.indexPath)

	// Create the request body
	reqBody := map[string]interface{}{
		"datapoints": datapoints,
	}

	// Marshal the request body to JSON
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %v", err)
	}

	// Create the HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Execute the request
	response, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute HTTP request: %v", err)
	}
	defer response.Body.Close()

	// Read the response body
	respBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	// Check for non-200 status code
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("API request failed with status code %d: %s", response.StatusCode, string(respBody))
	}

	return nil
}

// FindSimilarChunks finds similar chunks for a query embedding
func (s *VectorStore) FindSimilarChunks(ctx context.Context, queryEmbedding []float32, limit int) ([]string, error) {
	// Log the configuration being used
	log.Printf("FindSimilarChunks called with configuration:\n" +
		"  Project ID: %s\n" +
		"  Location: %s\n" +
		"  Index Path: %s\n" +
		"  Index Endpoint Path: %s\n" +
		"  Deployed Index ID: %s",
		s.projectID, s.location, s.indexPath, s.indexEndpointPath, s.deployedIndexID)
	
	// Use the specific API endpoint for Vector Search
	// Format: {numeric-id}.{region}-{project-id}.vdb.vertexai.goog
	apiEndpoint := "1002937326.us-central1-455488113475.vdb.vertexai.goog"
	log.Printf("Using Vector Search API endpoint: %s", apiEndpoint)
	
	// Create the request URL using the specific API endpoint
	url := fmt.Sprintf("https://%s/v1/%s:findNeighbors", apiEndpoint, s.indexEndpointPath)
	log.Printf("Sending request to URL: %s", url)

	// Create the request body
	reqBody := map[string]interface{}{
		"deployed_index_id": s.deployedIndexID,
		"queries": []map[string]interface{}{
			{
				"datapoint": map[string]interface{}{
					"feature_vector": queryEmbedding,
				},
				"neighbor_count": limit,
			},
		},
	}

	// Log the request body (without the full embedding vector to avoid log spam)
	reqBodyForLog := map[string]interface{}{
		"deployed_index_id": reqBody["deployed_index_id"],
		"queries": []map[string]interface{}{
			{
				"datapoint": map[string]interface{}{
					"feature_vector": []float32{queryEmbedding[0], queryEmbedding[1], queryEmbedding[2]},
					"feature_vector_length": len(queryEmbedding),
				},
				"neighbor_count": limit,
			},
		},
	}
	log.Printf("Request body: %+v", reqBodyForLog)

	// Marshal the request body to JSON
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %v", err)
	}

	// Create the HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Log the request headers
	headers := make(map[string]string)
	for k, v := range req.Header {
		headers[k] = strings.Join(v, ", ")
	}
	log.Printf("Request headers: %+v", headers)

	// Log the request start time
	startTime := time.Now()
	log.Printf("Sending request to Vertex AI Vector Search API...")

	// Execute the request
	response, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute HTTP request: %v", err)
	}
	defer response.Body.Close()

	// Log the response status and headers
	log.Printf("Response status: %s (%d)", response.Status, response.StatusCode)
	log.Printf("Response headers: %+v", response.Header)

	// Read the response body
	respBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// Log the response time and size
	duration := time.Since(startTime)
	log.Printf("Request completed in %v, response size: %d bytes", duration, len(respBody))

	// Log the response body (truncated if too large)
	maxBodyLogSize := 1000
	respBodyStr := string(respBody)
	if len(respBodyStr) > maxBodyLogSize {
		log.Printf("Response body (truncated): %s...", respBodyStr[:maxBodyLogSize])
	} else {
		log.Printf("Response body: %s", respBodyStr)
	}

	// Check for non-200 status code
	if response.StatusCode != http.StatusOK {
		// Try to parse the error response for more details
		var errorResp struct {
			Error struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
				Status  string `json:"status"`
				Details []map[string]interface{} `json:"details"`
			} `json:"error"`
		}
		
		if err := json.Unmarshal(respBody, &errorResp); err == nil && errorResp.Error.Code != 0 {
			// Log detailed error information
			log.Printf("Vector Search API error details:\n" +
				"  Code: %d\n" +
				"  Status: %s\n" +
				"  Message: %s",
				errorResp.Error.Code, errorResp.Error.Status, errorResp.Error.Message)
			
			// Check specifically for UNIMPLEMENTED status
			if errorResp.Error.Status == "UNIMPLEMENTED" {
				return nil, fmt.Errorf("Vector Search API returned UNIMPLEMENTED (501) error. This typically means the deployed index is not properly configured or the endpoint is not ready. Check that the index endpoint '%s' and deployed index ID '%s' are correct and the deployment is complete", s.indexEndpointPath, s.deployedIndexID)
			}
			
			return nil, fmt.Errorf("Vector Search API error: %s (code: %d, status: %s)", 
				errorResp.Error.Message, errorResp.Error.Code, errorResp.Error.Status)
		}
		
		// Fallback to basic error if we couldn't parse the detailed error
		return nil, fmt.Errorf("API request failed with status code %d: %s", response.StatusCode, string(respBody))
	}

	// Parse the response
	var resp struct {
		NearestNeighbors []struct {
			Neighbors []struct {
				Datapoint struct {
					DatapointId string `json:"datapointId"`
				} `json:"datapoint"`
				Distance float64 `json:"distance"`
			} `json:"neighbors"`
		} `json:"nearestNeighbors"`
	}

	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	// Extract chunk IDs
	var chunkIDs []string

	if len(resp.NearestNeighbors) > 0 && len(resp.NearestNeighbors[0].Neighbors) > 0 {
		log.Printf("Found %d neighbors in response", len(resp.NearestNeighbors[0].Neighbors))
		for i, neighbor := range resp.NearestNeighbors[0].Neighbors {
			chunkID := neighbor.Datapoint.DatapointId
			log.Printf("Neighbor %d: ID=%s, Distance=%.4f", i+1, chunkID, neighbor.Distance)
			chunkIDs = append(chunkIDs, chunkID)
		}
	} else {
		log.Println("No neighbors found in response")
	}

	// Retrieve chunks from Firestore
	var chunks []string

	for _, chunkID := range chunkIDs {
		doc, err := s.firestoreClient.Collection("document_chunks").Doc(chunkID).Get(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get chunk %s: %v", chunkID, err)
		}

		content, ok := doc.Data()["content"].(string)
		if !ok {
			return nil, fmt.Errorf("invalid content format for chunk %s", chunkID)
		}

		chunks = append(chunks, content)
	}

	return chunks, nil
}

// DeleteDocumentEmbeddings deletes embeddings for a document
func (s *VectorStore) DeleteDocumentEmbeddings(ctx context.Context, documentID string) error {
	// 1. Get all chunk IDs for this document
	query := s.firestoreClient.Collection("document_chunks").Where("document_id", "==", documentID)
	iter := query.Documents(ctx)
	defer iter.Stop()

	// Extract chunk IDs and collect document references
	var chunkIDs []string
	var docRefs []*firestore.DocumentRef

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return fmt.Errorf("error iterating document chunks: %v", err)
		}

		chunkIDs = append(chunkIDs, doc.Ref.ID)
		docRefs = append(docRefs, doc.Ref)
	}

	// 2. Delete from Vertex AI Vector Search
	if len(chunkIDs) > 0 {
		// Create the request URL for RemoveDatapoints
		url := fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/%s:removeDatapoints",
			s.location, s.indexPath)

		// Create the request body
		reqBody := map[string]interface{}{
			"datapoint_ids": chunkIDs,
		}

		// Marshal the request body to JSON
		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %v", err)
		}

		// Create the HTTP request
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
		if err != nil {
			return fmt.Errorf("failed to create HTTP request: %v", err)
		}

		// Set headers
		req.Header.Set("Content-Type", "application/json")

		// Execute the request
		response, err := s.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to execute HTTP request: %v", err)
		}
		defer response.Body.Close()

		// Read the response body
		respBody, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body: %v", err)
		}

		// Check for non-200 status code
		if response.StatusCode != http.StatusOK {
			return fmt.Errorf("API request failed with status code %d: %s", response.StatusCode, string(respBody))
		}
	}

	// 3. Delete from Firestore
	if len(docRefs) > 0 {
		batch := s.firestoreClient.Batch()
		for _, ref := range docRefs {
			batch.Delete(ref)
		}

		_, err := batch.Commit(ctx)
		if err != nil {
			return fmt.Errorf("failed to delete chunks from Firestore: %v", err)
		}
	}

	return nil
}

// DeleteDocumentEmbeddingsByPrefix deletes all document embeddings with IDs starting with the given prefix
func (s *VectorStore) DeleteDocumentEmbeddingsByPrefix(ctx context.Context, prefix string) error {
	// 1. Get all chunk IDs for documents with IDs starting with the prefix
	// Use a range query: >= prefix and < prefix + \uf8ff (which is higher than any UTF-8 character)
	query := s.firestoreClient.Collection("document_chunks").Where("document_id", ">", prefix).Where("document_id", "<", prefix+"\uf8ff")
	iter := query.Documents(ctx)
	defer iter.Stop()

	// Extract chunk IDs and collect document references
	var chunkIDs []string
	var docRefs []*firestore.DocumentRef
	var documentIDs []string

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return fmt.Errorf("error iterating document chunks: %v", err)
		}

		chunkIDs = append(chunkIDs, doc.Ref.ID)
		docRefs = append(docRefs, doc.Ref)

		// Extract document ID for logging
		var data map[string]interface{}
		if err := doc.DataTo(&data); err == nil {
			if docID, ok := data["document_id"].(string); ok {
				documentIDs = append(documentIDs, docID)
			}
		}
	}

	// Log the number of documents found
	log.Printf("Found %d chunks with document IDs starting with prefix '%s'", len(chunkIDs), prefix)
	if len(documentIDs) > 0 {
		log.Printf("Document IDs: %v", documentIDs)
	}

	// 2. Delete from Vertex AI Vector Search
	if len(chunkIDs) > 0 {
		// Create the request URL for RemoveDatapoints
		url := fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/%s:removeDatapoints",
			s.location, s.indexPath)

		// Create the request body
		reqBody := map[string]interface{}{
			"datapoint_ids": chunkIDs,
		}

		// Marshal the request body to JSON
		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %v", err)
		}

		// Create the HTTP request
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
		if err != nil {
			return fmt.Errorf("failed to create HTTP request: %v", err)
		}

		// Set headers
		req.Header.Set("Content-Type", "application/json")

		// Execute the request
		response, err := s.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to execute HTTP request: %v", err)
		}
		defer response.Body.Close()

		// Read the response body
		respBody, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body: %v", err)
		}

		// Check for non-200 status code
		if response.StatusCode != http.StatusOK {
			return fmt.Errorf("API request failed with status code %d: %s", response.StatusCode, string(respBody))
		}
	}

	// 3. Delete from Firestore
	if len(docRefs) > 0 {
		// Use batched writes for better performance
		// Firestore has a limit of 500 operations per batch
		const batchSize = 500
		for i := 0; i < len(docRefs); i += batchSize {
			end := i + batchSize
			if end > len(docRefs) {
				end = len(docRefs)
			}

			batch := s.firestoreClient.Batch()
			for _, ref := range docRefs[i:end] {
				batch.Delete(ref)
			}

			_, err := batch.Commit(ctx)
			if err != nil {
				return fmt.Errorf("failed to delete chunks from Firestore (batch %d-%d): %v", i, end, err)
			}
		}
	}

	return nil
}
