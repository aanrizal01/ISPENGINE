package main

import (
	"database/sql"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
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
	kmlFile := "mymaps.kml"
	f, err := os.Open(kmlFile)
	if err != nil {
		log.Fatalf("Gagal membuka file KML: %v", err)
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		log.Fatalf("Gagal membaca file KML: %v", err)
	}

	var kml KML
	if err := xml.Unmarshal(data, &kml); err != nil {
		log.Fatalf("Gagal parse KML: %v", err)
	}

	db, err := sql.Open("sqlite", "onboarding.db")
	if err != nil {
		log.Fatalf("Gagal membuka database onboarding.db: %v", err)
	}
	defer db.Close()

	// Clear old demo dummy ODPs
	_, _ = db.Exec(`DELETE FROM odp_nodes WHERE code LIKE 'ODP-CKR-%'`)

	tx, err := db.Begin()
	if err != nil {
		log.Fatalf("Gagal memulai transaksi: %v", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO odp_nodes (id, code, name, latitude, longitude, total_ports, used_ports, status, cluster_area, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		log.Fatalf("Gagal prepare statement: %v", err)
	}
	defer stmt.Close()

	totalInserted := 0
	counters := make(map[string]int)

	for _, folder := range kml.Document.Folders {
		folderName := strings.TrimSpace(folder.Name)
		if strings.Contains(folderName, "Wireless") {
			continue // Abaikan wireless
		}

		switch {
		case strings.Contains(folderName, "Biaro"):
			prefix = "ODP-BIO"
		case strings.Contains(folderName, "Payakumbuh"):
			prefix = "ODP-PYK"
		case strings.Contains(folderName, "Geringging"):
			prefix = "ODP-SGG"
		default:
			prefix = "ODP-NET"
		}

		for _, pm := range folder.Placemarks {
			if pm.Point == nil {
				continue
			}

			rawCoords := strings.TrimSpace(pm.Point.Coordinates)
			parts := strings.Split(rawCoords, ",")
			if len(parts) < 2 {
				continue
			}

			lng, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
			lat, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
			if err1 != nil || err2 != nil {
				continue
			}

			counters[prefix]++
			idx := counters[prefix]
			odpCode := fmt.Sprintf("%s-%04d", prefix, idx)

			odpName := strings.TrimSpace(pm.Name)
			if odpName == "" || odpName == "Untitled Placemark" {
				odpName = fmt.Sprintf("Tiang %s #%d", folderName, idx)
			}

			id := uuid.NewString()
			totalPorts := 8
			if strings.Contains(folderName, "Wireless") {
				totalPorts = 16
			}
			usedPorts := 0
			status := "AVAILABLE"
			now := time.Now()

			_, err = stmt.Exec(id, odpCode, odpName, lat, lng, totalPorts, usedPorts, status, folderName, now, now)
			if err != nil {
				log.Printf("Gagal insert %s: %v", odpCode, err)
				continue
			}

			totalInserted++
		}
	}

	if err := tx.Commit(); err != nil {
		log.Fatalf("Gagal commit transaksi: %v", err)
	}

	fmt.Printf("==================================================\n")
	fmt.Printf("SUKSES IMPORT TITIK DARI GOOGLE MY MAPS!\n")
	fmt.Printf("==================================================\n")
	fmt.Printf("Peta Coverage: %s\n", kml.Document.Name)
	fmt.Printf("Total Titik ODP / BTS Berhasil Dimasukkan: %d titik\n", totalInserted)
	for pfx, cnt := range counters {
		fmt.Printf(" - %s: %d titik\n", pfx, cnt)
	}
}
