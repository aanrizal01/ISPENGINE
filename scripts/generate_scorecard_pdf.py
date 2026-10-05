import os
from reportlab.lib.pagesizes import A4
from reportlab.lib import colors
from reportlab.lib.styles import getSampleStyleSheet, ParagraphStyle
from reportlab.platypus import (
    SimpleDocTemplate, Paragraph, Spacer, Table, TableStyle, PageBreak, KeepTogether, HRFlowable
)

def create_scorecard_pdf(filename="web/FORM_SCORECARD_EVALUASI_KPI_BULANAN.pdf"):
    doc = SimpleDocTemplate(
        filename,
        pagesize=A4,
        leftMargin=36,
        rightMargin=36,
        topMargin=36,
        bottomMargin=36
    )

    styles = getSampleStyleSheet()
    
    # Custom styles
    title_style = ParagraphStyle(
        'DocTitle',
        parent=styles['Normal'],
        fontName='Helvetica-Bold',
        fontSize=13,
        leading=16,
        textColor=colors.HexColor('#0f172a'),
        alignment=1 # Center
    )

    subtitle_style = ParagraphStyle(
        'DocSubTitle',
        parent=styles['Normal'],
        fontName='Helvetica',
        fontSize=9,
        leading=12,
        textColor=colors.HexColor('#475569'),
        alignment=1 # Center
    )

    section_header_style = ParagraphStyle(
        'SectionHeader',
        parent=styles['Normal'],
        fontName='Helvetica-Bold',
        fontSize=10,
        leading=13,
        textColor=colors.HexColor('#1e293b')
    )

    badge_style = ParagraphStyle(
        'Badge',
        parent=styles['Normal'],
        fontName='Helvetica-Bold',
        fontSize=8,
        leading=10,
        textColor=colors.HexColor('#0284c7'),
        alignment=1
    )

    body_style = ParagraphStyle(
        'BodyDark',
        parent=styles['Normal'],
        fontName='Helvetica',
        fontSize=8.5,
        leading=11,
        textColor=colors.HexColor('#1e293b')
    )

    body_bold = ParagraphStyle(
        'BodyBold',
        parent=styles['Normal'],
        fontName='Helvetica-Bold',
        fontSize=8.5,
        leading=11,
        textColor=colors.HexColor('#0f172a')
    )

    body_center = ParagraphStyle(
        'BodyCenter',
        parent=styles['Normal'],
        fontName='Helvetica',
        fontSize=8.5,
        leading=11,
        textColor=colors.HexColor('#1e293b'),
        alignment=1
    )

    story = []

    # ══════════════════════════════════════════════════════════
    # PAGE 1: COVER & PANDUAN PENILAIAN
    # ══════════════════════════════════════════════════════════
    story.append(Paragraph("OFFICIAL HR & OPERATIONAL EVALUATION FORM", badge_style))
    story.append(Spacer(1, 4))
    story.append(Paragraph("FORM SCORECARD EVALUASI & PENILAIAN KPI BULANAN", title_style))
    story.append(Paragraph("STANDAR KINERJA SDM OPERASIONAL PENYELENGGARA JASA INTERNET (ISP)", subtitle_style))
    story.append(Spacer(1, 6))
    story.append(HRFlowable(width="100%", thickness=1.5, color=colors.HexColor('#0284c7'), spaceAfter=8))

    # Meta Table
    meta_data = [
        [
            Paragraph("<b>Identitas ISP / Perusahaan:</b> [ NAMA PENYELENGGARA ISP ]", body_style),
            Paragraph("<b>Nomor Dokumen:</b> ISP/HRD-KPI/FORM/2026/10", body_style)
        ],
        [
            Paragraph("<b>Unit Bisnis:</b> Fiber Broadband & Network Operations", body_style),
            Paragraph("<b>Periode Evaluasi:</b> Bulan: Oktober  Tahun: 2026", body_style)
        ],
        [
            Paragraph("<b>Disahkan Oleh:</b> Direktur Utama / Management ISP", body_style),
            Paragraph("<b>Tanggal Evaluasi:</b> ____ / ____ / 2026", body_style)
        ],
    ]
    t_meta = Table(meta_data, colWidths=[270, 250])
    t_meta.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,-1), colors.HexColor('#f8fafc')),
        ('BOX', (0,0), (-1,-1), 0.5, colors.HexColor('#cbd5e1')),
        ('INNERGRID', (0,0), (-1,-1), 0.5, colors.HexColor('#e2e8f0')),
        ('TOPPADDING', (0,0), (-1,-1), 5),
        ('BOTTOMPADDING', (0,0), (-1,-1), 5),
        ('LEFTPADDING', (0,0), (-1,-1), 8),
        ('RIGHTPADDING', (0,0), (-1,-1), 8),
    ]))
    story.append(t_meta)
    story.append(Spacer(1, 10))

    story.append(Paragraph("PANDUAN PENGISIAN & SKEMA PEMBOBOTAN KPI", section_header_style))
    story.append(Spacer(1, 4))

    guide_text = (
        "<b>1. Metrik Kuantitatif / Sistem (Bobot 70%):</b><br/>"
        "• Data ditarik langsung dari database operasional platform ISP (Work Orders / SPK, BAST Digital, dan data pelanggan aktif).<br/>"
        "• Nilai dihitung proporsional terhadap target bulanan yang telah ditetapkan pada SOP operasional.<br/><br/>"
        "<b>2. Metrik Kualitatif / Observasi Lapangan (Bobot 30%):</b><br/>"
        "• Dinilai langsung oleh atasan langsung (Kepala Divisi / Koordinator / Direktur Utama).<br/>"
        "• Meliputi kepatuhan K3, kedisiplinan kerja, kejujuran, integritas, dan keramahan terhadap pelanggan.<br/><br/>"
        "<b>3. Kategori Predikat Akhir:</b><br/>"
        "• <b>Skor 90 – 100 (Predikat A - Sangat Baik):</b> Komisi Penuh + Insentif Prestasi / Penghargaan Staf Terbaik.<br/>"
        "• <b>Skor 75 – 89 (Predikat B - Baik / Standar):</b> Komisi Penuh sesuai ketentuan SOP baku.<br/>"
        "• <b>Skor 60 – 74 (Predikat C - Cukup):</b> Sesi pembinaan & evaluasi performa khusus.<br/>"
        "• <b>Skor &lt; 60 (Predikat D - Kurang):</b> Surat Peringatan (SP) dan peninjauan penugasan kerja."
    )
    story.append(Paragraph(guide_text, body_style))
    story.append(Spacer(1, 12))

    # Section 1 Header on Page 1
    story.append(Paragraph("1. FORMULIR PENILAIAN: REGU TEKNISI LAPANGAN", section_header_style))
    story.append(Spacer(1, 4))
    
    tech_info = [
        [
            Paragraph("<b>Nama Teknisi:</b> [ ______________________________ ]", body_style),
            Paragraph("<b>ID / Username:</b> [ ____________________ ]", body_style)
        ],
        [
            Paragraph("<b>Jabatan:</b> Teknisi Instalasi & Maintenance Fiber", body_style),
            Paragraph("<b>Penilai:</b> Kepala NOC / Koordinator", body_style)
        ]
    ]
    t_tech_info = Table(tech_info, colWidths=[270, 250])
    t_tech_info.setStyle(TableStyle([
        ('BOX', (0,0), (-1,-1), 0.5, colors.HexColor('#cbd5e1')),
        ('INNERGRID', (0,0), (-1,-1), 0.5, colors.HexColor('#e2e8f0')),
        ('TOPPADDING', (0,0), (-1,-1), 4),
        ('BOTTOMPADDING', (0,0), (-1,-1), 4),
        ('LEFTPADDING', (0,0), (-1,-1), 6),
        ('RIGHTPADDING', (0,0), (-1,-1), 6),
    ]))
    story.append(t_tech_info)
    story.append(Spacer(1, 6))

    tech_table = [
        ["No", "Indikator Kinerja Utama (KPI)", "Sumber Data", "Target", "Bobot", "Realisasi", "Skor (Bobot x Real/Target)"],
        ["A", "KINERJA TEKNIS & PRODUKTIVITAS", "", "", "70%", "", ""],
        ["1", "Jumlah SPK Pasang Baru Selesai (Completed)", "Portal Teknisi", "≥ 20 SPK", "30%", "____ SPK", "____ %"],
        ["2", "Kualitas Redaman BAST (≤ -23.0 dBm)", "BAST Digital", "100% Bagus", "25%", "____ %", "____ %"],
        ["3", "Kecepatan Penanganan Gangguan (≤ 4 Jam)", "Tiket NOC", "≥ 90%", "15%", "____ %", "____ %"],
        ["B", "DISIPLIN, K3 & PERILAKU KERJA", "", "", "30%", "", ""],
        ["4", "Kepatuhan K3 (Helm, Rompi, Sabuk Pengaman)", "Observasi", "100% Tertib", "15%", "____ %", "____ %"],
        ["5", "Presensi, Kehadiran Tepat Waktu & Sikap Pelanggan", "Absensi", "Nihil Komplain", "15%", "____ %", "____ %"],
        ["", "TOTAL SKOR AKHIR KINERJA TEKNISI BULANAN", "", "", "100%", "", "____ (Predikat: ____)"]
    ]
    t_tech = Table(tech_table, colWidths=[24, 185, 75, 65, 45, 55, 72])
    t_tech.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,0), colors.HexColor('#0f172a')),
        ('TEXTCOLOR', (0,0), (-1,0), colors.white),
        ('FONTNAME', (0,0), (-1,0), 'Helvetica-Bold'),
        ('FONTSIZE', (0,0), (-1,-1), 7.5),
        ('ALIGN', (0,0), (0,-1), 'CENTER'),
        ('ALIGN', (3,0), (-1,-1), 'CENTER'),
        ('BOX', (0,0), (-1,-1), 0.5, colors.HexColor('#64748b')),
        ('INNERGRID', (0,0), (-1,-1), 0.5, colors.HexColor('#cbd5e1')),
        ('BACKGROUND', (0,1), (-1,1), colors.HexColor('#f1f5f9')),
        ('FONTNAME', (0,1), (-1,1), 'Helvetica-Bold'),
        ('BACKGROUND', (0,5), (-1,5), colors.HexColor('#f1f5f9')),
        ('FONTNAME', (0,5), (-1,5), 'Helvetica-Bold'),
        ('BACKGROUND', (0,-1), (-1,-1), colors.HexColor('#e2e8f0')),
        ('FONTNAME', (0,-1), (-1,-1), 'Helvetica-Bold'),
        ('TOPPADDING', (0,0), (-1,-1), 3),
        ('BOTTOMPADDING', (0,0), (-1,-1), 3),
    ]))
    story.append(t_tech)
    story.append(Spacer(1, 4))
    story.append(Paragraph("<b>Catatan Khusus / Rekomendasi Atasan:</b> ____________________________________________________________________", body_style))

    # ══════════════════════════════════════════════════════════
    # PAGE 2: SALES MARKETING
    # ══════════════════════════════════════════════════════════
    story.append(PageBreak())
    story.append(Paragraph("2. FORMULIR PENILAIAN: ACCOUNT EXECUTIVE / SALES MARKETING", section_header_style))
    story.append(Spacer(1, 4))
    
    sales_info = [
        [
            Paragraph("<b>Nama Staf Sales:</b> [ ______________________________ ]", body_style),
            Paragraph("<b>ID / Partner Code:</b> [ ____________________ ]", body_style)
        ],
        [
            Paragraph("<b>Jabatan:</b> Account Executive / Sales Representative", body_style),
            Paragraph("<b>Penilai:</b> Direktur Utama / Koordinator", body_style)
        ]
    ]
    t_sales_info = Table(sales_info, colWidths=[270, 250])
    t_sales_info.setStyle(TableStyle([
        ('BOX', (0,0), (-1,-1), 0.5, colors.HexColor('#cbd5e1')),
        ('INNERGRID', (0,0), (-1,-1), 0.5, colors.HexColor('#e2e8f0')),
        ('TOPPADDING', (0,0), (-1,-1), 4),
        ('BOTTOMPADDING', (0,0), (-1,-1), 4),
        ('LEFTPADDING', (0,0), (-1,-1), 6),
        ('RIGHTPADDING', (0,0), (-1,-1), 6),
    ]))
    story.append(t_sales_info)
    story.append(Spacer(1, 6))

    sales_table = [
        ["No", "Indikator Kinerja Utama (KPI)", "Sumber Data", "Target", "Bobot", "Realisasi", "Skor (Bobot x Real/Target)"],
        ["A", "KINERJA PENJUALAN & OMSET", "", "", "70%", "", ""],
        ["1", "Pencapaian Pelanggan Baru Aktif (Active)", "Portal Sales", "30 Pelanggan", "45%", "____ Plg", "____ %"],
        ["2", "Rasio Konversi Prospek ke Pelanggan Aktif", "Database ISP", "≥ 80%", "15%", "____ %", "____ %"],
        ["3", "Follow-Up Calon Pelanggan Overdistance", "Laporan Survei", "≥ 90% Survei", "10%", "____ %", "____ %"],
        ["B", "TATA KELOLA & KOORDINASI TIM", "", "", "30%", "", ""],
        ["4", "Kecepatan Input Data Calon Pelanggan ke Portal", "Database", "≤ 2 Jam", "15%", "____ %", "____ %"],
        ["5", "Laporan Pipeline Mingguan & Kepatuhan SOP Sales", "Administrasi", "100% Disiplin", "15%", "____ %", "____ %"],
        ["", "TOTAL SKOR AKHIR KINERJA SALES BULANAN", "", "", "100%", "", "____ (Predikat: ____)"]
    ]
    t_sales = Table(sales_table, colWidths=[24, 185, 75, 65, 45, 55, 72])
    t_sales.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,0), colors.HexColor('#0f172a')),
        ('TEXTCOLOR', (0,0), (-1,0), colors.white),
        ('FONTNAME', (0,0), (-1,0), 'Helvetica-Bold'),
        ('FONTSIZE', (0,0), (-1,-1), 7.5),
        ('ALIGN', (0,0), (0,-1), 'CENTER'),
        ('ALIGN', (3,0), (-1,-1), 'CENTER'),
        ('BOX', (0,0), (-1,-1), 0.5, colors.HexColor('#64748b')),
        ('INNERGRID', (0,0), (-1,-1), 0.5, colors.HexColor('#cbd5e1')),
        ('BACKGROUND', (0,1), (-1,1), colors.HexColor('#f1f5f9')),
        ('FONTNAME', (0,1), (-1,1), 'Helvetica-Bold'),
        ('BACKGROUND', (0,5), (-1,5), colors.HexColor('#f1f5f9')),
        ('FONTNAME', (0,5), (-1,5), 'Helvetica-Bold'),
        ('BACKGROUND', (0,-1), (-1,-1), colors.HexColor('#e2e8f0')),
        ('FONTNAME', (0,-1), (-1,-1), 'Helvetica-Bold'),
        ('TOPPADDING', (0,0), (-1,-1), 3),
        ('BOTTOMPADDING', (0,0), (-1,-1), 3),
    ]))
    story.append(t_sales)
    story.append(Spacer(1, 6))

    comm_text = (
        "<b>Kalkulasi Hak Komisi Penjualan:</b><br/>"
        "• Total Pelanggan Aktif Terpasang: ______ Pelanggan × Rp 50.000,- = <b>Rp __________________</b><br/>"
        "• Bonus Prestasi / Karyawan Terbaik (Jika Predikat A): <b>Rp __________________</b><br/>"
        "• <b>Total Hak Pencairan Dana Komisi: Rp __________________</b>"
    )
    story.append(Paragraph(comm_text, body_style))

    # ══════════════════════════════════════════════════════════
    # PAGE 3: NOC & CORE NETWORK
    # ══════════════════════════════════════════════════════════
    story.append(Spacer(1, 14))
    story.append(Paragraph("3. FORMULIR PENILAIAN: KEPALA NOC & CORE NETWORK", section_header_style))
    story.append(Spacer(1, 4))
    
    noc_info = [
        [
            Paragraph("<b>Nama Staf NOC:</b> [ ______________________________ ]", body_style),
            Paragraph("<b>ID / Username:</b> [ ____________________ ]", body_style)
        ],
        [
            Paragraph("<b>Jabatan:</b> NOC Specialist & Core Network Administrator", body_style),
            Paragraph("<b>Penilai:</b> Direktur Utama / Owner", body_style)
        ]
    ]
    t_noc_info = Table(noc_info, colWidths=[270, 250])
    t_noc_info.setStyle(TableStyle([
        ('BOX', (0,0), (-1,-1), 0.5, colors.HexColor('#cbd5e1')),
        ('INNERGRID', (0,0), (-1,-1), 0.5, colors.HexColor('#e2e8f0')),
        ('TOPPADDING', (0,0), (-1,-1), 4),
        ('BOTTOMPADDING', (0,0), (-1,-1), 4),
        ('LEFTPADDING', (0,0), (-1,-1), 6),
        ('RIGHTPADDING', (0,0), (-1,-1), 6),
    ]))
    story.append(t_noc_info)
    story.append(Spacer(1, 6))

    noc_table = [
        ["No", "Indikator Kinerja Utama (KPI)", "Sumber Data", "Target", "Bobot", "Realisasi", "Skor"],
        ["1", "Uptime Jaringan & Router MikroTik Gateway", "Sistem Monitoring", "≥ 99.5%", "30%", "____ %", "____ %"],
        ["2", "Kecepatan Verifikasi Pendaftaran & Terbit SPK", "Log NOC", "≤ 2 Jam", "20%", "____ Jam", "____ %"],
        ["3", "Penyelesaian Gangguan Core Network Severity 1 & 2", "Tiket Insiden", "≤ 3 Jam", "20%", "____ Jam", "____ %"],
        ["4", "Kerapian Pemeliharaan Data ODP GIS & Kabel FO", "GIS Platform", "100% Akurat", "15%", "____ %", "____ %"],
        ["5", "Manajemen Koordinasi Tim Teknisi Lapangan", "Evaluasi Direksi", "Efektif", "15%", "____ %", "____ %"],
        ["", "TOTAL SKOR AKHIR KINERJA KEPALA NOC", "", "", "100%", "", "____ (Predikat: ____)"]
    ]
    t_noc = Table(noc_table, colWidths=[24, 185, 75, 65, 45, 55, 72])
    t_noc.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,0), colors.HexColor('#0f172a')),
        ('TEXTCOLOR', (0,0), (-1,0), colors.white),
        ('FONTNAME', (0,0), (-1,0), 'Helvetica-Bold'),
        ('FONTSIZE', (0,0), (-1,-1), 7.5),
        ('ALIGN', (0,0), (0,-1), 'CENTER'),
        ('ALIGN', (3,0), (-1,-1), 'CENTER'),
        ('BOX', (0,0), (-1,-1), 0.5, colors.HexColor('#64748b')),
        ('INNERGRID', (0,0), (-1,-1), 0.5, colors.HexColor('#cbd5e1')),
        ('BACKGROUND', (0,-1), (-1,-1), colors.HexColor('#e2e8f0')),
        ('FONTNAME', (0,-1), (-1,-1), 'Helvetica-Bold'),
        ('TOPPADDING', (0,0), (-1,-1), 3),
        ('BOTTOMPADDING', (0,0), (-1,-1), 3),
    ]))
    story.append(t_noc)

    # ══════════════════════════════════════════════════════════
    # PAGE 4: FINANCE & PENGESAHAN
    # ══════════════════════════════════════════════════════════
    story.append(PageBreak())
    story.append(Paragraph("4. FORMULIR PENILAIAN: STAF KEUANGAN & BILLING (FINANCE)", section_header_style))
    story.append(Spacer(1, 4))
    
    fin_info = [
        [
            Paragraph("<b>Nama Staf:</b> [ ______________________________ ]", body_style),
            Paragraph("<b>ID / Username:</b> [ ____________________ ]", body_style)
        ],
        [
            Paragraph("<b>Jabatan:</b> Finance & Billing Administrator", body_style),
            Paragraph("<b>Penilai:</b> Direktur Utama / Management", body_style)
        ]
    ]
    t_fin_info = Table(fin_info, colWidths=[270, 250])
    t_fin_info.setStyle(TableStyle([
        ('BOX', (0,0), (-1,-1), 0.5, colors.HexColor('#cbd5e1')),
        ('INNERGRID', (0,0), (-1,-1), 0.5, colors.HexColor('#e2e8f0')),
        ('TOPPADDING', (0,0), (-1,-1), 4),
        ('BOTTOMPADDING', (0,0), (-1,-1), 4),
        ('LEFTPADDING', (0,0), (-1,-1), 6),
        ('RIGHTPADDING', (0,0), (-1,-1), 6),
    ]))
    story.append(t_fin_info)
    story.append(Spacer(1, 6))

    fin_table = [
        ["No", "Indikator Kinerja Utama (KPI)", "Sumber Data", "Target", "Bobot", "Realisasi", "Skor"],
        ["1", "Collection Rate Tagihan (Terbayar s/d Tgl 20)", "Billing Engine", "≥ 95%", "35%", "____ %", "____ %"],
        ["2", "Ketepatan Waktu Penerbitan Invoice (Tgl 1 Tiap Bulan)", "Log Worker", "100% Tepat", "20%", "____ %", "____ %"],
        ["3", "Kecepatan Entri Pembayaran Tunai & Kasir", "Log Kasir", "≤ 1 Jam", "15%", "____ %", "____ %"],
        ["4", "Ketepatan Rekonsiliasi Kas & Mutasi Bank / Payment Gateway", "Pembukuan", "Nol Selisih", "15%", "____ %", "____ %"],
        ["5", "Kerapian Pengarsipan Faktur Pajak & Rekapitulasi", "Arsip Keuangan", "100% Tertib", "15%", "____ %", "____ %"],
        ["", "TOTAL SKOR AKHIR KINERJA STAF KEUANGAN", "", "", "100%", "", "____ (Predikat: ____)"]
    ]
    t_fin = Table(fin_table, colWidths=[24, 185, 75, 65, 45, 55, 72])
    t_fin.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,0), colors.HexColor('#0f172a')),
        ('TEXTCOLOR', (0,0), (-1,0), colors.white),
        ('FONTNAME', (0,0), (-1,0), 'Helvetica-Bold'),
        ('FONTSIZE', (0,0), (-1,-1), 7.5),
        ('ALIGN', (0,0), (0,-1), 'CENTER'),
        ('ALIGN', (3,0), (-1,-1), 'CENTER'),
        ('BOX', (0,0), (-1,-1), 0.5, colors.HexColor('#64748b')),
        ('INNERGRID', (0,0), (-1,-1), 0.5, colors.HexColor('#cbd5e1')),
        ('BACKGROUND', (0,-1), (-1,-1), colors.HexColor('#e2e8f0')),
        ('FONTNAME', (0,-1), (-1,-1), 'Helvetica-Bold'),
        ('TOPPADDING', (0,0), (-1,-1), 3),
        ('BOTTOMPADDING', (0,0), (-1,-1), 3),
    ]))
    story.append(t_fin)
    story.append(Spacer(1, 20))

    # Lembar Pengesahan
    story.append(Paragraph("LEMBAR PENGESAHAN & PENETAPAN EVALUASI BULANAN", section_header_style))
    story.append(Spacer(1, 6))

    sign_data = [
        [
            Paragraph("<b>Disusun Oleh:</b><br/>Karyawan / Staf yang Dinilai", body_center),
            Paragraph("<b>Diperiksa Oleh:</b><br/>Kepala Divisi / Atasan Langsung", body_center),
            Paragraph("<b>Disetujui & Disahkan Oleh:</b><br/>Direktur Utama / Management ISP", body_center)
        ],
        [
            Paragraph("<br/><br/><br/>( ______________________________ )", body_center),
            Paragraph("<br/><br/><br/>( ______________________________ )", body_center),
            Paragraph("<br/><br/><br/>( ______________________________ )", body_center)
        ],
        [
            Paragraph("Tanggal: ____ / ____ / 2026", body_center),
            Paragraph("Tanggal: ____ / ____ / 2026", body_center),
            Paragraph("Tanggal: ____ / ____ / 2026", body_center)
        ]
    ]
    t_sign = Table(sign_data, colWidths=[173, 173, 174])
    t_sign.setStyle(TableStyle([
        ('BOX', (0,0), (-1,-1), 0.5, colors.HexColor('#94a3b8')),
        ('INNERGRID', (0,0), (-1,-1), 0.5, colors.HexColor('#cbd5e1')),
        ('BACKGROUND', (0,0), (-1,0), colors.HexColor('#f8fafc')),
        ('TOPPADDING', (0,0), (-1,-1), 6),
        ('BOTTOMPADDING', (0,0), (-1,-1), 6),
    ]))
    story.append(t_sign)

    doc.build(story)
    print(f"Successfully generated clean multi-tenant PDF at: {filename}")

if __name__ == "__main__":
    create_scorecard_pdf("web/FORM_SCORECARD_EVALUASI_KPI_BULANAN.pdf")
    # Also overwrite the legacy file so even legacy links get the clean document!
    create_scorecard_pdf("web/FORM_SCORECARD_EVALUASI_KPI_BULANAN_GOGIGANET.pdf")
