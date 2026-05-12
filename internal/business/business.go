package business

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"remanence/internal/logger"
	"remanence/internal/messages"
	"strconv"
)

type MessageResponse struct {
	ID      string `json:"id"`
	Content string `json:"content,omitempty"`
}

func GetMessageByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	id := r.PathValue("id")
	logger.Log.Debug("Fetching message", slog.String("id", id))

	content, err := messages.GetMessageContentByID(id)
	if err != nil {
		logger.Log.Error("Error getting message", slog.String("id", id), slog.String("error", err.Error()))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(MessageResponse{
		ID:      id,
		Content: content,
	})
}

func CreateMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	// Limit request body to 64KB to prevent abuse
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)

	content := r.FormValue("content")
	if content == "" {
		http.Error(w, "Content cannot be empty", http.StatusBadRequest)
		return
	}

	var lifeLimit int
	if lifeLimitStr := r.FormValue("lifeLimit"); lifeLimitStr != "" {
		var err error
		lifeLimit, err = strconv.Atoi(lifeLimitStr)
		if err != nil {
			http.Error(w, "invalid lifeLimit value", http.StatusBadRequest)
			return
		}
	}
	isOneShot := r.FormValue("isOneShot") != "false"

	id, err := messages.NewMessageWithRandomID(content, isOneShot, lifeLimit)
	if err != nil {
		logger.Log.Error("Error creating message", slog.String("error", err.Error()))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Only return the ID — never echo the content back over the wire
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(MessageResponse{ID: id})
}
