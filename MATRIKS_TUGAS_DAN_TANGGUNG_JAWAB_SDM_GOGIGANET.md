# MATRIKS TUGAS, WEWENANG & TANGGUNG JAWAB (JOB DESCRIPTION)
## STRUKTUR ORGANISASI & OPERASIONAL 3 ENGINE TELEKOMUNIKASI
**PT GOGIGA MEDIA TEKNOLOGI — GOGIGANET FIBER BROADBAND**

* **Nomor Dokumen**: `GMT/HRD-JOBDESC/2026/09/003`
* **Klasifikasi**: Dokumen Resmi Tata Kelola SDM & Operasional
* **Versi / Edisi**: `1.0 (Edisi Standar Operasional 3 Engine)`
* **Tanggal Ditetapkan**: 26 September 2026
* **Disahkan Oleh**: **Aan Rizal S.Kom (Direktur Utama / Owner PT GOGIGA MEDIA TEKNOLOGI)**

---

## 1. STRUKTUR HIERARKI ORGANISASI GOGIGANET

```
                       +---------------------------------------+
                       |      DIREKTUR UTAMA / OWNER           |
                       |          Aan Rizal S.Kom              |
                       +---------------------------------------+
                                           |
         +---------------------------------+---------------------------------+
         |                                 |                                 |
+----------------------+       +-----------------------+         +-----------------------+
|  KEPALA NOC & CORE   |       |  ACCOUNT EXECUTIVE    |         | STAF KEUANGAN & KASIR |
| Nando Azkia Putra    |       |  Fajar Malem Sitepu   |         |      (Finance)        |
+----------------------+       +-----------------------+         +-----------------------+
         |                                 |                                 |
         | (Instruksi SPK)                 | (Eskalasi Survei)               | (Kwitansi / Lunas)
         v                                 v                                 v
+------------------------------------------------------------------------------------+
|                      REGU TEKNISI LAPANGAN & MAINTENANCE                           |
|                  Ricci  |  Zikka Sintio Anugrah  |  Egi                            |
+------------------------------------------------------------------------------------+
```

---

## 2. RINCIAN TUGAS & TANGGUNG JAWAB PER JABATAN

---

### A. DIREKTUR UTAMA / OWNER (SUPER ADMIN)
* **Pejabat**: **Aan Rizal S.Kom**
* **Akses Sistem Utama**:
  - GOGIGABILL Core (`https://billing.gogiga.net.id`)
  - Executive Overview ISP (`https://noc-fo.gogiga.net.id`)
  - FTTX Wholesale Master (`https://fttx.gogiga.net.id`)
* **Tanggung Jawab Utama**:
  1. Menetapkan arah kebijakan strategis, ekspansi cakupan wilayah fiber optik, dan pengadaan perangkat modal (OLT, ODC, Server, Router Core).
  2. Menentukan kebijakan penetapan harga paket retail, promo pemasaran, dan batas margin keuntungan (*Net Gross Margin*).
  3. Menyetujui kontrak kemitraan strategis B2B Wholesale (Penyelenggara Jartaplok, ISP Rekanan, dan Pelanggan Korporat Dedicated).
  4. Memantau laporan eksekutif bulanan: *Monthly Recurring Revenue* (MRR), *Wholesale Jartaplok Cost*, rasio laba, dan pencairan komisi penjualan.
* **Wewenang**:
  - Memiliki hak akses penuh (*Superuser*) untuk mengubah konfigurasi sensitif sistem, database, dan otorisasi pengeluaran dana perusahaan.

---

### B. KEPALA NETWORK OPERATIONS CENTER (NOC) & CORE FO
* **Pejabat**: **Nando Azkia Putra S.Kom**
* **Akses Sistem Utama**:
  - NOC FO Command Center (`https://noc-fo.gogiga.net.id`) — Level: `ADMIN_NOC`
  - FTTX Command Center (`https://fttx.gogiga.net.id`) — Level: `NOC Engineer`
* **Tanggung Jawab Utama**:
  1. **Stabilitas Jaringan**: Memastikan *uptime* jaringan internet dan router MikroTik Gateway POP Harau berjalan 99.8% prima.
  2. **Verifikasi & Dispatch SPK**: Memeriksa validitas pendaftaran calon pelanggan (KTP, Kontrak Digital) dan ketersediaan port ODP, lalu menerbitkan Surat Perintah Kerja (SPK) untuk teknisi.
  3. **Monitoring Redaman Optik (SNMP)**: Memantau dashboard sinyal optik dBm di FTTX Engine. Mengidentifikasi dan menandai tiang ODP yang mengalami redaman buruk ($\le -24\text{ dBm}$) atau alarm LOS.
  4. **Penanganan Gangguan L2/L3**: Mengatasi kendala sesi PPPoE macet di router MikroTik menggunakan fitur **"🔄 Kick Sesi" (CoA Disconnect RFC 3576)**.
  5. **Tanggap Darurat Kabel Putus (*Cut Fiber*)**: Mengoordinasikan pengukuran OTDR dan regu penanganan saat terjadi insiden kabel tertimpa pohon atau putus.
  6. **Sinkronisasi ODP**: Memastikan tiang ODP baru di FTTX telah tersinkronisasi sempurna ke peta GIS Onboarding ISP.
