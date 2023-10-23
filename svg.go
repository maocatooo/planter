package main

import (
	"bytes"
	"io"
	"log"
	"net/http"
)

func genSVG(plantUMLCode string) []byte {
	// PlantUML code to be converted to SVG

	// URL of the PlantUML server
	plantUMLServerURL := "https://kroki.io/plantuml/svg"

	// Send the PlantUML code to the PlantUML server
	resp, err := http.Post(plantUMLServerURL, "text/plain", bytes.NewBufferString(plantUMLCode))
	if err != nil {
		log.Fatal("Error sending request:", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatal("PlantUML server returned an error:", resp.Status)
	}
	bs, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("Error reading response:", err)
	}
	return bs
}
