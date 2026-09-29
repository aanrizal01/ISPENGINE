# GOGIGANET ISP - Workspace Guidelines & Modern Telco Design System

Dokumen ini adalah acuan resmi (**Design System Preset**) bagi AI assistant saat membuat, mengedit, atau mendesain antarmuka (UI/UX) dan logika kode di seluruh proyek GOGIGANET ISP.

---

## 1. Standar Desain UI Modern Telco (Wajib Dipatuhi)

Setiap pembuatan atau modifikasi halaman web / antarmuka (HTML, CSS, Tailwind, JS) WAJIB mengikuti 4 pilar berikut:

### A. Geometri Sudut (Strictly No Overrounded)
* **DILARANG** menggunakan sudut balon / kapsul: `rounded-3xl`, `rounded-2xl`, atau `rounded-xl` pada input formulir dan modal.
* **Standar Ukuran Sudut (Tailwind)**:
  - **Kontainer Utama / Kartu / Dialog Modal**: `rounded-lg` (8px).
  - **Form Input, Select, Textarea, Tombol**: `rounded-md` (6px).
  - **Badge Status / Pill / Tag Filter**: `rounded` (4px).
  - `rounded-full` **HANYA** diperbolehkan untuk titik indikator bulat kecil telemetri (misal: `w-2 h-2 rounded-full`).

### B. Palet Warna (Authoritative Telco, No Warna-Warni / Neon)
* **DILARANG** menggunakan warna neon pelangi (mint green `bg-emerald-50`, gradien kuning-oranye neon, ungu-pink).
* **Palet Resmi**:
  - **Surface Gelap Utama (Header/Hero/Sidebar)**: Deep Slate / Telco Navy (`bg-slate-900 border-slate-800 text-white`).
  - **Surface Terang**: Neutral Slate Light (`bg-slate-50 border-slate-200` atau `bg-white border-slate-200`).
  - **Aksen Utama**: Telco Blue (`text-blue-600` / `bg-blue-600 hover:bg-blue-700 text-white`).
  - **Tombol Submit / Aksi Utama**: Telco Navy (`bg-slate-900 hover:bg-slate-800 text-white font-mono text-xs font-bold`).
  - **Indikator Status (Restrained)**:
    - *Aktif / Online*: `bg-emerald-50 text-emerald-700 border-emerald-200`
    - *Menunggu / Pending / Warning*: `bg-amber-50 text-amber-700 border-amber-200`
    - *Jeda / Isolir*: `bg-slate-100 text-slate-700 border-slate-200`
    - *Batal / Error*: `bg-rose-50 text-rose-700 border-rose-200`
    - *Infrastruktur Jartaplok / Wholesale*: `bg-indigo-50 text-indigo-700 border-indigo-200`

### C. Kerapian Tata Letak & Spasi (Tidak Boleh Dempet / Cramped)
* Berikan *breathing room* (jarak napas) yang lega:
  - Padding modal dialog minimal `p-5 sm:p-6 space-y-5`.
  - Tinggi form input yang nyaman: `py-2.5 px-3.5` (bukan yang pendek/berhimpitan).
  - Label form memiliki margin `mb-1.5`.
  - Bagi formulir panjang ke dalam seksi logis dengan subjudul nomor yang rapi.
  - Gunakan responsif 2-kolom `grid grid-cols-1 md:grid-cols-2 gap-4` agar di layar laptop tidak berhimpitan.

### D. Tipografi Terstruktur
* **Plus Jakarta Sans (`font-sans`)**: Digunakan untuk seluruh teks antarmuka, judul, navigasi, dan label.
* **JetBrains Mono (`font-mono`)**: Digunakan khusus untuk data telemetri teknis:
  - Alamat IP & MAC Address
  - Kode Tiang ODP
  - Koordinat GPS (Lat, Lng)
  - Redaman Optik (dBm)
  - Nomor Registrasi (`REG-xxxx`) & Nomor SPK
  - Nominal Keuangan & Tarif (Rp)

### E. Bebas Emoticon (100% Zero Emojis)
* **DILARANG** menyisipkan karakter emoticon/emoji di seluruh antarmuka web (seperti 👑, 🚀, 🔧, 💼, ⚡, ⏱️, 🥇, dll).
* Gunakan secara konsisten ikon SVG vektor presisi dari **Lucide Icons** (`data-lucide="..."`) atau tipografi telco bersih.

---

## 2. Standar Klasifikasi Pelanggan Retail vs Bisnis
* Pelanggan **HANYA** diakui sebagai **Klien Bisnis / Corporate** jika memilih paket `paket-custom-enterprise` atau nama paket memuat kata `Custom`, `Enterprise`, `Corporate`, atau `Dedicated`.
* Catatan operasional teknis atau riwayat pergantian paket (upgrade) disimpan di `dispatch_notes`, **BUKAN** di `custom_notes` agar tidak memicu deteksi korporat yang salah.

---

## 3. Prosedur Deployment VPS
* Host: `103.179.65.72`
* Path HTML produksi: `/home/anri01/gogiga-isp/web/index.html` dan `/home/anri01/gogiga-isp/index.html`
* Restart proses PM2 ID 2 (`pm2 restart 2`).
