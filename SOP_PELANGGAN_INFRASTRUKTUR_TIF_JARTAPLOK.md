# BUKU PANDUAN STANDAR OPERASIONAL PROSEDUR (SOP) RESMI
## PENANGANAN PELANGGAN INFRASTRUKTUR MITRA JARTAPLOK PT TELKOM INFRASTRUKTUR INDONESIA (TIF)
**PT GOGIGA MEDIA TEKNOLOGI — GOGIGANET FIBER BROADBAND**

* **Nomor Dokumen**: `GMT/SOP-TIF/2026/09/002`
* **Klasifikasi Dokumen**: Standard Operating Procedure (SOP) Operasional & Kemitraan Wholesale
* **Versi / Revisi**: `1.0 (Definitif Operasional)`
* **Tanggal Berlaku**: 28 September 2026
* **Otoritas Pengesahan**: **Aan Rizal S.Kom (Direktur Utama / Owner PT GOGIGA MEDIA TEKNOLOGI)**
* **Penanggung Jawab Teknis**: **Nando Azkia Putra S.Kom (Kepala NOC & Core FO)**
* **Mitra Terkait**: **PT Telkom Infrastruktur Indonesia (TIF) — Regional Payakumbuh / Sumbar**
* **Kode Mitra dalam Sistem**: `TELKO-PYK`

---

## 1. LATAR BELAKANG DAN TUJUAN

Standar Operasional Prosedur (SOP) ini diterbitkan sebagai pedoman resmi bagi seluruh divisi (Sales, Network Operations Center/NOC, Teknisi Lapangan, dan Finance/Billing) dalam menangani pelanggan GoGiga ISP yang dilayani menggunakan infrastruktur jaringan akses lokal (Jartaplok) milik **PT Telkom Infrastruktur Indonesia (TIF)** dengan skema **Bitstream 1:1**.

### Tujuan Utama:
1. **Standarisasi Alur Penanganan**: Menetapkan tahapan yang terukur mulai dari verifikasi ketiadaan coverage mandiri (*uncovered*), permohonan titik ODP ke TIF, instalasi fisik, hingga aktivasi BAST digital.
2. **Perlindungan Aset Data & Identitas Pelanggan (Anti Vendor Lock-In)**: Menerapkan arsitektur **Dual-Credential Mapping** agar kredensial resmi GoGiga (`xxxx@gogiga.net.id`) tetap tercatat sebagai *Master Customer Identity* di database billing dan RADIUS, sedangkan akun dial yang diwajibkan oleh Telkom/TIF hanya diperlakukan sebagai atribut transport modem fisik ONT.
3. **Mitigasi Risiko Keuangan**: Mengantisipasi kebijakan sewa TIF (*Suspension Policy: DISALLOWED*) di mana biaya port bulanan tetap ditagihkan oleh Telkom kepada GoGiga tanpa pembebasan (*no waiver*), sekalipun pelanggan dalam masa tunggakan atau isolir.

---

## 2. MATRIKS KOMPARASI: INFRASTRUKTUR MANDIRI VS MITRA TIF VS MITRA GNET

