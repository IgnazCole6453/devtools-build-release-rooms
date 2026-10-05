package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
)

type server struct {
	client    *roomClient
	mu        sync.Mutex
	completed map[string]json.RawMessage
}

func (s *server) match(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()
	var input matchRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	p, err := place(input)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if previous, ok := s.completed[input.RequestID]; ok {
		w.Header().Set("Content-Type", "application/json")
		w.Write(previous)
		return
	}
	data := map[string]string{"build_id": input.BuildID, "release_id": input.ReleaseID, "developer_id": input.DeveloperID, "kind": input.Kind}
	if input.Kind == "diagnostic" {
		data["diagnostic"] = input.Diagnostic
	}
	if err := s.client.create(r.Context(), p.Channel, "channel:"+p.Channel); err != nil {
		respondError(w, err)
		return
	}
	if err := s.client.publish(r.Context(), p.Channel, p.Event, input.DeveloperID, "event:"+input.RequestID, data); err != nil {
		respondError(w, err)
		return
	}
	token, err := s.client.token(r.Context(), p.Channel, input.DeveloperID, "token:"+input.RequestID)
	if err != nil {
		respondError(w, err)
		return
	}
	output, _ := json.Marshal(struct {
		Channel     string          `json:"channel"`
		Event       string          `json:"event"`
		Credentials json.RawMessage `json:"credentials"`
	}{p.Channel, p.Event, token})
	s.completed[input.RequestID] = output
	w.Header().Set("Content-Type", "application/json")
	w.Write(output)
}

func (s *server) state(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", http.StatusMethodNotAllowed)
		return
	}
	channel := r.URL.Query().Get("channel")
	if !strings.HasPrefix(channel, "devtools-") || len(channel) != len("devtools-")+24 || strings.Trim(channel[len("devtools-"):], "0123456789abcdef") != "" {
		http.Error(w, "invalid channel", http.StatusBadRequest)
		return
	}
	data, err := s.client.presence(r.Context(), channel)
	if err != nil {
		respondError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func respondError(w http.ResponseWriter, err error) {
	var rejected *rejection
	if errors.As(err, &rejected) {
		status := rejected.Status
		if status < 400 || status >= 500 {
			status = http.StatusBadGateway
		}
		http.Error(w, rejected.Error(), status)
		return
	}
	http.Error(w, "upstream request failed", http.StatusBadGateway)
}

func main() {
	client, err := newRoomClient()
	if err != nil {
		log.Fatal(err)
	}
	s := &server{client: client, completed: make(map[string]json.RawMessage)}
	http.HandleFunc("/match", s.match)
	http.HandleFunc("/state", s.state)
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Fatal(http.ListenAndServe(addr, nil))
}
