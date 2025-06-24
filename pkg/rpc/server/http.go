package server

import (
	"fmt"
	"net/http"

	"github.com/rollkit/rollkit/types"
)

// RegisterCustomHTTPEndpoints is the designated place to add new, non-gRPC, plain HTTP handlers.
// Additional custom HTTP endpoints can be registered on the mux here.
func RegisterCustomHTTPEndpoints(mux *http.ServeMux) {
	mux.HandleFunc("/health/live", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "OK")
	})

	// Add REST-style endpoints for metadata
	mux.HandleFunc("/api/v1/metadata/keys", handleListMetadataKeys)
	mux.HandleFunc("/api/v1/metadata", handleGetAllMetadata)

	// Example for adding more custom endpoints:
	// mux.HandleFunc("/custom/myendpoint", func(w http.ResponseWriter, r *http.Request) {
	//     // Your handler logic here
	//     w.WriteHeader(http.StatusOK)
	//     fmt.Fprintln(w, "My custom endpoint!")
	// })
}

// handleListMetadataKeys provides a REST endpoint for listing metadata keys
func handleListMetadataKeys(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	
	// Get the known metadata keys
	knownKeys := types.GetKnownMetadataKeys()
	
	// Build JSON response
	response := `{"keys":[`
	first := true
	for key, description := range knownKeys {
		if !first {
			response += ","
		}
		response += fmt.Sprintf(`{"key":"%s","description":"%s"}`, key, description)
		first = false
	}
	response += `]}`

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, response)
}

// handleGetAllMetadata provides a REST endpoint for getting all metadata
// Note: This is a simplified implementation for demonstration.
// In a production environment, you'd want to pass a store instance to access metadata.
func handleGetAllMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	
	// This endpoint would need access to the store to fetch actual metadata values.
	// For now, we return the available keys with a note about using the RPC interface.
	response := `{"message":"Use the RPC interface or gRPC-Web to fetch actual metadata values","available_keys":[`
	
	knownKeys := types.GetKnownMetadataKeysList()
	for i, key := range knownKeys {
		if i > 0 {
			response += ","
		}
		response += fmt.Sprintf(`"%s"`, key)
	}
	
	response += `],"rpc_method":"rollkit.v1.StoreService/GetAllMetadata"}`

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, response)
}