| Parameter Operasional | Infrastruktur Mandiri GoGiga | Mitra Jartaplok GNET Biaro | Mitra Jartaplok Telkom (TIF) |
|---|---|---|---|
| **Kode Mitra di Sistem** | `INTERNAL-GOGIGA` | `GNET-BIARO` | `TELKO-PYK` |
| **Model Layanan** | End-to-End OLT Milik GoGiga | Sewa Pasif Port FO (Dark Fiber/Splitter) | **Bitstream 1:1 (Layanan Aktif Telkom)** |
| **Ketersediaan Data KML** | Lengkap di Database GIS GoGiga | Tersedia Peta Titik Tiang Bersama | **Tidak Tersedia KML Massal (Manual Attachment)** |
| **Batas Maksimal Dropcore** | 250 Meter | 250 Meter | **Maksimal 150 Meter (Standar Ketat Telkom)** |
| **Kebijakan Isolir/Suspensi** | Bebas Biaya (Milik Sendiri) | *Waiver Policy* (Biaya sewa gugur saat isolir) | **DISALLOWED (Biaya port tetap ditagih Telkom)** |
| **Model Penagihan Pelanggan** | Prabayar / Pascabayar | Prabayar / Pascabayar | **100% Wajib PASCABAYAR (Postpaid)** |
| **Biaya Pasang Baru (OTC Wholesale)** | Biaya Internal Teknisi Mandiri | Rp 150.000 / titik pasang baru | **Rp 500.000 / titik pasang baru (Ditagih Telkom)** |
| **Penyediaan Modem ONT** | Pengadaan Mandiri oleh GoGiga | Pengadaan Mandiri oleh GoGiga | **100% Disediakan oleh Telkom (Zero CAPEX ONT GoGiga)** |
| **Kredensial Dial Modem (ONT)** | Akun Resmi `@gogiga.net.id` | Akun Resmi `@gogiga.net.id` | **Wajib Akun Dial Telkom (`upstream_pppoe`)** |
| **Batas Redaman Optik (OPM)** | -16.00 dBm s/d -24.00 dBm | -16.00 dBm s/d -24.00 dBm | **-16.00 dBm s/d -24.00 dBm (Reject jika < -27.00 dBm)** |

---

## 3. ARSITEKTUR DUAL-CREDENTIAL MAPPING & ATURAN DATABASE

Untuk mencegah ketergantungan (vendor lock-in) pada Telkom dan memudahkan migrasi sirkuit di masa mendatang, sistem GOGIGANET menerapkan pemisahan data 2-tingkat secara mutlak:

### A. Master Customer Identity (Tingkat 1)
* **Atribut Database**: `pppoe_username` dan `pppoe_password`.
* **Format Penamaan**: `14xxxx@gogiga.net.id` (Nomor Layanan GoGiga).
* **Fungsi**: Digunakan sebagai kunci unik di GigaBill, server MikroTik RADIUS, pencatatan keuangan, dan faktur tagihan resmi pelanggan.
* **Larangan Mutlak**: **DILARANG** mengubah, mengganti, atau menimpa nilai `pppoe_username` dengan akun PPPoE Telkom.

### B. Upstream Transport Dial Credential (Tingkat 2)
* **Atribut Database**: `upstream_pppoe_username` dan `upstream_pppoe_password`.
* **Format Penamaan**: Diterbitkan oleh Telkom/TIF (contoh: `111400200089@telkom` atau format ID TIF).
* **Fungsi**: Hanya disetel pada konfigurasi WAN router modem ONT fisik pelanggan agar modem diizinkan melakukan sesi dial-up ke BRAS/OLT milik Telkom.

### C. Prosedur Migrasi Mandiri di Masa Depan (Exit Strategy)
Apabila GoGiga memperluas jalur kabel optik dan mendirikan OLT mandiri di wilayah tersebut:
1. Teknisi memindahkan colokan dropcore dari tiang ODP Telkom ke tiang ODP GoGiga.
2. Teknisi mengubah setelan WAN ONT dari `upstream_pppoe` Telkom kembali ke akun master `pppoe_username` (`@gogiga.net.id`).
3. Port di TIF dinonaktifkan resmi (*terminate port*) tanpa perlu membuat ulang akun pelanggan di GigaBill atau mengubah riwayat invoice pelanggan.

---

## 4. TAHAPAN PROSEDUR OPERASIONAL (END-TO-END)

