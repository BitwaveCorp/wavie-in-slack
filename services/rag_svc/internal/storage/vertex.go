package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"cloud.google.com/go/aiplatform/apiv1/aiplatformpb"
	"cloud.google.com/go/firestore"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/iterator"
)

// VectorStore handles vector storage and retrieval
type VectorStore struct {
	projectID       string
	location        string
	firestoreClient *firestore.Client
	httpClient      *http.Client
	indexPath        string
	indexEndpointPath string
	deployedIndexID  string
}

// NewVectorStore creates a new vector store
func NewVectorStore(ctx context.Context, projectID, location, indexID, indexEndpointID, deployedIndexID string) (*VectorStore, error) {
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
		projectID:        projectID,
		location:         location,
		firestoreClient:  firestoreClient,
		httpClient:       httpClient,
		indexPath:        indexPath,
		indexEndpointPath: indexEndpointPath,
		deployedIndexID:  deployedIndexID,
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
	// Create the request URL
	url := fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/%s:findNeighbors", 
		s.location, s.indexEndpointPath)
	
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
	
	// Execute the request
	response, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute HTTP request: %v", err)
	}
	defer response.Body.Close()
	
	// Read the response body
	respBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}
	
	// Check for non-200 status code
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status code %d: %s", response.StatusCode, string(respBody))
	}
	
	// Parse the response
	var resp struct {
		NearestNeighbors []struct {
			Neighbors []struct {
				DatapointId string  `json:"datapoint_id"`
				Distance    float64 `json:"distance"`
			} `json:"neighbors"`
		} `json:"nearest_neighbors"`
	}
	
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}
	
	// Extract chunk IDs
	var chunkIDs []string
	
	if len(resp.NearestNeighbors) > 0 && len(resp.NearestNeighbors[0].Neighbors) > 0 {
		for _, neighbor := range resp.NearestNeighbors[0].Neighbors {
			chunkIDs = append(chunkIDs, neighbor.DatapointId)
		}
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
