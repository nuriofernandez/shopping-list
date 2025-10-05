package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

// The path where the JSON data will be stored persistently.
const dataFilePath = "data.json"

// JSONData is a type alias for a generic JSON object structure.
type JSONData map[string]interface{}

// Store holds the application state, including the file path and a mutex
// for concurrent access control to the file.
type Store struct {
	filepath string
	// RWMutex allows many readers or one writer at a time.
	mu sync.RWMutex
}

// NewStore initializes a new Store and ensures the data file exists.
func NewStore(path string) *Store {
	s := &Store{filepath: path}
	// Attempt to create the file if it doesn't exist, initializing it with an empty JSON object.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		log.Printf("Data file %s not found, creating a new empty one.", path)
		if err := s.saveDataFile(JSONData{}); err != nil {
			log.Fatalf("Failed to initialize data file: %v", err)
		}
	}
	return s
}

// readDataFile reads the JSON data from the file, locking the store for reading.
func (s *Store) readDataFile() (JSONData, error) {
	s.mu.RLock()         // Acquire read lock
	defer s.mu.RUnlock() // Release read lock when function returns

	content, err := os.ReadFile(s.filepath)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	// Handle empty file case
	if len(content) == 0 {
		return JSONData{}, nil
	}

	var data JSONData
	if err := json.Unmarshal(content, &data); err != nil {
		return nil, fmt.Errorf("error unmarshaling JSON: %w", err)
	}
	return data, nil
}

// saveDataFile writes the JSON data to the file, locking the store for writing.
// This function overwrites the entire file content.
func (s *Store) saveDataFile(data JSONData) error {
	s.mu.Lock()         // Acquire write lock
	defer s.mu.Unlock() // Release write lock when function returns

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}

	// Write the data to the file, overwriting existing content.
	if err := os.WriteFile(s.filepath, jsonData, 0644); err != nil {
		return fmt.Errorf("error writing to file: %w", err)
	}

	log.Printf("Successfully saved data to %s", s.filepath)
	return nil
}

// getDataHandler handles GET /data requests to fetch the JSON content.
func getDataHandler(s *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		data, err := s.readDataFile()
		if err != nil {
			log.Printf("Error in GET /data: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(data); err != nil {
			log.Printf("Error encoding response: %v", err)
			// Note: Cannot send another header/status after writing the body
		}
	}
}

// updateDataHandler handles POST and PUT requests to completely overwrite the JSON file.
func updateDataHandler(s *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost && r.Method != http.MethodPut {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Could not read request body", http.StatusBadRequest)
			return
		}

		var newData JSONData
		if err := json.Unmarshal(body, &newData); err != nil {
			http.Error(w, "Invalid JSON format in request body", http.StatusBadRequest)
			return
		}

		// Save the new data, overwriting the old content.
		if err := s.saveDataFile(newData); err != nil {
			log.Printf("Error in %s /data: %v", r.Method, err)
			http.Error(w, "Internal Server Error: Failed to save data", http.StatusInternalServerError)
			return
		}

		// Success response
		status := http.StatusOK
		if r.Method == http.MethodPost {
			status = http.StatusCreated // Use 201 for POST (new resource state created)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		fmt.Fprintf(w, `{"message": "Data successfully stored/updated", "status": %d}`, status)
	}
}

func main() {
	// 1. Initialize the Store
	store := NewStore(dataFilePath)

	router := mux.NewRouter()

	router.HandleFunc("/data", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			getDataHandler(store)(w, r)
		case http.MethodPost, http.MethodPut:
			updateDataHandler(store)(w, r)
		default:
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	})

	router.PathPrefix("/").Handler(http.FileServer(http.Dir("website")))

	headers := handlers.AllowedHeaders([]string{"X-Requested-With", "Content-Type", "Authorization"})
	methods := handlers.AllowedMethods([]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"})
	origins := handlers.AllowedOrigins([]string{"*"})

	// 3. Start the server
	port := "8080"
	log.Printf("Starting API server on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, handlers.CORS(headers, methods, origins)(router)))
}
