# BUKU PANDUAN STANDAR OPERASIONAL PROSEDUR (SOP) RESMI
**PT GOGIGA MEDIA TEKNOLOGI — GOGIGANET FIBER BROADBAND**

* **Nomor Dokumen**: `GMT/SOP-OPS/2026/09/001`
* **Alamat Kantor**: Depan kantor Wali, Jalan Pulutan, Koto Tuo, Kec. Harau, Kabupaten Lima Puluh Kota, Sumatera Barat 26271
* **Versi / Revisi**: `1.0 (Final Operasional)`
* **Tanggal Berlaku**: 23 September 2026
* **Otoritas Pengesahan**: **Aan Rizal S.Kom (Direktur Utama / Owner PT GOGIGA MEDIA TEKNOLOGI)**
* **Penanggung Jawab Teknis**: **Nando Azkia Putra S.Kom (Kepala NOC & Core FO)**
* **Wilayah Cakupan**: Kota Payakumbuh, Kab. Lima Puluh Kota, & Sumatera Barat

---

## DAFTAR ISI SOP

| Kode Dokumen | Judul Standar Operasional Prosedur | Penanggung Jawab |
|---|---|---|
| **SOP-01** | Standar Registrasi & Onboarding Pelanggan Baru | Pelanggan & Customer Service |
| **SOP-02** | Standar Prospek Penjualan, Referral, & Komisi Sales | Tim Sales & Account Executive |
| **SOP-03** | Standar Verifikasi Data & Penerbitan SPK Teknisi | Network Operations Center (NOC) |
| **SOP-04** | Standar Keselamatan Kerja & Penarikan Kabel Dropcore | Teknisi Lapangan |
| **SOP-05** | Standar Pengukuran Redaman OPM & BAST Digital | Teknisi Lapangan |
| **SOP-06** | Standar Penanganan Gangguan Jaringan (Troubleshooting) | NOC & Tim Pemeliharaan |
| **SOP-07** | Standar Kemitraan Penyelenggara JARTAPLOK (B2B Wholesale) | Mitra Jartaplok & Finance |
| **SOP-08** | Standar Tata Kelola Eksekutif & Super User | Direktur Utama / Owner |

---