### TAHAP 1: IDENTIFIKASI & SURVEI LOKASI (SALES & NOC)
1. Permohonan masuk melalui portal pendaftaran mandiri [portal.gogiga.net.id](https://portal.gogiga.net.id) atau tim Sales.
2. Sistem mendeteksi lokasi berada di luar jangkauan ODP mandiri GoGiga (> 250 meter) sehingga berstatus `PENDING_SURVEY_OVERDISTANCE` atau `UNCOVERED_WISHLIST`.
3. Sales/NOC memeriksa keberadaan tiang dan ODP Telkom/TIF di dekat lokasi rumah pelanggan:
   * **Batas Jarak Wajib**: Jarak dari titik tiang ODP TIF ke rumah pelanggan **TIDAK BOLEH LEBIH DARI 150 METER**.
   * Jika jarak melebihi 150 meter, permohonan **DITOLAK RESMI** atau dialihkan ke pembangunan tiang sisipan mandiri.
4. NOC menghubungi PIC Operasional TIF Payakumbuh untuk meminta konfirmasi ketersediaan port kosong pada ODP Telkom terkait.

---

### TAHAP 2: ATTACHMENT ODP MITRA DI PORTAL NOC (COMMAND CENTER)
1. Staff NOC membuka portal [noc-fo.gogiga.net.id](https://noc-fo.gogiga.net.id) dan masuk ke menu **Permohonan Pelanggan**.
2. Buka detail pelanggan yang bersangkutan, lalu klik tombol **Oper Order / Attach ke TIF** (atau ikon jaringan).
3. Isi formulir modal attachment:
   * **Mitra Wholesale Tujuan**: Pilih `PT Telkom Infrastruktur Indonesia (TIF) - Skema Bitstream 1:1` (Kode: `TELKO-PYK`).
   * **Kode / No. ODP Mitra**: Ketikkan kode resmi ODP Telkom (contoh: `ODP-TIF-PYK-012`).
   * **Jarak Kabel Dropcore**: Masukkan estimasi jarak riil (maksimal `150` meter).
   * **Nama / Lokasi Tiang ODP**: Masukkan patokan tiang (contoh: `Tiang Telkom Depan Kantor Lurah`).
   * **Akun Dial ONT Upstream (Opsional)**: Masukkan username dan password PPPoE dari Telkom jika PIC Telkom telah menerbitkannya di awal.
4. Klik tombol **Hubungkan Sirkuit ke Mitra**.
5. Sistem secara otomatis:
   * Mendaftarkan titik ODP mitra ke basis data lokal.
   * Menghubungkan registrasi ke mitra `TELKO-PYK`.
   * Mempromosikan status pendaftaran menjadi **Siap Pasang (`INSTALLATION_SCHEDULED`)**.
   * Menerbitkan Surat Perintah Kerja (SPK) instalasi teknisi secara otomatis.

---

### TAHAP 3: INSTALASI FISIK DROPCORE (TEKNISI LAPANGAN)
1. **Standar K3 & Izin Kerja**:
   * Teknisi wajib mengenakan perlengkapan keselamatan kerja lengkap (Helm, Rompi, Sepatu Safety, Sabuk Pengaman).
   * Menunjukkan Surat Tugas resmi GoGiga kepada aparat lingkungan dan pengawas lapangan Telkom/TIF bila diperlukan.
2. **Penarikan Kabel Dropcore**:
   * Gunakan kabel dropcore fiber optik 1 Core G.657A.
   * Pastikan panjang penarikan tidak melebihi 150 meter.
   * Ketinggian gantung kabel melintasi jalan umum minimal 5,5 meter.
   * Gunakan klem gantung (*S-Clamp*) standar telekomunikasi pada tiang tumpu.
3. **Koneksi di ODP Telkom (TIF)**:
   * Lakukan pembukaan box ODP Telkom secara hati-hati sesuai izin PIC Telkom.
   * Tancapkan patchcord/fast connector dropcore pada nomor port yang telah dialokasikan oleh TIF.
   * Tutup dan kunci kembali pintu box ODP Telkom dengan sempurna untuk mencegah masuknya air hujan.

---

### TAHAP 4: PENGUKURAN REDAMAN OPTIK (OPM)
1. Ukur redaman optik pada ujung konektor dropcore di dalam rumah pelanggan menggunakan *Optical Power Meter* (OPM) pada panjang gelombang 1490nm / 1310nm.
2. **Kriteria Kelayakan Redaman**:
   * **Sangat Baik (Ideal)**: `-18.00 dBm` s/d `-22.00 dBm`
   * **Batas Toleransi Layak**: `-22.01 dBm` s/d `-24.00 dBm`
   * **Peringatan / Margin Rendah**: `-24.01 dBm` s/d `-26.99 dBm` *(Wajib dilaporkan)*
   * **TOLAK KERJA (SOP REJECTED)**: **$< -27.00\text{ dBm}$** *(Misal: -27.50 dBm, -29.00 dBm)*.
3. Apabila redaman lebih buruk dari -27.00 dBm:
   * Teknisi **DILARANG MELAKUKAN AKTIVASI**.
   * Lakukan terminasi ulang (*re-splice*) pada *Fast Connector* pelanggan.
   * Jika redaman pada port ODP Telkom memang sudah buruk sejak awal, eskalasikan segera ke PIC TIF untuk pembersihan adapter ODP Telkom.

---

### TAHAP 5: KONFIGURASI ONT & PENYELESAIAN BAST TEKNISI
1. **Konfigurasi ONT Modem Fisik**:
   * Masuk ke antarmuka web manajemen ONT (ZTE / Huawei / Fiberhome).
   * Pada menu WAN Connection, pilih mode **PPPoE Route**.
   * Masukkan **Username Dial Jartaplok** (dari Telkom/TIF) dan **Password Dial Jartaplok**.
   * Aktifkan NAT, VLAN ID (sesuai instruksi alokasi TIF jika menggunakan VLAN tagging), dan simpan setelan.
   * Pastikan lampu indikator **PON** dan **Internet** menyala hijau konstan tanpa kedip merah (*LOS*).
2. **Pengujian Kualitas Layanan (Speedtest QoS)**:
   * Hubungkan laptop/smartphone ke port LAN 1 atau WiFi 5GHz ONT.
   * Lakukan uji kecepatan speedtest. Hasil download dan upload wajib mencapai minimal 90% dari profil paket yang dipilih pelanggan.
3. **Penyelesaian BAST Digital pada Portal Teknisi**:
   * Teknisi membuka [teknisi.gogiga.net.id](https://teknisi.gogiga.net.id).
   * Buka tugas instalasi pelanggan yang bersangkutan dan klik **Isi BAST & Aktivasi**.
   * Isi Bagian 1: Masukkan nilai riil redaman OPM (contoh: `-19.50`).
   * Isi Bagian 2:
     - Masukkan Serial Number (SN) fisik ONT.
     - Masukkan MAC Address fisik ONT.
     - Masukkan panjang kabel dropcore terpakai (dalam meter).
     - Masukkan **Username Dial Jartaplok** dan **Password Dial Jartaplok** (kredensial ini otomatis tersimpan ke profil pelanggan di server).
   * Isi Bagian 3: Masukkan hasil uji Download dan Upload Speedtest (Mbps).
   * Isi Bagian 4:
     - Lampirkan **Foto 4A**: Foto tampak depan fasad rumah pelanggan.
     - Lampirkan **Foto 4B**: Foto fisik modem ONT yang menyala rapi beserta indikator lampunya.
   * Isi Bagian 5: Mintakan tanda tangan digital serah terima pelanggan langsung di layar ponsel.
   * Klik tombol **Kirim BAST & Aktifkan Layanan**.
4. **Verifikasi Sistem Otomatis**:
   * Sistem otomatis memverifikasi bahwa redaman $\ge -27.00\text{ dBm}$.
   * Status pelanggan langsung dipromosikan menjadi **AKTIF (`ACTIVE`)**.
   * Notifikasi aktivasi dan surat tagihan/kontrak otomatis terkirim ke WhatsApp pelanggan.

---

### TAHAP 6: KEBIJAKAN PENAGIHAN PASCABAYAR TANPA ISOLIR (ALWAYS-ON FAIR BILLING)

Seluruh pelanggan yang dilayani melalui infrastruktur PT Telkom Infrastruktur Indonesia (TIF) beroperasi dengan **SKEMA 100% PASCABAYAR DENGAN KEBIJAKAN TANPA ISOLIR (ALWAYS-ON FAIR BILLING)**.

1. **Filosofi Keadilan Pelanggan (*Fair Consumer Experience*)**:
   * Pada skema isolir konvensional, apabila pelanggan terlambat membayar dan baru melunasi tagihan di pertengahan bulan (misal tanggal 15), pelanggan merasa sangat dirugikan karena diwajibkan membayar tarif satu bulan penuh padahal layanan internet sempat mati selama belasan hari.
   * Oleh karena itu, pada rute TIF, GoGiga memberlakukan prinsip **TIDAK ADA ISOLIR (NO SUSPENSION)**:
     - Koneksi internet dibiarkan **tetap aktif dan menyala normal tanpa pemutusan sementara**.
     - Pelanggan mendapatkan hak penuh pemakaian internet 30 hari tanpa jeda, sehingga saat membayar tagihan, pelanggan membayar dengan adil sesuai layanan riil yang dinikmatinya.

2. **Peran Uang Jaminan Rp 500.000 sebagai Penjamin Risiko**:
   * GoGiga dapat menerapkan kebijakan tanpa isolir ini secara aman karena **telah memegang Uang Jaminan Berlangganan (Security Deposit) sebesar Rp 500.000** di awal pendaftaran.
   * Uang jaminan ini menjadi bumper penjamin risiko piutang pemakaian pascabayar berjalan (*credit default shield*).

3. **Jadwal & Siklus Penagihan Bulanan**:
   * **Tanggal 01**: Invoice pascabayar atas pemakaian bulan sebelumnya diterbitkan otomatis oleh sistem Billing GigaBill ke WhatsApp dan portal pelanggan.
   * **Tanggal 01 s/d 20**: Masa pembayaran aktif pelanggan.
   * **Tanggal 20**: Batas akhir pembayaran resmi (*Due Date*).
   * **Tanggal 21 s/d Akhir Bulan**: Internet **tetap aktif normal**. Sistem mengirimkan pengingat ramah secara berkala (*Friendly Billing Reminder*) via WhatsApp.
   * Pelanggan yang melunasi pembayaran kapan pun di dalam periode ini (misal tanggal 10, 15, atau 25) tetap dikenakan tagihan normal tanpa denda keterlambatan dan tanpa pernah merasakan internet mati.

4. **Ambang Batas Terminasi Permanen & Pembongkaran (*Direct Dismantle*)**:
   * Karena tidak ada pemutusan sementara (isolir), tindakan penegakan dilakukan melalui **Pemutusan Permanen (Dismantle)** apabila pelanggan beritikad buruk tidak membayar sama sekali:
   * **Batas Maksimal Toleransi**: Hingga akhir bulan penagihan (30 hari menunggak / melewati tanggal 30/31).
   * Jika sampai akhir bulan tidak ada pembayaran atau konfirmasi sama sekali dari pelanggan:
     - Sistem **TIDAK MELAKUKAN ISOLIR**, melainkan langsung menerbitkan **SPK DISMANTLE & TERMINASI PERMANEN**.
     - **Uang Jaminan Rp 500.000 HANGUS OTOMATIS (FORFEITED)** untuk melunasi tagihan pascabayar pemakaian internet yang belum dibayar serta menutup biaya port.
     - Tim NOC mengirimkan tiket penghentian port (*termination request*) resmi ke PIC TIF agar sewa wholesale Telkom disetop.
     - Teknisi menarik kembali unit modem fisik ONT dari rumah pelanggan.

---

### TAHAP 7: PENANGANAN GANGGUAN (TROUBLESHOOTING) & ESKALASI TIF
1. **Laporan Gangguan Masuk**:
   * Pelanggan melapor melalui layanan pelanggan (CS) atau status monitoring menunjukkan modem offline.
2. **Pengecekan Tingkat 1 (Internal GoGiga)**:
   * NOC memeriksa status upstream PPPoE di server atau koordinasi TIF.
   * Teknisi memeriksa kondisi fisik kabel dropcore dan lampu indikator ONT di rumah pelanggan.
   * Jika lampu LOS berkedip merah: periksa redaman kabel dropcore dari ODP ke ONT.
3. **Pengecekan Tingkat 2 (Eskalasi ke TIF)**:
   * Apabila redaman pada port ODP Telkom hilang atau lebih buruk dari batas toleransi (-27 dBm) akibat gangguan kabel distribusi/backbone Telkom:
   * NOC GoGiga wajib menerbitkan Tiket Eskalasi Resmi kepada Helpdesk TIF Payakumbuh menyertakan data:
     - Nomor Tiket GoGiga: `TKT-xxxx`
     - Kode ODP Telkom: `ODP-TIF-PYK-xxx`
     - Nomor Port TIF: Port X
     - Username Dial TIF: `upstream_pppoe_username`
     - Nilai Redaman Terukur: `xx.xx dBm`
     - Koordinat GPS Tiang
4. **Target Pemulihan (SLA Kemitraan)**:
   * Gangguan pada segmen dropcore GoGiga: Maksimal **4 Jam**.
   * Gangguan pada segmen distribusi/kabel feeder TIF: Mengikuti SLA kemitraan Telkom (Maksimal **8 Jam Kerja**).

---

## 5. TANGGUNG JAWAB & DISIPLIN KERJA

1. **Divisi Sales**: Wajib memverifikasi bahwa titik rumah calon pelanggan tidak melebihi jarak 150 meter dari ODP Telkom sebelum menjanjikan pemasangan kepada konsumen.
2. **Divisi NOC**: Bertanggung jawab penuh atas keakuratan penginputan data kode ODP TIF, nomor registrasi, serta koordinasi alokasi port resmi dengan pihak Telkom.
3. **Divisi Teknisi Lapangan**: Wajib mematuhi standar redaman (OPM $\ge -27.00\text{ dBm}$), memastikan kerapian instalasi, dan menginput kredensial dial Jartaplok secara presisi pada form BAST digital jika menangani instalasi mandiri.
4. **Divisi Finance**: Bertanggung jawab mengawasi laporan rekonsiliasi bulanan tagihan sewa port bitstream dari PT Telkom Infrastruktur Indonesia dan memastikan prosedur dismantle dieksekusi tepat waktu guna mencegah timbulnya biaya sewa port pasif yang merugikan perusahaan.

---

## 6. PROTOKOL KHUSUS: PEMASANGAN FISIK DIHANDLE OLEH TEKNISI TELKOM (FULL FULFILLMENT BY TIF)

Apabila dalam perjanjian operasional disepakati bahwa **penarikan kabel dan pemasangan fisik ONT di rumah pelanggan dilakukan langsung oleh tim teknisi Telkom (Telkom Akses / TIF)**, maka prosedur operasional disesuaikan sebagai berikut:

### A. Peran Para Pihak (Model Retail Service Provider - RSP)
* **GoGiga ISP**: Bertindak sebagai *Commercial Owner & Retail Service Provider*. Hubungan kontrak, penagihan bulanan, billing GigaBill, dan *Customer Support* 100% berada di bawah kendali GoGiga.
* **Teknisi Telkom (TIF / Telkom Akses)**: Bertindak sebagai *Last-Mile Fulfillment Subcontractor* (jasa penarikan dropcore fisik, penyediaan/pemasangan modem ONT, dan koneksi ke tiang ODP Telkom).

### B. Alur Kerja Operasional (Step-by-Step)

```
 [1. Calon Pelanggan] ──► [2. NOC GoGiga Order ke PIC TIF] ──► [3. Teknisi Telkom Pasang Fisik]
                                                                        │
 [6. CS GoGiga QC & Onboarding] ◄── [5. NOC GoGiga Input & Aktifkan] ◄── [4. Telkom Kirim Laporan Hasil Pasang]
```

1. **Penerusan Order ke Telkom (Order Placement)**:
   * Setelah permohonan pelanggan terverifikasi di portal GoGiga, staff NOC GoGiga meneruskan order kerja ke grup koordinasi / portal B2B PIC Telkom Payakumbuh menyertakan:
     - Nama & Nomor Telepon Pelanggan
     - Alamat Lengkap & Titik Koordinat GPS
     - Kode ODP Telkom Tujuan (hasil survei awal)
     - Paket Bandwidth yang dipesan
   * Telkom menerbitkan Nomor Tiket Pasang Telkom (Nomor SC / SPK Telkom).
   * Status di portal GoGiga: Di-attach ke `TELKO-PYK` dengan catatan nomor tiket Telkom pada `dispatch_notes`.

2. **Edukasi Awal ke Pelanggan (Pencegahan Kebingungan Merek)**:
   * Tim Customer Service (CS) GoGiga WAJIB menghubungi pelanggan sebelum teknisi datang:
     > *"Bapak/Ibu [Nama], untuk penarikan jalur kabel fiber optik ke rumah akan dibantu oleh tim mitra infrastruktur lapangan kami (rekanan teknisi Telkom/TIF). Seluruh paket internet, akun resmi, dan pembayaran bulanan tetap resmi di bawah naungan GoGiga Fiber."*
   * Edukasi ini krusial agar pelanggan tidak mengira mereka mendaftar langsung ke produk ritel Telkom.

3. **Pelaksanaan Lapangan oleh Teknisi Telkom**:
   * Teknisi Telkom mendatangi rumah pelanggan, menarik dropcore dari ODP, dan memasang modem ONT.
   * Teknisi Telkom memasukkan akun PPPoE dial milik Telkom (`upstream_pppoe`) ke dalam modem ONT hingga lampu PON dan Internet menyala hijau konstan.
   * Teknisi Telkom membuat Berita Acara internal Telkom (Laporan Hasil Pasang / LHP).

4. **Penyerahan Data Hasil Pasang (LHP Telkom ke NOC GoGiga)**:
   * PIC Telkom menyerahkan LHP kepada NOC GoGiga yang memuat 5 data wajib:
     1. **Nomor Port ODP**: Nomor port fisik yang dicolok di ODP Telkom.
     2. **Redaman OPM**: Nilai dBm hasil ukur teknisi Telkom (wajib $\ge -27.00\text{ dBm}$).
     3. **Identitas Perangkat**: Serial Number (SN) dan MAC Address modem ONT yang dipasang.
     4. **Akun Dial Upstream**: Username dan Password PPPoE Telkom yang aktif mendial.
     5. **Waktu Aktif (*Live Date*)**: Tanggal dan jam sirkuit pertama kali aktif online.

5. **Aktivasi Administratif oleh NOC GoGiga di Portal [noc-fo.gogiga.net.id](https://noc-fo.gogiga.net.id)**:
   * Karena teknisi GoGiga tidak ke lapangan, **Staff NOC GoGiga yang melakukan penutupan order secara administratif**:
     - Buka data pelanggan di portal NOC.
     - Masukkan kredensial dial dari LHP Telkom pada kartu **Akun Dial Fisik Modem (Jartaplok)** (`detail-upstream-pppoe-user` & `detail-upstream-pppoe-pass`), lalu klik **Simpan Kredensial Dial**.
     - Buka approval/BAST, masukkan nilai redaman OPM, Serial Number ONT, dan MAC Address dari LHP Telkom.
     - Klik **Promosikan / Aktifkan Layanan (`ACTIVE`)**.
   * Sistem otomatis menyinkronkan status ke Billing GigaBill, dan siklus tagihan bulanan resmi dimulai terhitung sejak *Live Date* Telkom.

6. **Panggilan Pengawalan Mutu (Quality Call & Welcome Call oleh CS GoGiga)**:
   * Dalam waktu maksimal 1x24 jam setelah aktivasi, CS GoGiga menghubungi pelanggan:
     - Memastikan koneksi internet telah lancar dan dinikmati keluarga.
     - Membantu pelanggan mengganti nama WiFi (SSID) dan password sesuai keinginan pelanggan (bisa dipandu atau via remote ONT).
     - Mengingatkan kembali Nomor Pelanggan resmi GoGiga (`REG-xxxx`), tanggal jatuh tempo bulanan, dan nomor hotline bantuan CS GoGiga.

### C. Pembagian Tanggung Jawab (Demarcation Line & Garansi)

| Ruang Lingkup | Pelaksana / Penanggung Jawab | Catatan Finansial & Operasional |
|---|---|---|
| **Penarikan Kabel Dropcore & Sambungan ODP** | PT Telkom Infrastruktur Indonesia (TIF) | Kerusakan fisik kabel luar ruang (dropcore putus/redaman drop di ODP) dieskalasikan ke Helpdesk TIF. |
| **Perangkat ONT Modem** | 100% Disediakan oleh Telkom | Unit ONT fisik dipasok oleh Telkom (termasuk dalam paket pasang baru). GoGiga bebas CAPEX pembelian modem. Garansi kerusakan / tukar guling modem rusak ditanggung sepenuhnya oleh Telkom. |
| **Kredensial Dial ONT (Transport)** | Telkom (`upstream_pppoe`) | Diterbitkan dan dikelola Telkom untuk akses bitstream. |
| **Identitas Master, Billing, & Invoice** | PT GOGIGA MEDIA TEKNOLOGI | Pelanggan hanya membayar tagihan ke rekening resmi GoGiga. |
| **Biaya Pasang Baru (OTC / PSB)** | Rp 500.000 (Ditagihkan Telkom ke GoGiga) | Biaya *one-time charge* pasang baru sebesar Rp 500.000 ditagihkan Telkom ke GoGiga pada invoice wholesale perdana. GoGiga dapat membebankan biaya ini ke pelanggan atau mengikatnya dengan komitmen kontrak berlangganan. |
| **Penghentian Layanan (Dismantle)** | Koordinasi Resmi via Tiket TIF | Penarikan port dilakukan via tiket resmi ke PIC TIF; teknisi GoGiga dilarang mencabut kabel di ODP Telkom tanpa izin. |

### D. Kebijakan Uang Jaminan Berlangganan (Security Deposit) & Penalti Terminasi Dini

Guna mengamankan biaya modal pasang baru (OTC) sebesar Rp 500.000 yang dibayarkan GoGiga kepada PT Telkom Infrastruktur Indonesia (TIF), diberlakukan skema **Uang Jaminan Berlangganan (Security Deposit 1 Tahun)**:

1. **Penyetoran Uang Jaminan di Awal**:
   * Setiap pelanggan baru yang mendaftar melalui jalur infrastruktur Telkom/TIF wajib menyetorkan **Uang Jaminan Berlangganan sebesar Rp 500.000** saat pendaftaran disetujui.
   * Pada sistem Billing GigaBill, nominal ini dicatat sebagai **Deposit Jaminan Dibekukan (`held_balance`)** dengan masa tahan (*hold lock-in*) selama **12 Bulan (1 Tahun)** sejak tanggal aktivasi.
   * Uang jaminan ini bersifat pasif dan **tidak memotong tagihan internet bulanan berjalan**.

2. **Ketentuan Pengembalian (100% Refundable $\ge$ 1 Tahun)**:
   * Apabila pelanggan telah menyelesaikan masa berlangganan minimal **1 Tahun (12 bulan)** berturut-turut tanpa tunggakan:
     - Uang jaminan Rp 500.000 **dapat dikembalikan penuh (100% Refundable)** secara tunai atau transfer ke rekening pelanggan jika pelanggan memutuskan berhenti berlangganan secara baik-baik dengan mengembalikan modem ONT Telkom.
     - Pelanggan juga dapat memilih untuk mengalihkan uang jaminan tersebut menjadi saldo kredit/deposit aktif guna memotong tagihan internet pada bulan ke-13 dan seterusnya.

3. **Ketentuan Penalti / Uang Jaminan Hangus (< 1 Tahun)**:
   * Apabila pelanggan memutuskan berhenti berlangganan, meminta pemutusan sepihak, atau diputus paksa (*dismantle*) oleh sistem akibat menunggak sebelum mencapai masa 1 tahun (< 12 bulan):
     - **UANG JAMINAN BERLANGGANAN RP 500.000 HANGUS OTOMATIS (FORFEITED)** sebagai denda ganti rugi pemutusan dini (*Early Termination Penalty*).
     - Dana jaminan yang hangus tersebut dialokasikan GoGiga untuk menutupi beban biaya pasang baru (OTC PSB) Rp 500.000 yang telah dibayarkan kepada pihak Telkom/TIF di awal pemasangan.
     - GoGiga tetap berhak menarik kembali unit modem fisik ONT Telkom dari rumah pelanggan.

---

*Dokumen ini sah dan mengikat seluruh unit operasional PT GOGIGA MEDIA TEKNOLOGI sejak tanggal ditetapkan.*
