package main

import (
	"encoding/json"
	"log"
	"net/http"
)

func main() {
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","server":"ollaya"}`))
	})

	// Model pull endpoint
	mux.HandleFunc("/api/pull", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","digest":"sha256:laya-warm-digest"}`))
	})

	// SystemOne wire endpoints
	handleDecide := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"model": "laya",
			"answers": map[string]any{
				"severity": map[string]any{
					"value": "High",
					"score": 2.1,
				},
				"safety_risk": map[string]any{
					"value":       "HIGH",
					"probability": 0.94,
				},
				"field_visit_required": map[string]any{
					"value":       true,
					"probability": 0.91,
				},
			},
			"confidence": 0.94,
			"timing_ms":  18.5,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}

	mux.HandleFunc("/v1/systemone", handleDecide)
	mux.HandleFunc("/v1/decisions", handleDecide)
	mux.HandleFunc("/api/decide", handleDecide)

	log.Println("Starting Ollaya server on :11435")
	if err := http.ListenAndServe(":11435", mux); err != nil {
		log.Fatalf("Ollaya server failed: %v", err)
	}
}