## SOP-01: REGISTRASI & ONBOARDING PELANGGAN
1. **Akses Portal Mandiri**: Calon pelanggan membuka portal resmi di [portal.gogiga.net.id](https://portal.gogiga.net.id).
2. **Pengecekan Coverage Otomatis**:
   * Sistem otomatis menghitung jarak koordinat GPS rumah ke tiang ODP terdekat.
   * **Jarak $\le$ 250 Meter**: *IN_COVERAGE* (langsung memilih paket retail).
   * **Jarak > 250 Meter**: *PENDING_SURVEY_OVERDISTANCE* (dialihkan ke antrean survei khusus teknisi/sales).
   * **Luar Area**: *UNCOVERED_WISHLIST* (disimpan sebagai data usulan penambahan tiang baru).
3. **Pengisian Identitas & Dokumen**:
   * Foto KTP dan foto fasad rumah wajib jelas.
   * Nomor WhatsApp aktif untuk penerimaan invoice dan link pelacakan status.
4. **Penandatanganan Kontrak Digital**: Calon pelanggan menandatangani kontrak langsung di layar sentuh perangkat sebelum permohonan diproses ke tim NOC.

---

## SOP-02: TIM SALES & MARKETING (ACCOUNT EXECUTIVE)
1. **Login Portal Sales**: [sales.gogiga.net.id](https://sales.gogiga.net.id) menggunakan **Username & Password** staf (Fajar Malem Sitepu / `fajar`).
2. **Tautan Referral**: Setiap staf sales memiliki kode unik (misal `FAJAR-PYK`). Tautan referral disebarkan ke publik:
   $$\text{https://portal.gogiga.net.id/?ref=FAJAR-PYK}$$
3. **Ketentuan Komisi Penjualan**:
   * **Retail / Rumah Tangga**: **Rp 50.000** per pelanggan aktif terpasang.
   * Komisi cair setelah BAST ditandatangani dan status pelanggan menjadi `ACTIVE`.
   * **Pelanggan Korporat / Dedicated**: Perhitungan komisi khusus sesuai kesepakatan BoQ yang disetujui Direktur Utama.

---

## SOP-03: NETWORK OPERATIONS CENTER (NOC) & DISPATCH
1. **Login Command Center**: [noc-fo.gogiga.net.id](https://noc-fo.gogiga.net.id) menggunakan kredensial NOC.
2. **Verifikasi Permohonan**:
   * Cek kelengkapan KTP dan keabsahan tanda tangan kontrak digital.
   * Cek ketersediaan port ODP: Batas aman maksimal **8 port per ODP**.
3. **Penerbitan Surat Perintah Kerja (SPK)**:
   * Menunjuk regu teknisi yang bertugas (Nando, Ricci, Zikka, atau Egi).
   * Menentukan tanggal dan estimasi jam tiba di lokasi.
   * Menambahkan catatan rute tiang dan nomor port ODP tujuan.
4. **Protokol Eskalasi Kapasitas Penuh (Subkontrak Tim Mitra Golden)**:
   * **Kondisi Pemicu**: Apabila antrean pemasangan baru melampaui kapasitas kerja seluruh regu internal (Nando, Ricci, Zikka, Egi jadwal penuh $\ge 48$ jam).
   * **Pendelegasian SPK**: NOC berhak menerbitkan SPK dengan menugaskan **`Mitra Teknisi Golden` (PT GNET BIARO AKSES)** untuk mencegah penumpukan antrean dan pembatalan (*churn*) oleh pelanggan.
   * **Tarif Jasa Pasang Baru (OTC Subkontrak)**: Flat **Rp 150.000 / titik pasang baru**.
   * **Mekanisme Pencatatan**:
     - *Jika di area jaringan Golden (`GNET-BIARO`)*: Biaya OTC Rp 150.000 otomatis tercatat di Berita Acara Rekonsiliasi Wholesale bulanan di portal `rekan.gogiga.net.id`.
     - *Jika di area jaringan Mandiri GoGiga*: Material kabel dropcore & modem ONT diambil dari gudang GoGiga di Koto Tuo, teknisi Golden menerima jasa murni Rp 150.000/titik.
   * **Pengawasan Standar Teknis**: Teknisi Golden wajib memenuhi batas redaman OPM standar GoGiga (**-16.00 dBm s/d -24.00 dBm**) dan menyerahkan foto dokumentasi rumah, foto ONT menyala, serta tanda tangan BAST digital.

---

## SOP-04: INSTALASI DROPCORE & TEKNISI LAPANGAN
1. **Standar K3 (Keselamatan & Kesehatan Kerja)**:
   * Wajib memakai helm safety, rompi reflektif, sepatu safety, dan sabuk pengaman tiang.
   * Tangga fiber terisolasi listrik untuk tiang bersama PLN.
2. **Spesifikasi Kabel & Penarikan**:
   * Menggunakan kabel dropcore **1 Core G.657A** (tahan tekukan).
   * Batas panjang maksimal: **250 meter** dari ODP.
   * Setiap tiang tumpu wajib dipasangi klem gantung (*S-Clamp / Dead-end clamp*).
   * Ketinggian kabel melintasi jalan raya minimal **5,5 meter**.
   * Diberikan *drip loop* (lekukan tetesan air) sebelum kabel masuk ke dinding rumah.

---

## SOP-05: PENGUKURAN REDAMAN OPTIK (OPM) & BAST DIGITAL
1. **Standar Ambang Batas Redaman (*Power Budget*)**:
   * **Rentang Standar Layak**: **-16.00 dBm s/d -24.00 dBm**
   * **Nilai Target Ideal**: **-18.00 dBm s/d -20.00 dBm**
   * **Batas Kritis / Dilarang Aktivasi**: **$> -26.00\text{ dBm}$** *(Wajib potong ulang / re-splice fast connector)*.
2. **Pengujian Sinyal & Perangkat**:
   * Pastikan lampu PON pada modem/ONT menyala hijau konstan tanpa kedip merah (*LOS*).
   * Lakukan uji kecepatan (Speedtest) minimal **90% bandwidth paket**.
   * Latensi ping ke Gateway NOC **$< 5\text{ ms}$**.
3. **Penyelesaian Berita Acara (BAST)** di [teknisi.gogiga.net.id](https://teknisi.gogiga.net.id):
   * Masukkan nilai OPM aktual, nomor Serial Number (SN) ONT, dan MAC Address.
   * Unggah foto redaman OPM dan foto hasil instalasi rapi.
   * Mintakan tanda tangan digital pelanggan di smartphone teknisi, lalu klik **"Kirim BAST"**.

---

## SOP-06: PENANGANAN GANGGUAN (TROUBLESHOOTING)
1. **Target Waktu Pemulihan (SLA)**:
   * **Gangguan Massal / Kabel Backbone Putus**: Maksimal **2 Jam**.
   * **Pelanggan Korporat / Dedicated**: Maksimal **3 Jam**.
   * **Pelanggan Rumah Tangga (Individu)**: Maksimal **6 Jam**.
2. **Langkah Penelusuran**:
   * NOC memeriksa indikasi alarm OLT.
   * Teknisi memeriksa redaman menggunakan *Optical Power Meter* dan menembakkan *Visual Fault Locator (Laser VFL)* untuk mendeteksi titik patahan atau bending kabel dropcore.
   * Segera lakukan penyambungan ulang (*re-splice*) bila ditemukan rugi-rugi redaman tinggi.

---

## SOP-07: KEMITRAAN PENYELENGGARA JARTAPLOK (WHOLESALE B2B)

### A. Skema Sewa Port Pasif FO (PT GNET BIARO AKSES - Kode: `GNET-BIARO`)
1. **Ruang Lingkup**: Pemanfaatan tiang dan jaringan distribusi kabel optik lokal milik mitra (PT GNET BIARO AKSES).
2. **Skema Tarif Sewa Port Bulanan**:
   * Paket 20 Mbps – 50 Mbps: **Rp 50.000 / port aktif / bulan**
   * Paket 100 Mbps: **Rp 90.000 / port aktif / bulan**
   * Paket 150 Mbps: **Rp 135.000 / port aktif / bulan**
   * Paket 200 Mbps: **Rp 180.000 / port aktif / bulan**
   * Paket 300 Mbps (Corporate): **Rp 270.000 / port aktif / bulan**
   * Biaya Pasang Baru (OTC): **Rp 150.000 / pelanggan baru**
3. **Rekonsiliasi & Tagihan**:
   * Rekanan dapat memantau utilisasi port langsung melalui [rekan.gogiga.net.id](https://rekan.gogiga.net.id) dengan akun `gnet_biaro`.
   * Sistem ISP menerbitkan Berita Acara Rekonsiliasi Penggunaan Port sebagai data pembanding riil untuk dicocokkan sebelum rekanan Jartaplok menerbitkan invoice resmi tagihan sewa.
   * Pelanggan yang sedang diisolir (*SUSPENDED*) mendapatkan pembebasan biaya sewa port (*Waiver Policy*).
4. **Bantuan Fulfillment Lapangan / Subkontrak Teknisi Pasang Baru**:
   * Armada teknisi PT GNET BIARO AKSES (Golden) difungsikan sebagai regu pelaksana cadangan resmi saat jadwal teknisi internal GoGiga penuh (*overload*).
   * Biaya jasa pasang baru (OTC) disepakati sebesar **Rp 150.000 / titik pemasangan** dan ditagihkan melalui Berita Acara Rekonsiliasi bulanan.
   * Setiap pekerjaan penarikan kabel wajib disertai dokumen BAST digital, foto rumah, foto modem ONT dengan PON hijau, dan uji redaman optik OPM (-16.00 s/d -24.00 dBm).

### B. Skema Bitstream 1:1 (PT Telkom Infrastruktur Indonesia / TIF - Kode: `TELKO-PYK`)
1. **Ruang Lingkup & Dokumen Acuan**: Pemanfaatan jaringan distribusi akses aktif Bitstream 1:1 milik PT Telkom Infrastruktur Indonesia. Prosedur teknis dan operasional lengkap diatur tersendiri dalam Dokumen Khusus: **`GMT/SOP-TIF/2026/09/002`** ([SOP_PELANGGAN_INFRASTRUKTUR_TIF_JARTAPLOK.md](file:///c:/Users/62811/Documents/ISP/SOP_PELANGGAN_INFRASTRUKTUR_TIF_JARTAPLOK.md)).
2. **Batas Jarak Penarikan Dropcore**: Maksimal **150 meter** dari tiang ODP Telkom (lebih ketat dibanding standar umum 250m).
3. **Arsitektur Dual-Credential Mapping**:
   * Akun resmi GoGiga (`pppoe_username` / `@gogiga.net.id`) **wajib dipertahankan** sebagai identitas master di database billing dan RADIUS.
   * Kredensial yang diwajibkan Telkom (`upstream_pppoe_username` & `upstream_pppoe_password`) dicatat sebagai atribut transport modem fisik ONT.
   * Mencegah ketergantungan (vendor lock-in) dan memudahkan pemindahan ke OLT GoGiga sendiri di masa depan.
4. **Kebijakan Tanpa Isolir (Always-On Fair Billing)**:
   * Menghindari kerugian di sisi pelanggan (pelanggan yang membayar di pertengahan bulan tetap menikmati hak internet 30 hari penuh tanpa pernah dimatikan).
   * GoGiga tidak menerapkan pemutusan sementara (isolir). Jika pelanggan menunggak hingga akhir bulan penagihan (30 hari), sistem langsung memproses **SPK Dismantle & Terminasi Permanen**.
5. **Uang Jaminan Berlangganan (Deposit Rp 500.000 Berjangka 1 Tahun)**:
   * Pelanggan menyetorkan deposit Rp 500.000 di awal (untuk mengamankan modal OTC pasang baru Telkom dan risiko piutang pascabayar).
   * **Masa Berlangganan $\ge$ 1 Tahun**: Deposit **100% dapat dikembalikan (Refundable)** jika berhenti baik-baik.
   * **Putus Berlangganan < 1 Tahun / Dismantle Nunggak**: Deposit **HANGUS OTOMATIS (FORFEITED)** sebagai penalti kompensasi biaya pasang baru Telkom dan penutup piutang tagihan.
6. **Model Penagihan Wajib Pascabayar (100% Postpaid)**:
   * Seluruh pelanggan pada rute infrastruktur TIF dilayani dengan sistem pascabayar (pemakaian bulan berjalan ditagihkan pada tanggal 01 bulan berikutnya, jatuh tempo tanggal 20, tanpa isolir hingga batas akhir bulan).

---

## SOP-08: TATA KELOLA EKSEKUTIF (SUPER USER / OWNER)
1. **Hak Akses Direktur Utama (`owner`)**:
   * Menentukan kebijakan harga paket, tarif rekanan, dan persetujuan survei khusus.
   * Mengelola akun staf, penambahan/penonaktifan pegawai, dan hak wewenang akses.
   * Mengakses laporan keuangan eksekutif, rekapitulasi laba bersih, dan utilisasi modal.
2. **Kerahasiaan Sistem**:
   * Seluruh staf dilarang menyebarkan password atau data pribadi pelanggan ke pihak luar.
   * Seluruh perubahan data terekam secara otomatis dalam log audit sistem server.
