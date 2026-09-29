package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

func main() {
	db, err := sql.Open("sqlite", "onboarding.db")
	if err != nil {
		log.Fatalf("Gagal buka DB: %v", err)
	}
	defer db.Close()

	// 1. Nonaktifkan semua selain Golden Payakumbuh
	res1, err := db.Exec(`UPDATE odp_nodes SET status = 'INACTIVE' WHERE cluster_area != 'Golden Payakumbuh'`)
	if err != nil {
		log.Fatalf("Gagal nonaktifkan: %v", err)
	}
	inactCount, _ := res1.RowsAffected()

	// 2. Aktifkan hanya Golden Payakumbuh
	res2, err := db.Exec(`UPDATE odp_nodes SET status = 'AVAILABLE' WHERE cluster_area = 'Golden Payakumbuh'`)
	if err != nil {
		log.Fatalf("Gagal aktifkan: %v", err)
	}
	actCount, _ := res2.RowsAffected()

	// 3. Ambil koordinat rata-rata Payakumbuh untuk titik tengah peta
	var avgLat, avgLng float64
	_ = db.QueryRow(`SELECT AVG(latitude), AVG(longitude) FROM odp_nodes WHERE status = 'AVAILABLE'`).Scan(&avgLat, &avgLng)

	fmt.Printf("Status Update:\n")
	fmt.Printf(" - Titik Dinonaktifkan: %d (Biaro, Bukittinggi, Geringging, Wireless)\n", inactCount)
	fmt.Printf(" - Titik Aktif Payakumbuh: %d ODP\n", actCount)
	fmt.Printf(" - Titik Tengah (Center Map) Payakumbuh: Lat: %f, Lng: %f\n", avgLat, avgLng)
}
