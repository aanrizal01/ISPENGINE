package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strings"
)

type KML struct {
	Document Document `xml:"Document"`
}

type Document struct {
	Name    string   `xml:"name"`
	Folders []Folder `xml:"Folder"`
}

type Folder struct {
	Name       string      `xml:"name"`
	Placemarks []Placemark `xml:"Placemark"`
}

type Placemark struct {
	Name        string `xml:"name"`
	Description string `xml:"description"`
	Point       *Point `xml:"Point"`
}

type Point struct {
	Coordinates string `xml:"coordinates"`
}

func main() {
	f, err := os.Open("mymaps.kml")
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		return
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		return
	}

	var kml KML
	if err := xml.Unmarshal(data, &kml); err != nil {
		fmt.Printf("XML error: %v\n", err)
		return
	}

	fmt.Printf("Document Name: %s\n", kml.Document.Name)
	fmt.Printf("Total Folders: %d\n", len(kml.Document.Folders))

	totalPoints := 0
	for _, folder := range kml.Document.Folders {
		pointsInFolder := 0
		for _, pm := range folder.Placemarks {
			if pm.Point != nil {
				coords := strings.TrimSpace(pm.Point.Coordinates)
				parts := strings.Split(coords, ",")
				if len(parts) >= 2 {
					pointsInFolder++
				}
			}
		}
		totalPoints += pointsInFolder
		fmt.Printf(" - Folder '%s': %d Points (Placemarks: %d)\n", folder.Name, pointsInFolder, len(folder.Placemarks))
	}
	fmt.Printf("Total Points Found: %d\n", totalPoints)
}
