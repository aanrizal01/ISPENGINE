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
	f, _ := os.Open("mymaps.kml")
	defer f.Close()
	data, _ := io.ReadAll(f)
	var kml KML
	_ = xml.Unmarshal(data, &kml)

	for _, folder := range kml.Document.Folders {
		fmt.Printf("\n=== Folder: %s ===\n", folder.Name)
		count := 0
		for _, pm := range folder.Placemarks {
			if pm.Point != nil && count < 5 {
				name := strings.TrimSpace(pm.Name)
				coords := strings.TrimSpace(pm.Point.Coordinates)
				fmt.Printf(" - [%s] Coords: %s\n", name, coords)
				count++
			}
		}
	}
}
