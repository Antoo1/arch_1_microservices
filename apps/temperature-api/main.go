// Command temperature-api is a stateless simulator of a remote temperature sensor.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"time"
)

const (
	defaultAddr = ":8081"
	unit        = "°C"
	sensorType  = "temperature"
	statusOK    = "active"

	minTemp = -10.0
	maxTemp = 40.0
)

// Reading must stay in sync with smart_home's services.TemperatureResponse.
type Reading struct {
	Value       float64   `json:"value"`
	Unit        string    `json:"unit"`
	Timestamp   time.Time `json:"timestamp"`
	Location    string    `json:"location"`
	Status      string    `json:"status"`
	SensorID    string    `json:"sensor_id"`
	SensorType  string    `json:"sensor_type"`
	Description string    `json:"description"`
}

func resolveIdentity(location, sensorID string) (string, string) {
	if location == "" {
		switch sensorID {
		case "1":
			location = "Living Room"
		case "2":
			location = "Bedroom"
		case "3":
			location = "Kitchen"
		default:
			location = "Unknown"
		}
	}

	if sensorID == "" {
		switch location {
		case "Living Room":
			sensorID = "1"
		case "Bedroom":
			sensorID = "2"
		case "Kitchen":
			sensorID = "3"
		default:
			sensorID = "0"
		}
	}

	return location, sensorID
}

func newReading(location, sensorID string) Reading {
	location, sensorID = resolveIdentity(location, sensorID)

	value := minTemp + rand.Float64()*(maxTemp-minTemp)
	value = math.Round(value*10) / 10

	return Reading{
		Value:       value,
		Unit:        unit,
		Timestamp:   time.Now().UTC(),
		Location:    location,
		Status:      statusOK,
		SensorID:    sensorID,
		SensorType:  sensorType,
		Description: "Simulated temperature reading",
	}
}

func writeJSON(w http.ResponseWriter, reading Reading) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(reading); err != nil {
		log.Printf("failed to encode response: %v", err)
	}
}

func handleByLocation(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, newReading(r.URL.Query().Get("location"), ""))
}

func handleByID(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, newReading("", r.PathValue("id")))
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintln(w, `{"status":"ok"}`)
}

func main() {
	addr := os.Getenv("PORT")
	if addr == "" {
		addr = defaultAddr
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /temperature", handleByLocation)
	mux.HandleFunc("GET /temperature/{id}", handleByID)
	mux.HandleFunc("GET /health", handleHealth)

	log.Printf("temperature-api listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
