package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
)

type matchRequest struct {
	RequestID   string `json:"request_id"`
	BuildID     string `json:"build_id"`
	ReleaseID   string `json:"release_id"`
	DeveloperID string `json:"developer_id"`
	Kind        string `json:"kind"`
	Diagnostic  string `json:"diagnostic"`
}

type placement struct {
	Channel string `json:"channel"`
	Event   string `json:"event"`
}

func place(m matchRequest) (placement, error) {
	if m.RequestID == "" || m.BuildID == "" || m.ReleaseID == "" || m.DeveloperID == "" {
		return placement{}, errors.New("request_id, build_id, release_id and developer_id are required")
	}
	if m.Kind != "build" && m.Kind != "release" && m.Kind != "diagnostic" {
		return placement{}, errors.New("kind must be build, release or diagnostic")
	}
	if m.Kind == "diagnostic" && m.Diagnostic == "" {
		return placement{}, errors.New("diagnostic is required for diagnostic events")
	}
	pair, _ := json.Marshal([2]string{m.BuildID, m.ReleaseID})
	sum := sha256.Sum256(pair)
	return placement{Channel: "devtools-" + hex.EncodeToString(sum[:12]), Event: m.Kind}, nil
}
