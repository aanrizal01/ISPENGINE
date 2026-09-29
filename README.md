# ISP Onboarding & Partner Registration Gateway

Layanan mandiri (*standalone microservice*) pendaftaran pelanggan baru ISP dan gerbang kemitraan (*multi-partner gateway*) yang terintegrasi langsung dengan **GOGIGABILL Core Billing** melalui REST API.

## Fitur Utama

1. **Smart Coverage Engine (Haversine/ODP Distance):**
   - Menghitung jarak rumah pelanggan ke titik ODP (*Optical Distribution Point*) terdekat.
   - Membatasi penarikan kabel *dropcore* (default: maks 250 meter).
   - Validasi sisa kapasitas port ODP secara *real-time*.
2. **Multi-Partner & Reseller Portal Ready:**
   - Mendukung otentikasi mitra via header `X-Partner-Key`.
   - Setiap pendaftaran otomatis mengaitkan `partner_id` untuk perhitungan komisi di GOGIGABILL.
3. **Pipeline Work Order & BAST Digital:**
   - Surat Perintah Kerja (SPK) untuk teknisi survei dan instalasi.
   - Validasi batas redaman optik (OPM $\ge -27\text{ dBm}$) sebelum BAST disetujui.
4. **Auto-Promote ke GOGIGABILL Core:**
   - Saat BAST selesai, gateway langsung memanggil API GOGIGABILL:
     - `POST /api/v1/customers` (Membuat akun pelanggan)
     - `POST /api/v1/subscriptions` (Membuat langganan dan akun PPPoE ke FreeRADIUS/MikroTik)

## Daftar API Endpoints

### 1. Publik / Calon Pelanggan
- `POST /api/v1/public/coverage-check`: Pengecekan koordinat lat/long terhadap ODP.
- `POST /api/v1/public/register`: Formulir pendaftaran calon pelanggan.
- `GET  /api/v1/public/plans`: Mengambil paket internet (dari GOGIGABILL / fallback).
- `GET  /api/v1/public/track/{regNo}`: Pelacakan progres pemasangan berdasarkan nomor registrasi.
- `GET  /api/v1/public/odps`: Daftar titik persebaran ODP.

### 2. Mitra / Agen Sales (`X-Partner-Key: <key>`)
- `POST /api/v1/partner/register`: Submit pelanggan atas nama mitra.
- `GET  /api/v1/partner/registrations`: Melihat daftar pengajuan pelanggan dari mitra tersebut.

### 3. Teknisi Lapangan (SPK & BAST)
- `GET  /api/v1/technician/work-orders`: Daftar SPK survei / pasang baru.
- `GET  /api/v1/technician/work-orders/{id}`: Detail SPK.
- `POST /api/v1/technician/work-orders/{id}/bast`: Submit BAST (Redaman dBm, SN ONT, Mac Address).

### 4. Admin Backoffice (`X-Admin-Key: <key>`)
- `GET  /api/v1/admin/registrations`: Manajemen pipeline pendaftaran.
- `POST /api/v1/admin/work-orders`: Penugasan teknisi.
- `GET / POST /api/v1/admin/odps`: Manajemen titik ODP dan kapasitas port.
- `GET / POST /api/v1/admin/partners`: Manajemen mitra dan pembuatan API Key.

## Menjalankan Aplikasi

```powershell
# Copy environment
cp .env.example .env

# Jalankan server
go run cmd/api/main.go
```
Port default: `http://localhost:8081`
