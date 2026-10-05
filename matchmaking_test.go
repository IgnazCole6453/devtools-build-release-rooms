package main

import "testing"

func TestPlacement(t *testing.T) {
	base := matchRequest{RequestID: "req-1", BuildID: "build-42", ReleaseID: "rc-7", DeveloperID: "dev-3", Kind: "build"}
	first, err := place(base)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name     string
		change   func(*matchRequest)
		sameRoom bool
		event    string
	}{
		{"another developer joins the same release", func(m *matchRequest) { m.DeveloperID = "dev-4" }, true, "build"},
		{"release operation stays with its build", func(m *matchRequest) { m.Kind = "release" }, true, "release"},
		{"diagnostic stays with its release", func(m *matchRequest) { m.Kind = "diagnostic"; m.Diagnostic = "worker lag" }, true, "diagnostic"},
		{"different release separates rooms", func(m *matchRequest) { m.ReleaseID = "rc-8" }, false, "build"},
		{"different build separates rooms", func(m *matchRequest) { m.BuildID = "build-43" }, false, "build"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := base
			tt.change(&m)
			got, err := place(m)
			if err != nil {
				t.Fatal(err)
			}
			if (got.Channel == first.Channel) != tt.sameRoom || got.Event != tt.event {
				t.Fatalf("placement = %+v; original = %+v", got, first)
			}
		})
	}
}
