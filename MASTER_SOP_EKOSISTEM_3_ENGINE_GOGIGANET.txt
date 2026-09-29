# BUKU MASTER STANDAR OPERASIONAL PROSEDUR (SOP)
## EKOSISTEM DIGITAL TERPADU TELEKOMUNIKASI (3 ENGINE)
**PT GOGIGA MEDIA TEKNOLOGI — GOGIGANET FIBER BROADBAND**

* **Nomor Dokumen**: `GMT/MASTER-SOP/2026/09/002`
* **Klasifikasi**: Dokumen Resmi Operasional, Finansial & Teknis Telekomunikasi
* **Versi / Edisi**: `2.0 (Master Integrated Edition)`
* **Tanggal Ditetapkan**: 26 September 2026
* **Otoritas Pengesahan**: **Aan Rizal S.Kom (Direktur Utama / Owner PT GOGIGA MEDIA TEKNOLOGI)**
* **Penanggung Jawab Teknis**: **Nando Azkia Putra S.Kom (Kepala NOC & Core FO)**
* **Wilayah Operasional**: Kota Payakumbuh, Kab. Lima Puluh Kota, Bukittinggi, Agam, & Sekitarnya (Sumatera Barat)

---

## DAFTAR ISI MASTER SOP

1. [Bab I: Arsitektur Ekosistem 3 Engine & Single Source of Truth](#bab-i-arsitektur-ekosistem-3-engine--single-source-of-truth)
2. [Bab II: Standar Port, Domain, & Matriks Hak Akses Kredensial](#bab-ii-standar-port-domain--matriks-hak-akses-kredensial)
3. [Bab III: Bagian I — SOP Infrastruktur Fisik & Core Optik (FTTX Engine)](#bab-iii-bagian-i--sop-infrastruktur-fisik--core-optik-fttx-engine)
   - [SOP-FTTX-01: Perencanaan & Pemasangan Tiang ODP / ODC Baru](#sop-fttx-01-perencanaan--pemasangan-tiang-odp--odc-baru)
   - [SOP-FTTX-02: Pemetaan Rute Kabel FO GIS (12/24/48 Core) & Ekspor KML](#sop-fttx-02-pemetaan-rute-kabel-fo-gis-122448-core--ekspor-kml)
   - [SOP-FTTX-03: Sinkronisasi Aset ODP ke ISP Onboarding Engine](#sop-fttx-03-sinkronisasi-aset-odp-ke-isp-onboarding-engine)
   - [SOP-FTTX-04: Standar Pengukuran Daya Optik SNMP & Ambang Batas dBm](#sop-fttx-04-standar-pengukuran-daya-optik-snmp--ambang-batas-dbm)
   - [SOP-FTTX-05: Zero-Touch Provisioning (SmartOLT) & TR-069 Remote Wi-Fi](#sop-fttx-05-zero-touch-provisioning-smartolt--tr-069-remote-wi-fi)
   - [SOP-FTTX-06: Manajemen Sewa Port Wholesale Jartaplok B2B](#sop-fttx-06-manajemen-sewa-port-wholesale-jartaplok-b2b)
4. [Bab IV: Bagian II — SOP Ritel, Sales & Operasional Lapangan (ISP Engine)](#bab-iv-bagian-ii--sop-ritel-sales--operasional-lapangan-isp-engine)
   - [SOP-ISP-01: Registrasi Mandiri Pelanggan & Cek Coverage Radius 250m](#sop-isp-01-registrasi-mandiri-pelanggan--cek-coverage-radius-250m)
   - [SOP-ISP-02: Prospek Tim Sales, Kode Referral, & Klaim Komisi Rp 50.000](#sop-isp-02-prospek-tim-sales-kode-referral--klaim-komisi-rp-50000)
   - [SOP-ISP-03: Verifikasi Dokumen KTP, Kontrak Digital, & Penerbitan SPK](#sop-isp-03-verifikasi-dokumen-ktp-kontrak-digital--penerbitan-spk)
   - [SOP-ISP-04: Standar K3 Teknisi Lapangan & Penarikan Kabel Dropcore](#sop-isp-04-standar-k3-teknisi-lapangan--penarikan-kabel-dropcore)
   - [SOP-ISP-05: Pengukuran Redaman OPM Lapangan & Berita Acara (BAST) Digital](#sop-isp-05-pengukuran-redaman-opm-lapangan--berita-acara-bast-digital)
   - [SOP-ISP-06: Monitoring Sesi PPPoE Live & Fitur Reset / Kick Sesi](#sop-isp-06-monitoring-sesi-pppoe-live--fitur-reset--kick-sesi)
5. [Bab V: Bagian III — SOP Penagihan, Pembayaran & AAA (GOGIGABILL Core)](#bab-v-bagian-iii--sop-penagihan-pembayaran--aaa-gogigabill-core)
   - [SOP-BILL-01: Siklus Otomasi Penagihan Bulanan & Notifikasi WhatsApp](#sop-bill-01-siklus-otomasi-penagihan-bulanan--notifikasi-whatsapp)
   - [SOP-BILL-02: Pembayaran Otomatis via Payment Gateway (Tripay QRIS/VA)](#sop-bill-02-pembayaran-otomatis-via-payment-gateway-tripay-qrisva)
   - [SOP-BILL-03: Prosedur Penerimaan Pembayaran Tunai / Manual Kasir](#sop-bill-03-prosedur-penerimaan-pembayaran-tunai--manual-kasir)
   - [SOP-BILL-04: Mekanisme Otomasi Isolir Penunggak (FreeRADIUS CoA)](#sop-bill-04-mekanisme-otomasi-isolir-penunggak-freeradius-coa)
   - [SOP-BILL-05: Mekanisme Pemulihan Otomatis Pasca Pelunasan (Auto-Restore)](#sop-bill-05-mekanisme-pemulihan-otomatis-pasca-pelunasan-auto-restore)
   - [SOP-BILL-06: Rekonsiliasi Keuangan, Settlement Bank & Laporan Pajak](#sop-bill-06-rekonsiliasi-keuangan-settlement-bank--laporan-pajak)
6. [Bab VI: Matriks Eskalasi Insiden, Disaster Recovery & Pemeliharaan Server](#bab-vi-matriks-eskalasi-insiden-disaster-recovery--pemeliharaan-server)

---

## BAB I: ARSITEKTUR EKOSISTEM 3 ENGINE & SINGLE SOURCE OF TRUTH

Ekosistem telekomunikasi **GOGIGANET** dibangun di atas arsitektur *microservices* terpadu dengan **1 (satu) Database Pusat PostgreSQL (`isp_billing`)** yang dilengkapi ekstensi geospasial **PostGIS**:

```
+-----------------------------------------------------------------------------------+
|                        CENTRAL POSTGRESQL + POSTGIS (isp_billing)                 |
|             (ODP Nodes, Fiber Routes, Customers, Invoices, RADIUS AAA)           |
+-----------------------------------------------------------------------------------+
          ^                                   ^                                  ^
          | (Sync via DB/API)                 | (Real-time DB)                   | (Real-time DB)
          v                                   v                                  v
+-----------------------+           +-----------------------+          +-----------------------+
|  ENGINE 1: FTTX CORE  |           |  ENGINE 2: ISP ENG.   |          | ENGINE 3: GOGIGABILL  |
|  (Physical & Infra)   |           |  (Retail Ops & Field) |          | (Financial & AAA)     |
|  • Port: 8082         |           |  • Port: 8081         |          | • Port: 8080          |
|  • OLT / ODC / ODP    |           |  • Coverage Check     |          | • Invoicing & Tripay  |
|  • Fiber Cable Route  |           |  • SPK & BAST Digital |          | • FreeRADIUS Server   |
|  • SNMP Optical Power |           |  • Sales & Referral   |          | • MikroTik CoA Isolir |
|  • TR-069 Wi-Fi ACS   |           |  • NOC Live Monitor   |          | • Revenue Share Mitra |
+-----------------------+           +-----------------------+          +-----------------------+
```

### Prinsip Bebas Tumpang Tindih (*Zero Overlap*):
1. **Engine 1 (FTTX)**: Khusus mengurus aset fisik pasif/aktif dari tower OLT hingga tiang ODP di jalanan.
2. **Engine 2 (ISP Engine)**: Khusus melayani kebutuhan pelanggan ritel (pendaftaran, survei kelayakan dropcore $\le 250\text{ m}$, penugasan teknisi pasang baru).
3. **Engine 3 (GOGIGABILL)**: Khusus memproses uang (faktur, pembayaran perbankan/QRIS) dan izin akses jaringan (FreeRADIUS login & bandwidth speed-limiter).

---

## BAB II: STANDAR PORT, DOMAIN, & MATRIKS HAK AKSES KREDENSIAL

| Layanan / Portal | Alamat Domain Resmi | Port Server | Sasaran Pengguna | Kredensial Default |
|---|---|:---:|---|---|
| **FTTX Command Center** | `https://fttx.gogiga.net.id` | `8082` | NOC Core & Tim Fiber | Akun Superuser / Staf FTTX |
| **NOC FO Command Center** | `https://noc-fo.gogiga.net.id` | `8081` | Dispatcher & Tim NOC | `admin` / `admin123` |
| **Portal Registrasi Publik** | `https://portal.gogiga.net.id` | `8081` | Calon Pelanggan Baru | Akses Terbuka (Public Self-Service) |
| **Portal Sales Marketing** | `https://sales.gogiga.net.id` | `8081` | Tim Sales / AE | `fajar` / `sales123` |
| **Portal Teknisi Lapangan** | `https://teknisi.gogiga.net.id` | `8081` | Regu Teknisi Instalasi | `nando`, `ricci`, `zikka`, `egi` |
| **Portal Rekan Jartaplok** | `https://rekan.gogiga.net.id` | `8081` | Mitra Pemilik Jaringan | `GNET-BIARO` / API Key |
| **GOGIGABILL Admin Billing**| `https://billing.gogiga.net.id` | `8080` | Finance, Kasir & Manajemen | Akun Admin Keuangan |

---

## BAB III: BAGIAN I — SOP INFRASTRUKTUR FISIK & CORE OPTIK (FTTX ENGINE)

### SOP-FTTX-01: Perencanaan & Pemasangan Tiang ODP / ODC Baru
1. **Survei Lapangan**: Tim Fiber Engineering melakukan survei jalur jalan raya / perumahan yang belum terlayani.
2. **Kapasitas Standar**:
   - **ODC**: Menggunakan splitter rasio `1:4` atau `1:8` PLC tray.
   - **ODP Tiang**: Menggunakan splitter rasio `1:8` PLC (kapasitas maksimal **8 port per tiang ODP**).
3. **Perekaman Koordinat**: Teknisi membuka `https://fttx.gogiga.net.id`, masuk ke menu **ODP**, klik tombol **"Select on Maps"**, lalu pin lokasi tepat tiang pada Google Maps.
4. **Pemberian Kode Standar**:
   - Format Payakumbuh: `ODP-PYK-xxxx` (misal: `ODP-PYK-0150`).
   - Format Biaro / Bukittinggi: `ODP-BRO-xxxx`.

### SOP-FTTX-02: Pemetaan Rute Kabel FO GIS (12/24/48 Core) & Ekspor KML
1. Buka menu **Fiber Routes** di FTTX Command Center.
2. Gambar rute kabel dari OLT ke ODC (Kabel Feeder - biasanya 24/48 Core) atau dari ODC ke ODP (Kabel Distribusi - 12/24 Core).
3. Tentukan spesifikasi jenis kabel: `G.652D` (Loose Tube Outdoor) atau `G.657A2` (Bending Insensitive).
4. Untuk kebutuhan dokumentasi perizinan dinas PUPR atau instansi daerah, gunakan tombol **`[Export KML]`** agar dapat langsung ditinjau pada aplikasi Google Earth Pro.

### SOP-FTTX-03: Sinkronisasi Aset ODP ke ISP Onboarding Engine
1. Setiap kali ada penambahan klaster tiang ODP baru yang telah selesai disambung (*splicing*), admin FTTX wajib membuka tab **Wholesale & Integration**.
2. Klik tombol **`[Sinkron ODP ke Maps ISP]`**.
3. Sistem secara instan mengirimkan metadata ODP (Kode, Nama, Lat/Lng, Kapasitas Port) ke database operasional ISP Engine sehingga tim sales langsung bisa menjual layanan di lokasi tersebut.

### SOP-FTTX-04: Standar Pengukuran Daya Optik SNMP & Ambang Batas dBm
Sistem FTTX menjalankan daemon SNMP Poller secara otomatis setiap **5 menit** untuk membaca redaman optik Rx/Tx seluruh ONT pelanggan.

**Standar Ambang Batas (Optical Threshold):**
* **$-15.0\text{ dBm}$ s/d $-23.9\text{ dBm}$**: 🟢 **NORMAL / SANGAT BAIK** (Koneksi prima tanpa kendala).
* **$-24.0\text{ dBm}$ s/d $-26.9\text{ dBm}$**: 🟡 **WARNING / REDAMAN LEMAH** (NOC menandai tiang untuk dijadwalkan pembersihan konektor SC/UPC atau perbaikan tekukan kabel).
* **$\le -27.0\text{ dBm}$**: 🔴 **CRITICAL** (Toleransi batas bawah, rentan mengalami *packet loss* tinggi).
* **$\le -35.0\text{ dBm}$ atau $0\text{ dBm}$**: ⚫ **LOS (Loss of Signal)** (Kabel putus total atau modem padam).

### SOP-FTTX-05: Zero-Touch Provisioning (SmartOLT) & TR-069 Remote Wi-Fi
1. Saat teknisi menancapkan modem baru di rumah pelanggan, OLT mendeteksi Serial Number baru pada menu **Unregistered ONTs**.
2. Klik **"Authorize ONT"**, pilih profil VLAN internet (`vlan666`) dan profil kecepatan sesuai paket pelanggan.
3. Fitur **TR-069 ACS Remote**:
   - NOC dan Customer Service dapat melihat perangkat smartphone/laptop yang tersambung ke modem pelanggan beserta nilai sinyal Wi-Fi (RSSI).
   - CS dapat membantu pelanggan mengubah nama Wi-Fi (SSID) dan kata sandi dari jarak jauh tanpa perlu teknisi datang ke rumah.

### SOP-FTTX-06: Manajemen Sewa Port Wholesale Jartaplok B2B
1. Digunakan jika ada ISP rekanan yang ingin menyewa port optik pasif milik GOGIGANET.
2. Tarif sewa port per bulan:
   - Paket 20 Mbps: Rp 154.000 / port
   - Paket 30 Mbps: Rp 169.000 / port
   - Paket 50 Mbps: Rp 50.000 (Sewa Port Pasif Murni)
3. Sistem FTTX menghitung prorata hari aktif sewa secara otomatis dan menerbitkan faktur B2B yang sudah dilengkapi kalkulasi PPN 11% dan bukti potong PPh Pasal 23 (2%).

---

## BAB IV: BAGIAN II — SOP RITEL, SALES & OPERASIONAL LAPANGAN (ISP ENGINE)

### SOP-ISP-01: Registrasi Mandiri Pelanggan & Cek Coverage Radius 250m
1. Calon pelanggan mengakses [portal.gogiga.net.id](https://portal.gogiga.net.id).
2. Sistem mendeteksi koordinat GPS rumah secara presisi:
   - **Jarak $\le 250\text{ meter}$**: Status **IN_COVERAGE**. Pelanggan dapat langsung memilih paket internet (Diamond 20M, Epic 30M, Honor 50M, Glory 100M).
   - **Jarak $> 250\text{ meter}$**: Status **PENDING_SURVEY_OVERDISTANCE**. Sistem memblokir auto-dispatch dan memasukkan permohonan ke antrean survei khusus untuk penentuan penambahan tiang sisipan atau biaya tambahan dropcore.
3. Pelanggan mengisi formulir data diri, mengunggah foto KTP, fasad rumah, dan menandatangani **Surat Perjanjian Berlangganan Digital** langsung di layar ponsel.

### SOP-ISP-02: Prospek Tim Sales, Kode Referral, & Klaim Komisi Rp 50.000
1. Tim Sales / Account Executive menggunakan portal [sales.gogiga.net.id](https://sales.gogiga.net.id).
2. Bagikan link pendaftaran ber-referral: `https://portal.gogiga.net.id/?ref=FAJAR-PYK`.
3. Setiap pelanggan yang mendaftar melalui link tersebut otomatis tercatat di bawah ID staf bersangkutan.
4. **Ketentuan Pencairan Komisi**:
   - Komisi retail tetap: **Rp 50.000,-** per pelanggan aktif terpasang.
   - Komisi berstatus *Eligible for Payout* seketika setelah Berita Acara Serah Terima (BAST) diverifikasi NOC dan pelanggan aktif internetnya.

### SOP-ISP-03: Verifikasi Dokumen KTP, Kontrak Digital, & Penerbitan SPK
1. Dispatcher NOC membuka [noc-fo.gogiga.net.id](https://noc-fo.gogiga.net.id) menu **Pendaftaran Pelanggan**.
2. Verifikasi dokumen:
   - Pastikan NIK KTP terbaca jelas dan foto rumah sesuai dengan titik koordinat maps.
   - Pastikan port ODP terdekat masih tersedia (kurang dari 8 port terisi).
3. Klik **"Terbitkan SPK Instalasi"**, pilih teknisi penanggung jawab (Nando, Ricci, Zikka, atau Egi), dan jadwalkan tanggal jam pengerjaan.

### SOP-ISP-04: Standar K3 Teknisi Lapangan & Penarikan Kabel Dropcore
1. **Alat Pelindung Diri (APD) Wajib**:
   - Helm keselamatan (*safety helmet*) tali dagu terpasang.
   - Rompi lapangan reflektif (*high-visibility vest*).
   - Sabuk pengaman tiang (*full body harness / safety belt*).
   - Sepatu boot kerja berisolasi karet.
2. **Standar Penarikan Dropcore 1-Core**:
   - Wajib menggunakan klem gantung (*S-Clamp / Dead-end clamp*) di setiap tiang tumpu.
   - Ketinggian kabel minimal melintasi jalan raya adalah **5.5 meter** dari aspal, dan jalur pemukiman minimal **4.5 meter**.
   - Hindari kabel tertekuk (*bending radius* minimal 30 mm).

### SOP-ISP-05: Pengukuran Redaman OPM Lapangan & Berita Acara (BAST) Digital
1. Teknisi wajib mengukur sinyal optik di konektor ujung rumah pelanggan menggunakan Optical Power Meter (OPM) pada panjang gelombang $1490\text{ nm}$.
2. **Kriteria Kelayakan**: Redaman wajib bernilai **$\le -23.0\text{ dBm}$** (misal: $-18.5\text{ dBm}$).
3. **Penyusunan BAST Digital**:
   - Teknisi membuka [teknisi.gogiga.net.id](https://teknisi.gogiga.net.id).
   - Masukkan nilai redaman dBm, Serial Number ONT, dan MAC Address.
   - Unggah foto bukti redaman OPM dan foto modem menyala hijau.
   - Pelanggan menandatangani BAST digital di layar HP teknisi sebagai tanda terima pekerjaan selesai.
4. Status registrasi seketika berubah menjadi **ACTIVE** dan internet aktif.

### SOP-ISP-06: Monitoring Sesi PPPoE Live & Fitur Reset / Kick Sesi
1. Dashboard NOC menampilkan status real-time setiap pelanggan:
   - 🟢 **ONLINE**: Menampilkan IP Publik/Privat dinamis, MAC Address modem, dan durasi uptime.
   - ⚪ **OFFLINE**: Menandakan modem pelanggan mati atau kabel terputus.
2. Jika ada keluhan internet lemot atau modem macet akibat IP stuck di router MikroTik, NOC cukup menekan tombol **"🔄 Kick Sesi"**.
3. Sistem mengirim paket **RFC 3576 CoA Disconnect** ke MikroTik Core `103.179.65.30` sehingga modem pelanggan melakukan *re-dial* otomatis dan mendapatkan IP segar dalam hitungan 3 detik.

---

## BAB V: BAGIAN III — SOP PENAGIHAN, PEMBAYARAN & AAA (GOGIGABILL CORE)

### SOP-BILL-01: Siklus Otomasi Penagihan Bulanan & Notifikasi WhatsApp
1. **Tanggal Terbit Invoice**: Setiap tanggal **1** setiap bulannya (pukul 00:01 WIB), GOGIGABILL Worker secara otomatis meng-*generate* invoice bulanan seluruh pelanggan aktif.
2. **Tanggal Jatuh Tempo (Due Date)**: Tanggal **20** setiap bulannya.
3. **Pengiriman Notifikasi Tagihan**:
   - **H-7 (Tanggal 13)**: Pesan WhatsApp reminder tagihan dengan link rincian pembayaran.
   - **H-1 (Tanggal 19)**: Pesan WhatsApp peringatan menjelang jatuh tempo.
   - **Hari H (Tanggal 20 jam 23:59 WIB)**: Batas akhir pembayaran sebelum sistem isolir otomatis bekerja.

### SOP-BILL-02: Pembayaran Otomatis via Payment Gateway (Tripay QRIS/VA)
1. Pelanggan membuka tautan invoice di smartphone dan memilih metode pembayaran:
   - **QRIS Real-Time**: BCA, Mandiri, GoPay, OVO, Dana, ShopeePay.
   - **Virtual Account (VA)**: BRI, Mandiri, BNI, Permata.
   - **Gerai Retail**: Indomaret & Alfamart.
2. Begitu pembayaran berhasil di Tripay, webhook diterima oleh GOGIGABILL Core dalam 1 detik.
3. Sistem secara otomatis mencatat kwitansi lunas dan mengirimkan tanda terima pembayaran via WhatsApp.

### SOP-BILL-03: Prosedur Penerimaan Pembayaran Tunai / Manual Kasir
1. Jika pelanggan membayar tunai ke kantor GOGIGANET atau dititipkan ke teknisi:
2. Kasir/Admin membuka [billing.gogiga.net.id](https://billing.gogiga.net.id), cari nama atau ID pelanggan.
3. Buka invoice terkait, klik **"Catat Pembayaran Manual"**.
4. Pilih metode kas: `Kas Kantor` atau `Transfer Rekening Bank Mandiri GMT`.
5. Invoice seketika ditandai `PAID`. Dilarang menunda pencatatan pembayaran tunai lebih dari 1x24 jam untuk mencegah terjadinya isolir keliru.

### SOP-BILL-04: Mekanisme Otomasi Isolir Penunggak (FreeRADIUS CoA)
1. Pada tanggal **21 pukul 00:05 WIB**, sistem mendeteksi seluruh tagihan yang berstatus `UNPAID` dan telah melewati jatuh tempo.
2. GOGIGABILL otomatis mengubah grup akun di database FreeRADIUS (`radusergroup`) dari paket reguler ke grup `ISOLIR`.
3. GOGIGABILL menembakkan perintah **CoA Disconnect RFC 3576** ke router MikroTik Gateway:
   - Sesi PPPoE lama pelanggan diputus seketika.
   - Saat modem pelanggan dial-up ulang, MikroTik menetapkan IP pelanggan ke dalam address-list `ISOLIR`.
   - Firewall MikroTik membatasi kecepatan ke **128 Kbps** dan mengarahkan (*redirect*) seluruh lalu lintas HTTP/HTTPS ke halaman informasi pembayaran: `http://isolir.gogiga.net.id`.

### SOP-BILL-05: Mekanisme Pemulihan Otomatis Pasca Pelunasan (Auto-Restore)
1. Begitu pelanggan yang terisolir melakukan pembayaran (baik lewat QRIS, VA, maupun konfirmasi kasir):
2. Sistem detik itu juga memulihkan grup di FreeRADIUS kembali ke paket semula (misal `Paket Epic (30M)`).
3. Sistem otomatis menembakkan paket CoA Disconnect ke MikroTik.
4. Modem pelanggan terhubung kembali dengan kecepatan penuh dan keluar dari address-list isolir secara **100% otomatis tanpa campur tangan teknisi NOC**.

### SOP-BILL-06: Rekonsiliasi Keuangan, Settlement Bank & Laporan Pajak
1. Setiap akhir bulan (tanggal 28-30), Manajer Keuangan melakukan *settlement balance* dari payment gateway Tripay ke Rekening Giro Perusahaan.
2. Ekspor laporan keuangan bulanan untuk keperluan pembukuan dan setoran pajak:
   - Rekap PPN Keluaran 11% atas tagihan ritel pelanggan.
   - Rekap PPh 23 (2%) atas transaksi sewa port wholesale Jartaplok.
   - Rekap pengeluaran operasional dan komisi sales.

---

## BAB VI: MATRIKS ESKALASI INSIDEN, DISASTER RECOVERY & PEMELIHARAAN SERVER

### 1. Matriks Penanganan Gangguan Layanan (SLA Matrix)

| Tingkat Keparahan | Contoh Kejadian | Batas Waktu Respon | Target Penyelesaian (SLA) | Penanggung Jawab |
|---|---|:---:|:---:|---|
| **Severity 1 (Kritis)** | Kabel Feeder Utama putus terkena pohon / kecelakaan, OLT padam total, atau Server VPS down. | **15 Menit** | **Maksimal 3 Jam** | Kepala NOC, Tim Splicer FO & Direktur Utama |
| **Severity 2 (Mayor)** | Satu tiang ODP padam / redaman drop serentak pada 8 pelanggan. | **30 Menit** | **Maksimal 4 Jam** | Koordinator Teknisi Lapangan |
| **Severity 3 (Minor)** | Gangguan satu rumah pelanggan (dropcore putus, Wi-Fi lambat, salah password). | **2 Jam** | **Maksimal 12 Jam** | Teknisi Piket Lapangan |

### 2. Prosedur Darurat Kabel Putus (*Cut Fiber Emergency*)
1. Sistem FTTX atau pemantauan NOC mendeteksi alarm LOS serentak pada satu klaster ODP.
2. Tim NOC menggunakan Optical Time Domain Reflectometer (OTDR) dari titik ODC untuk mengukur jarak titik putus kabel.
3. Regu penanganan darurat berangkat ke titik putus membawa *Fusion Splicer*, *Closure Sambung*, dan kabel pengganti.
4. Setelah penyambungan selesai, ukur kembali redaman di ODP terjauh sebelum menutup *closure*.

### 3. Prosedur Pencadangan Data (*Disaster Recovery & Backup*)
1. **Pencadangan Otomatis Database PostgreSQL**:
   - Skrip cron berjalan setiap hari pukul **02.00 WIB** di server:
     ```bash
     docker exec gogigabill-postgres pg_dump -U postgres isp_billing | gzip > /home/anri01/backups/isp_billing_$(date +\%F).sql.gz
     ```
   - Retensi file backup disimpan selama **30 hari**.
2. **Pencadangan File Konfigurasi**:
   - Salinan file `.env` dan konfigurasi Nginx disimpan di direktori terisolasi `/home/anri01/backups/config/`.

### 4. Cheatsheet Pemeliharaan Layanan VPS (PM2 & Docker)
* **Memeriksa Status Seluruh Engine**:
  ```bash
  pm2 status
  ```
* **Melihat Log Real-time**:
  ```bash
  pm2 logs gogiga-isp      # Log ISP Engine (Port 8081)
  pm2 logs gogiga-fttx     # Log FTTX Command Center (Port 8082)
  pm2 logs gogigabill-api  # Log Billing Engine Core (Port 8080)
  ```
* **Restart Bersih Layanan**:
  ```bash
  pm2 restart 2 --update-env   # Restart ISP Engine
  pm2 restart 3 --update-env   # Restart FTTX Engine
  pm2 restart 4 --update-env   # Restart Billing API
  ```

---

## BAB VII: LEMBAR PENGESAHAN DOKUMEN

Dokumen Master Standar Operasional Prosedur (SOP) ini berlaku mengikat bagi seluruh jajaran Direksi, Karyawan, Tim NOC, Teknisi Lapangan, Account Executive, serta Rekanan PT GOGIGA MEDIA TEKNOLOGI terhitung sejak tanggal ditetapkan.

Ditetapkan di : **Payakumbuh, Sumatera Barat**  
Pada Tanggal  : **26 September 2026**  

<br>

| Disusun & Diverifikasi Oleh: | Disetujui & Disahkan Oleh: |
| :---: | :---: |
| <br><br><br>**Nando Azkia Putra S.Kom**<br>Kepala NOC & Koordinator Core FO | <br><br><br>**Aan Rizal S.Kom**<br>Direktur Utama / Owner PT GOGIGA MEDIA TEKNOLOGI |
