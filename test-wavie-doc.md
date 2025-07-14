# Wavie Bot Documentation

Wavie is an AI assistant bot for Slack that helps teams with various tasks. It uses Claude AI to provide intelligent responses and can access knowledge files uploaded by users.

## Features

- Answer questions based on uploaded knowledge
- Assist with coding tasks
- Summarize conversations
- Help with writing and editing

## How to Use Wavie

1. Mention @Wavie in a channel or direct message
2. Ask your question or describe your task
3. Wavie will respond with helpful information

## Knowledge Management

Users can upload documents to Wavie's knowledge base:

1. Go to the knowledge management page
2. Upload markdown or text files
3. Wavie will process these files and use them to answer questions

## Technical Details

- Built with Go
- Uses Claude API for AI responses
- Stores embeddings in Vertex AI Vector Search
- Manages document metadata in Firestore
