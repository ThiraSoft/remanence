package about

import (
	"encoding/json"
	"net/http"

	"remanence/internal/config"
)

// AboutResponse represents the response structure for the /about endpoint
type AboutResponse struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	BuildDate string `json:"buildDate"`
}

// handleAboutEndpoint implements the /about endpoint
func HandleAboutEndpoint(w http.ResponseWriter, r *http.Request) {
	response := AboutResponse{
		Name:      "remanence",
		Version:   config.AppVersion,
		BuildDate: config.BuildDate,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