* **Key Performance Indicator (KPI)**:
  - *Uptime* jaringan $\ge 99.5\%$.
  - Waktu verifikasi pendaftaran s/d terbit SPK $\le 2\text{ jam}$.
  - Target perbaikan gangguan mayor (SLA) $\le 3\text{ jam}$.

---

### C. ACCOUNT EXECUTIVE / KOORDINATOR SALES & MARKETING
* **Pejabat**: **Fajar Malem Sitepu**
* **Akses Sistem Utama**:
  - Portal Sales & Referral (`https://sales.gogiga.net.id`) — Level: `SALES`
  - Portal Registrasi Publik (`https://portal.gogiga.net.id/?ref=FAJAR-PYK`)
* **Tanggung Jawab Utama**:
  1. **Akuisisi Pelanggan**: Mencapai target penjualan bulanan untuk pelanggan ritel perumahan dan pelanggan bisnis/korporat.
  2. **Penyebaran Referral**: Menyebarkan tautan unik pendaftaran digital ber-referral serta mengedukasi calon pelanggan cara mendaftar mandiri via Google Maps.
  3. **Tindak Lanjut Status Overdistance**: Menghubungi dan memetakan calon pelanggan yang berstatus `PENDING_SURVEY_OVERDISTANCE` (jarak $> 250\text{ meter}$) untuk opsi penarikan kabel berbayar atau usulan penambahan tiang baru.
  4. **Klaim Komisi**: Memantau daftar pelanggan yang telah sukses terpasang (*ACTIVE*) dan mengajukan rekapitulasi pencairan komisi tetap **Rp 50.000 / pelanggan aktif**.
* **Key Performance Indicator (KPI)**:
  - Target pemasangan baru minimal 30 pelanggan aktif / bulan.
  - *Conversion rate* prospek pendaftaran menjadi pelanggan aktif $\ge 80\%$.

---

### D. REGU TEKNISI LAPANGAN & LAST-MILE
* **Petugas**: **Ricci, Zikka Sintio Anugrah, Egi (Dikoordinasikan oleh Nando)**
* **Akses Sistem Utama**:
  - Portal Teknisi Mobile (`https://teknisi.gogiga.net.id`) — Level: `TECHNICIAN`
* **Tanggung Jawab Utama**:
  1. **Eksekusi Penarikan Kabel Dropcore**: Menarik kabel dropcore 1-core dari tiang ODP ke rumah pelanggan sesuai SPK dengan standar K3 (ketinggian minimal 5.5m melintas jalan raya dan 4.5m di pemukiman).
  2. **Splicing & Terminasi**: Menyambung fiber optik menggunakan Fusion Splicer dengan redaman sambungan minimal dan memasang konektor proteksi.
  3. **Pengukuran Redaman OPM**: Wajib mengukur daya optik di ujung rumah pelanggan dengan Optical Power Meter (OPM) pada gelombang 1490nm. **Wajib bernilai $\le -23.0\text{ dBm}$**.
  4. **Penyusunan BAST Digital**:
     - Menginput data Serial Number ONT, MAC Address, dan nilai dBm ke portal teknisi di HP.
     - Mengunggah foto bukti redaman OPM dan foto modem menyala normal.
     - Meminta tanda tangan digital pelanggan di layar HP sebelum meninggalkan lokasi.
  5. **Maintenance & Troubleshooting**: Menangani keluhan gangguan fisik pelanggan (kabel putus di rumah, konektor kotor, modem rusak) sesuai arahan tim NOC.
* **Key Performance Indicator (KPI)**:
  - Kualitas redaman BAST $\le -23.0\text{ dBm}$ (100% kepatuhan).
  - Waktu penyelesaian instalasi per rumah $\le 2\text{ jam}$.
  - Nol kecelakaan kerja (Zero Accident K3).

---

### E. STAF KEUANGAN, BILLING & KASIR (FINANCE)
* **Petugas**: **Staf Keuangan & Billing**
* **Akses Sistem Utama**:
  - GOGIGABILL Admin Dashboard (`https://billing.gogiga.net.id`) — Level: `FINANCE`
