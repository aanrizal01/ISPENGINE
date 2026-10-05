import os
import sys
import json
import argparse
from reportlab.lib.pagesizes import A4
from reportlab.lib import colors
from reportlab.lib.styles import getSampleStyleSheet, ParagraphStyle
from reportlab.platypus import (
    SimpleDocTemplate, Paragraph, Spacer, Table, TableStyle, PageBreak, KeepTogether, HRFlowable
)

def build_scorecard_pdf(data, filename="web/FORM_SCORECARD_EVALUASI_KPI_BULANAN.pdf"):
    os.makedirs(os.path.dirname(filename) or ".", exist_ok=True)
    doc = SimpleDocTemplate(
        filename,
        pagesize=A4,
        leftMargin=36,
        rightMargin=36,
        topMargin=32,
        bottomMargin=32
    )

    styles = getSampleStyleSheet()
    
    title_style = ParagraphStyle(
        'DocTitle',
        parent=styles['Normal'],
        fontName='Helvetica-Bold',
        fontSize=12,
        leading=15,
        textColor=colors.HexColor('#0f172a'),
        alignment=1
    )

    subtitle_style = ParagraphStyle(
        'DocSubTitle',
        parent=styles['Normal'],
        fontName='Helvetica',
        fontSize=8.5,
        leading=11,
        textColor=colors.HexColor('#475569'),
        alignment=1
    )

    section_header_style = ParagraphStyle(
        'SectionHeader',
        parent=styles['Normal'],
        fontName='Helvetica-Bold',
        fontSize=9.5,
        leading=12,
        textColor=colors.HexColor('#1e293b')
    )

    badge_style = ParagraphStyle(
        'Badge',
        parent=styles['Normal'],
        fontName='Helvetica-Bold',
        fontSize=7.5,
        leading=9,
        textColor=colors.HexColor('#0284c7'),
        alignment=1
    )

    body_style = ParagraphStyle(
        'BodyDark',
        parent=styles['Normal'],
        fontName='Helvetica',
        fontSize=8,
        leading=10.5,
        textColor=colors.HexColor('#1e293b')
    )

    body_bold = ParagraphStyle(
        'BodyBold',
        parent=styles['Normal'],
        fontName='Helvetica-Bold',
        fontSize=8,
        leading=10.5,
        textColor=colors.HexColor('#0f172a')
    )

    body_center = ParagraphStyle(
        'BodyCenter',
        parent=styles['Normal'],
        fontName='Helvetica',
        fontSize=8,
        leading=10.5,
        textColor=colors.HexColor('#1e293b'),
        alignment=1
    )

    company_name = data.get("company_name", "PT. MITRA USAHA DATA")
    brand_name = data.get("brand_name", "DEV FIBER BROADBAND")
    form_no = data.get("form_no", "DEV/HRD-KPI/FORM/2026/10")
    period = data.get("period", "Bulan: Oktober Tahun: 2026")
    eval_date = data.get("eval_date", "05 Oktober 2026")
    director_name = data.get("director_name", "Lead Telecom Architect")
    noc_name = data.get("noc_name", "NOC Lab Specialist")
    technicians = data.get("technicians", [])
    sales = data.get("sales", [])

    story = []

    # ══════════════════════════════════════════════════════════
    # PAGE 1: HEADER & TEKNISI LAPANGAN
    # ══════════════════════════════════════════════════════════
    story.append(Paragraph("OFFICIAL HR & OPERATIONAL EVALUATION SCORECARD", badge_style))
    story.append(Spacer(1, 3))
    story.append(Paragraph("FORM SCORECARD EVALUASI & PENILAIAN KPI BULANAN", title_style))
    story.append(Paragraph(f"STANDAR KINERJA SDM OPERASIONAL {company_name.upper()} • {brand_name.upper()}", subtitle_style))
    story.append(Spacer(1, 5))
    story.append(HRFlowable(width="100%", thickness=1.5, color=colors.HexColor('#0284c7'), spaceAfter=6))

    # Meta Table (Prefilled)
    meta_data = [
        [
            Paragraph(f"<b>Penyelenggara ISP:</b> {company_name}", body_style),
            Paragraph(f"<b>Nomor Dokumen:</b> {form_no}", body_style)
        ],
        [
            Paragraph(f"<b>Brand Layanan:</b> {brand_name}", body_style),
            Paragraph(f"<b>Periode Evaluasi:</b> {period}", body_style)
        ],
        [
            Paragraph(f"<b>Disahkan Oleh:</b> {director_name} (Direktur Utama / Owner)", body_style),
            Paragraph(f"<b>Tanggal Penetapan:</b> {eval_date}", body_style)
        ],
    ]
    t_meta = Table(meta_data, colWidths=[270, 250])
    t_meta.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,-1), colors.HexColor('#f8fafc')),
        ('BOX', (0,0), (-1,-1), 0.5, colors.HexColor('#cbd5e1')),
        ('INNERGRID', (0,0), (-1,-1), 0.5, colors.HexColor('#e2e8f0')),
        ('TOPPADDING', (0,0), (-1,-1), 3.5),
        ('BOTTOMPADDING', (0,0), (-1,-1), 3.5),
        ('LEFTPADDING', (0,0), (-1,-1), 6),
        ('RIGHTPADDING', (0,0), (-1,-1), 6),
    ]))
    story.append(t_meta)
    story.append(Spacer(1, 8))

    # Section 1 Header
    story.append(Paragraph("1. FORMULIR PENILAIAN: REGU TEKNISI LAPANGAN", section_header_style))
    story.append(Spacer(1, 4))
    
    if not technicians:
        technicians = [
            {"name": "Field Tech Lab", "role": "Teknisi Lapangan", "completed": 0, "optical": "-", "score": 0, "grade": "BELUM DIEVALUASI"},
            {"name": "Rian Teknisi Fiber", "role": "Teknisi Lapangan", "completed": 0, "optical": "-", "score": 0, "grade": "BELUM DIEVALUASI"}
        ]

    for t_idx, tech in enumerate(technicians):
        t_name = tech.get("name", "Teknisi Lapangan")
        t_role = tech.get("role", "Teknisi Lapangan")
        t_comp = tech.get("completed", 0)
        t_opt = tech.get("optical", "-")
        t_score = tech.get("score", 0)
        t_grade = tech.get("grade", "BELUM DIEVALUASI")

        real_spk_text = f"{t_comp} SPK" if t_comp > 0 else "0 SPK (Belum ada BAST selesai)"
        real_opt_text = f"{t_opt} dBm" if t_opt != "-" else "- (Belum ada pengukuran BAST)"
        score_spk = f"{min(30, int(t_comp / 20 * 30))}%" if t_comp > 0 else "0%"
        score_opt = "25%" if t_comp > 0 else "0%"
        score_sla = "15%" if t_comp > 0 else "0%"
        score_k3 = "15%" if t_comp > 0 else "0%"
        score_pres = "15%" if t_comp > 0 else "0%"

        t_tech_info = Table([
            [
                Paragraph(f"<b>Teknisi #{t_idx+1}:</b> {t_name} ({t_role})", body_bold),
                Paragraph(f"<b>Penilai:</b> {noc_name} / {director_name}", body_style)
            ]
        ], colWidths=[310, 210])
        t_tech_info.setStyle(TableStyle([
            ('BACKGROUND', (0,0), (-1,-1), colors.HexColor('#f1f5f9')),
            ('BOX', (0,0), (-1,-1), 0.5, colors.HexColor('#cbd5e1')),
            ('TOPPADDING', (0,0), (-1,-1), 3),
            ('BOTTOMPADDING', (0,0), (-1,-1), 3),
            ('LEFTPADDING', (0,0), (-1,-1), 5),
            ('RIGHTPADDING', (0,0), (-1,-1), 5),
        ]))
        story.append(t_tech_info)
        story.append(Spacer(1, 2))

        tech_table = [
            ["No", "Indikator Kinerja Utama (KPI)", "Sumber Data", "Target", "Bobot", "Realisasi Aktual", "Skor Terhitung"],
            ["A", "KINERJA TEKNIS & PRODUKTIVITAS", "", "", "70%", "", ""],
            ["1", "Jumlah SPK Pasang Baru Selesai (Completed)", "Portal Teknisi", "≥ 20 SPK", "30%", real_spk_text, score_spk],
            ["2", "Kualitas Redaman BAST (≤ -23.0 dBm)", "BAST Digital", "100% Bagus", "25%", real_opt_text, score_opt],
            ["3", "Kecepatan Penanganan Gangguan (≤ 4 Jam)", "Tiket NOC", "≥ 90%", "15%", "Standar Operasional", score_sla],
            ["B", "DISIPLIN, K3 & PERILAKU KERJA", "", "", "30%", "", ""],
            ["4", "Kepatuhan K3 (Helm, Rompi, Sabuk Pengaman)", "Observasi", "100% Tertib", "15%", "Tertib SOP", score_k3],
            ["5", "Presensi, Kehadiran Tepat Waktu & Etika Pelanggan", "Absensi", "Nihil Komplain", "15%", "Nihil Komplain", score_pres],
            ["", "TOTAL SKOR AKHIR KINERJA TEKNISI", "", "", "100%", "", f"{t_score}/100 ({t_grade})"]
        ]
        t_tech = Table(tech_table, colWidths=[20, 180, 68, 62, 42, 88, 60])
        t_tech.setStyle(TableStyle([
            ('BACKGROUND', (0,0), (-1,0), colors.HexColor('#0f172a')),
            ('TEXTCOLOR', (0,0), (-1,0), colors.white),
            ('FONTNAME', (0,0), (-1,0), 'Helvetica-Bold'),
            ('FONTSIZE', (0,0), (-1,-1), 7),
            ('ALIGN', (0,0), (0,-1), 'CENTER'),
            ('ALIGN', (3,0), (-1,-1), 'CENTER'),
            ('BOX', (0,0), (-1,-1), 0.5, colors.HexColor('#64748b')),
            ('INNERGRID', (0,0), (-1,-1), 0.5, colors.HexColor('#cbd5e1')),
            ('BACKGROUND', (0,1), (-1,1), colors.HexColor('#f8fafc')),
            ('FONTNAME', (0,1), (-1,1), 'Helvetica-Bold'),
            ('BACKGROUND', (0,5), (-1,5), colors.HexColor('#f8fafc')),
            ('FONTNAME', (0,5), (-1,5), 'Helvetica-Bold'),
            ('BACKGROUND', (0,-1), (-1,-1), colors.HexColor('#e2e8f0')),
            ('FONTNAME', (0,-1), (-1,-1), 'Helvetica-Bold'),
            ('TOPPADDING', (0,0), (-1,-1), 2.5),
            ('BOTTOMPADDING', (0,0), (-1,-1), 2.5),
        ]))
        story.append(t_tech)
        story.append(Spacer(1, 6))

    # ══════════════════════════════════════════════════════════
    # PAGE 2: SALES MARKETING & NOC
    # ══════════════════════════════════════════════════════════
    story.append(PageBreak())
    story.append(Paragraph("2. FORMULIR PENILAIAN: ACCOUNT EXECUTIVE / SALES MARKETING", section_header_style))
    story.append(Spacer(1, 4))
    
    if not sales:
        sales = [
            {"name": "Sales Lab", "code": "SALES", "active": 0, "leads": 0, "commission": 0, "score": 0, "grade": "BELUM DIEVALUASI"}
        ]

    for s_idx, s in enumerate(sales):
        s_name = s.get("name", "Sales Lab")
        s_code = s.get("code", "SALES")
        s_act = s.get("active", 0)
        s_leads = s.get("leads", 0)
        s_comm = s.get("commission", 0)
        s_score = s.get("score", 0)
        s_grade = s.get("grade", "BELUM DIEVALUASI")

        s_act_text = f"{s_act} Pelanggan" if s_act > 0 else "0 Pelanggan (Belum ada konversi)"
        score_act = f"{min(45, int(s_act / 30 * 45))}%" if s_act > 0 else "0%"

        t_sales_info = Table([
            [
                Paragraph(f"<b>Staf Sales #{s_idx+1}:</b> {s_name} (ID: {s_code})", body_bold),
                Paragraph(f"<b>Penilai:</b> {director_name} (Direktur Utama / Owner)", body_style)
            ]
        ], colWidths=[310, 210])
        t_sales_info.setStyle(TableStyle([
            ('BACKGROUND', (0,0), (-1,-1), colors.HexColor('#f1f5f9')),
            ('BOX', (0,0), (-1,-1), 0.5, colors.HexColor('#cbd5e1')),
            ('TOPPADDING', (0,0), (-1,-1), 3),
            ('BOTTOMPADDING', (0,0), (-1,-1), 3),
            ('LEFTPADDING', (0,0), (-1,-1), 5),
            ('RIGHTPADDING', (0,0), (-1,-1), 5),
        ]))
        story.append(t_sales_info)
        story.append(Spacer(1, 2))

        sales_table = [
            ["No", "Indikator Kinerja Utama (KPI)", "Sumber Data", "Target", "Bobot", "Realisasi Aktual", "Skor Terhitung"],
            ["A", "KINERJA PENJUALAN & OMSET", "", "", "70%", "", ""],
            ["1", "Pencapaian Pelanggan Baru Aktif (Active)", "Portal Sales", "30 Pelanggan", "45%", s_act_text, score_act],
            ["2", "Rasio Konversi Prospek ke Pelanggan Aktif", "Database ISP", "≥ 80%", "15%", "0%", "0%"],
            ["3", "Follow-Up Calon Pelanggan Overdistance", "Laporan Survei", "≥ 90% Survei", "10%", "SOP Survei", "0%"],
            ["B", "TATA KELOLA & KOORDINASI TIM", "", "", "30%", "", ""],
            ["4", "Kecepatan Input Data Calon Pelanggan ke Portal", "Database", "≤ 2 Jam", "15%", "SOP", "0%"],
            ["5", "Laporan Pipeline Mingguan & Kepatuhan SOP", "Administrasi", "100% Disiplin", "15%", "Tertib", "0%"],
            ["", "TOTAL SKOR AKHIR KINERJA SALES", "", "", "100%", "", f"{s_score}/100 ({s_grade})"]
        ]
        t_sales = Table(sales_table, colWidths=[20, 180, 68, 62, 42, 88, 60])
        t_sales.setStyle(TableStyle([
            ('BACKGROUND', (0,0), (-1,0), colors.HexColor('#0f172a')),
            ('TEXTCOLOR', (0,0), (-1,0), colors.white),
            ('FONTNAME', (0,0), (-1,0), 'Helvetica-Bold'),
            ('FONTSIZE', (0,0), (-1,-1), 7),
            ('ALIGN', (0,0), (0,-1), 'CENTER'),
            ('ALIGN', (3,0), (-1,-1), 'CENTER'),
            ('BOX', (0,0), (-1,-1), 0.5, colors.HexColor('#64748b')),
            ('INNERGRID', (0,0), (-1,-1), 0.5, colors.HexColor('#cbd5e1')),
            ('BACKGROUND', (0,1), (-1,1), colors.HexColor('#f8fafc')),
            ('FONTNAME', (0,1), (-1,1), 'Helvetica-Bold'),
            ('BACKGROUND', (0,5), (-1,5), colors.HexColor('#f8fafc')),
            ('FONTNAME', (0,5), (-1,5), 'Helvetica-Bold'),
            ('BACKGROUND', (0,-1), (-1,-1), colors.HexColor('#e2e8f0')),
            ('FONTNAME', (0,-1), (-1,-1), 'Helvetica-Bold'),
            ('TOPPADDING', (0,0), (-1,-1), 2.5),
            ('BOTTOMPADDING', (0,0), (-1,-1), 2.5),
        ]))
        story.append(t_sales)
        story.append(Spacer(1, 3))

        comm_text = f"<b>Hak Komisi Penjualan:</b> {s_act} Pelanggan Aktif × Rp 50.000,- = <b>Rp {s_comm:,.0f}</b>"
        story.append(Paragraph(comm_text, body_style))
        story.append(Spacer(1, 8))

    # Section 3: NOC
    story.append(Spacer(1, 4))
    story.append(Paragraph("3. FORMULIR PENILAIAN: KEPALA NOC & CORE NETWORK", section_header_style))
    story.append(Spacer(1, 3))
    
    t_noc_info = Table([
        [
            Paragraph(f"<b>Penanggung Jawab NOC:</b> {noc_name} (Role: NOC)", body_bold),
            Paragraph(f"<b>Penilai:</b> {director_name} (Direktur Utama / Owner)", body_style)
        ]
    ], colWidths=[310, 210])
    t_noc_info.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,-1), colors.HexColor('#f1f5f9')),
        ('BOX', (0,0), (-1,-1), 0.5, colors.HexColor('#cbd5e1')),
        ('TOPPADDING', (0,0), (-1,-1), 3),
        ('BOTTOMPADDING', (0,0), (-1,-1), 3),
        ('LEFTPADDING', (0,0), (-1,-1), 5),
        ('RIGHTPADDING', (0,0), (-1,-1), 5),
    ]))
    story.append(t_noc_info)
    story.append(Spacer(1, 2))

    noc_table = [
        ["No", "Indikator Kinerja Utama (KPI)", "Sumber Data", "Target", "Bobot", "Realisasi Aktual", "Skor Terhitung"],
        ["1", "Uptime Jaringan & Router MikroTik Gateway", "Sistem Monitoring", "≥ 99.5%", "30%", "99.9% (Online)", "30%"],
        ["2", "Kecepatan Verifikasi Pendaftaran & Terbit SPK", "Log NOC", "≤ 2 Jam", "20%", "Standar SOP", "20%"],
        ["3", "Penyelesaian Gangguan Core Network Severity 1 & 2", "Tiket Insiden", "≤ 3 Jam", "20%", "SLA 100%", "20%"],
        ["4", "Pemeliharaan Data ODP GIS & Core Fiber Optic", "GIS Platform", "100% Akurat", "15%", "Data Tertib", "15%"],
        ["5", "Koordinasi & Dispatching Regu Teknisi Lapangan", "Evaluasi Direksi", "Efektif", "15%", "Efektif SOP", "15%"],
        ["", "TOTAL SKOR AKHIR KINERJA KEPALA NOC", "", "", "100%", "", "100/100 (STANDAR SOP)"]
    ]
    t_noc = Table(noc_table, colWidths=[20, 180, 68, 62, 42, 88, 60])
    t_noc.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,0), colors.HexColor('#0f172a')),
        ('TEXTCOLOR', (0,0), (-1,0), colors.white),
        ('FONTNAME', (0,0), (-1,0), 'Helvetica-Bold'),
        ('FONTSIZE', (0,0), (-1,-1), 7),
        ('ALIGN', (0,0), (0,-1), 'CENTER'),
        ('ALIGN', (3,0), (-1,-1), 'CENTER'),
        ('BOX', (0,0), (-1,-1), 0.5, colors.HexColor('#64748b')),
        ('INNERGRID', (0,0), (-1,-1), 0.5, colors.HexColor('#cbd5e1')),
        ('BACKGROUND', (0,-1), (-1,-1), colors.HexColor('#e2e8f0')),
        ('FONTNAME', (0,-1), (-1,-1), 'Helvetica-Bold'),
        ('TOPPADDING', (0,0), (-1,-1), 2.5),
        ('BOTTOMPADDING', (0,0), (-1,-1), 2.5),
    ]))
    story.append(t_noc)

    # ══════════════════════════════════════════════════════════
    # PAGE 3: FINANCE & LEMBAR PENGESAHAN
    # ══════════════════════════════════════════════════════════
    story.append(PageBreak())
    story.append(Paragraph("4. FORMULIR PENILAIAN: STAF KEUANGAN & BILLING (FINANCE)", section_header_style))
    story.append(Spacer(1, 4))
    
    t_fin_info = Table([
        [
            Paragraph(f"<b>Penanggung Jawab Keuangan:</b> Finance Administrator", body_bold),
            Paragraph(f"<b>Penilai:</b> {director_name} (Direktur Utama / Owner)", body_style)
        ]
    ], colWidths=[310, 210])
    t_fin_info.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,-1), colors.HexColor('#f1f5f9')),
        ('BOX', (0,0), (-1,-1), 0.5, colors.HexColor('#cbd5e1')),
        ('TOPPADDING', (0,0), (-1,-1), 3),
        ('BOTTOMPADDING', (0,0), (-1,-1), 3),
        ('LEFTPADDING', (0,0), (-1,-1), 5),
        ('RIGHTPADDING', (0,0), (-1,-1), 5),
    ]))
    story.append(t_fin_info)
    story.append(Spacer(1, 2))

    fin_table = [
        ["No", "Indikator Kinerja Utama (KPI)", "Sumber Data", "Target", "Bobot", "Realisasi Aktual", "Skor Terhitung"],
        ["1", "Collection Rate Tagihan Berlangganan (s/d Tgl 20)", "Billing Engine", "≥ 95%", "35%", "Standar SOP", "35%"],
        ["2", "Ketepatan Waktu Penerbitan Invoice (Tgl 1 Tiap Bulan)", "Log Worker", "100% Tepat", "20%", "Otomatis Sistem", "20%"],
        ["3", "Kecepatan Entri & Rekonsiliasi Kasir", "Log Kasir", "≤ 1 Jam", "15%", "Real-time", "15%"],
        ["4", "Ketepatan Rekonsiliasi Mutasi Bank & Payment Gateway", "Pembukuan", "Nol Selisih", "15%", "Nol Selisih", "15%"],
        ["5", "Kerapian Pengarsipan Faktur Pajak & Rekapitulasi", "Arsip Keuangan", "100% Tertib", "15%", "Tertib Administrasi", "15%"],
        ["", "TOTAL SKOR AKHIR KINERJA STAF KEUANGAN", "", "", "100%", "", "100/100 (STANDAR SOP)"]
    ]
    t_fin = Table(fin_table, colWidths=[20, 180, 68, 62, 42, 88, 60])
    t_fin.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,0), colors.HexColor('#0f172a')),
        ('TEXTCOLOR', (0,0), (-1,0), colors.white),
        ('FONTNAME', (0,0), (-1,0), 'Helvetica-Bold'),
        ('FONTSIZE', (0,0), (-1,-1), 7),
        ('ALIGN', (0,0), (0,-1), 'CENTER'),
        ('ALIGN', (3,0), (-1,-1), 'CENTER'),
        ('BOX', (0,0), (-1,-1), 0.5, colors.HexColor('#64748b')),
        ('INNERGRID', (0,0), (-1,-1), 0.5, colors.HexColor('#cbd5e1')),
        ('BACKGROUND', (0,-1), (-1,-1), colors.HexColor('#e2e8f0')),
        ('FONTNAME', (0,-1), (-1,-1), 'Helvetica-Bold'),
        ('TOPPADDING', (0,0), (-1,-1), 2.5),
        ('BOTTOMPADDING', (0,0), (-1,-1), 2.5),
    ]))
    story.append(t_fin)
    story.append(Spacer(1, 20))

    # Lembar Pengesahan
    story.append(Paragraph("LEMBAR PENGESAHAN & PENETAPAN EVALUASI BULANAN", section_header_style))
    story.append(Spacer(1, 6))

    sign_data = [
        [
            Paragraph("<b>Disusun Oleh:</b><br/>Karyawan / Staf yang Dinilai", body_center),
            Paragraph(f"<b>Diperiksa Oleh:</b><br/>Kepala Divisi NOC & Jaringan", body_center),
            Paragraph(f"<b>Disetujui & Disahkan Oleh:</b><br/>Direktur Utama / Pimpinan ISP", body_center)
        ],
        [
            Paragraph("<br/><br/><br/>( Staf Terkait )", body_center),
            Paragraph(f"<br/><br/><br/><b>( {noc_name} )</b>", body_center),
            Paragraph(f"<br/><br/><br/><b>( {director_name} )</b>", body_center)
        ],
        [
            Paragraph(f"Tanggal: {eval_date}", body_center),
            Paragraph(f"Tanggal: {eval_date}", body_center),
            Paragraph(f"Tanggal: {eval_date}", body_center)
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
    print(f"Successfully generated pre-filled PDF at: {filename}")

if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--json", help="Path to JSON file containing scorecard data")
    parser.add_argument("--out", default="web/FORM_SCORECARD_EVALUASI_KPI_BULANAN.pdf", help="Output PDF file path")
    args = parser.parse_args()

    if args.json and os.path.exists(args.json):
        with open(args.json, "r", encoding="utf-8") as f:
            data = json.load(f)
    else:
        # Default pre-filled data for dev tenant
        data = {
            "company_name": "PT. MITRA USAHA DATA",
            "brand_name": "DEV FIBER BROADBAND",
            "form_no": "DEV/HRD-KPI/FORM/2026/10",
            "period": "Bulan: Oktober Tahun: 2026",
            "eval_date": "05 Oktober 2026",
            "director_name": "Lead Telecom Architect",
            "noc_name": "NOC Lab Specialist",
            "technicians": [
                {"name": "Field Tech Lab", "role": "Teknisi Lapangan", "completed": 0, "optical": "-", "score": 0, "grade": "BELUM DIEVALUASI"},
                {"name": "Rian Teknisi Fiber", "role": "Teknisi Lapangan", "completed": 0, "optical": "-", "score": 0, "grade": "BELUM DIEVALUASI"}
            ],
            "sales": [
                {"name": "Sales Lab", "code": "SALES", "active": 0, "leads": 0, "commission": 0, "score": 0, "grade": "BELUM DIEVALUASI"}
            ]
        }

    build_scorecard_pdf(data, args.out)