* **Tanggung Jawab Utama**:
  1. **Monitoring Invoice Bulanan**: Memastikan seluruh invoice bulanan telah berhasil terbit pada tanggal 1 pukul 00:01 WIB secara otomatis oleh GOGIGABILL Worker.
  2. **Pencatatan Pembayaran Manual / Kas Kantor**:
     - Menerima dan mencatat setoran pembayaran tunai atau transfer rekening manual dari pelanggan/teknisi ke dalam sistem secara *real-time*.
     - Dilarang menunda entri pembayaran tunai untuk menghindari terjadinya isolir keliru.
  3. **Rekonsiliasi Payment Gateway (Tripay)**:
     - Memeriksa kesesuaian saldo masuk antara mutasi Tripay (QRIS, VA) dengan rekening giro perusahaan.
     - Melakukan proses penarikan dana (*settlement balance*) ke rekening utama perusahaan secara berkala.
  4. **Pengawasan Jadwal Jatuh Tempo & Auto-Isolir**:
     - Memantau kepatuhan pembayaran hingga batas tanggal 20 pukul 23:59 WIB.
     - Mengaudit daftar pelanggan yang masuk status isolir otomatis pada tanggal 21 pagi.
  5. **Bagi Hasil & Pajak**:
     - Memproses pencairan komisi tim sales berdasarkan data BAST terverifikasi.
     - Menghitung kewajiban sewa port wholesale Jartaplok (PPh 23 dan PPN 11%).
* **Key Performance Indicator (KPI)**:
  - Nol selisih rekonsiliasi kas/bank (*zero discrepancy*).
  - Kecepatan entri pembayaran manual $\le 1\text{ jam}$ setelah uang diterima.

---

### F. CUSTOMER SERVICE & HELPDESK L1
* **Petugas**: **Staf Layanan Pelanggan (Customer Care)**
* **Akses Sistem Utama**:
  - Dashboard Monitoring Sesi Pelanggan (`noc-fo` / `portal`)
  - Remote TR-069 ACS Wi-Fi di FTTX (`https://fttx.gogiga.net.id`)
* **Tanggung Jawab Utama**:
  1. **Pelayanan Informasi & Keluhan**: Menerima pesan dan panggilan pelanggan via WhatsApp resmi secara ramah, cepat, dan solutif.
  2. **Edukasi Pembayaran**: Membantu mengirimkan link invoice dan memandu pelanggan cara bayar via QRIS atau Virtual Account bank.
  3. **Bantuan Wi-Fi Jarak Jauh (TR-069 ACS)**:
     - Membantu pelanggan mengganti nama Wi-Fi (SSID) dan password dari web panel tanpa harus teknisi datang ke rumah.
     - Mengecek daftar perangkat (*connected hosts*) yang tersambung ke modem jika pelanggan mengeluh internet lambat akibat kuota perangkat berlebih.
  4. **Eskalasi Tiket**: Jika gangguan bersifat fisik (kabel putus atau OPM merah), segera meneruskan tiket ke tim NOC untuk dijadwalkan kunjungan teknisi.
* **Key Performance Indicator (KPI)**:
  - Waktu respon pertama WhatsApp (*First Response Time*) $\le 3\text{ menit}$.
  - Tingkat penyelesaian keluhan mandiri tanpa kunjungan fisik (*First Contact Resolution*) $\ge 60\%$.

---

## 3. MATRIKS HUBUNGAN KERJA (RACI MATRIX)

Keterangan:
* **R (Responsible)**: Pelaksana tugas utama yang mengerjakan pekerjaan.
* **A (Accountable)**: Penanggung jawab akhir yang mengesahkan / memegang kendali keputusan.
* **C (Consulted)**: Pihak yang dimintai masukan atau data teknis sebelum eksekusi.
* **I (Informed)**: Pihak yang menerima pemberitahuan / laporan hasil kerja.

| Aktivitas Operasional | Direktur Utama | Kepala NOC | Sales / AE | Teknisi Lapangan | Staf Finance | Customer Service |
|---|:---:|:---:|:---:|:---:|:---:|:---:|
| **Pembangunan Tiang ODP Baru** | A | R | I | C | I | I |
| **Prospek & Registrasi Pelanggan**| I | I | R | I | I | C |
| **Verifikasi Dokumen & Terbit SPK**| I | A / R | I | I | I | I |
| **Tarik Kabel Dropcore & BAST** | I | A | I | R | I | I |
| **Penerbitan Invoice Bulanan** | I | I | I | I | A / R | I |
| **Pencatatan Bayar Tunai / Lunas**| I | I | I | I | A / R | I |
| **Auto-Isolir (Telat Bayar)** | I | C | I | I | A | I |
| **Troubleshooting Lemot (Kick Sesi)**| I | A / R | I | C | I | C |
| **Ganti Password Wi-Fi via TR-069**| I | C | I | I | I | R |
| **Pencairan Komisi Penjualan** | A | I | C | I | R | I |

---

## 4. LEMBAR PENGESAHAN DOKUMEN TUGAS & TANGGUNG JAWAB

Dokumen rincian tugas dan tanggung jawab ini sah dan berlaku bagi seluruh personel PT GOGIGA MEDIA TEKNOLOGI terhitung sejak tanggal ditetapkan.

Ditetapkan di : **Payakumbuh, Sumatera Barat**  
Pada Tanggal  : **26 September 2026**  

<br>

**PT GOGIGA MEDIA TEKNOLOGI**  

<br><br><br>
**Aan Rizal S.Kom**  
Direktur Utama / Owner
