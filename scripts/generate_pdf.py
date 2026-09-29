import os
import sys
import subprocess
import markdown

def convert_md_to_pdf(md_file, pdf_file, doc_title, subtitle, badge):
    html_file = md_file.replace(".md", ".html")

    with open(md_file, "r", encoding="utf-8") as f:
        md_text = f.read()

    html_content = markdown.markdown(md_text, extensions=['tables', 'fenced_code', 'nl2br'])

    full_html = f"""<!DOCTYPE html>
<html lang="id">
<head>
    <meta charset="UTF-8">
    <title>{doc_title}</title>
    <style>
        @page {{
            size: A4;
            margin: 20mm 15mm 20mm 15mm;
            @bottom-right {{
                content: "Hal " counter(page);
                font-size: 9pt;
                color: #64748b;
            }}
        }}
        body {{
            font-family: 'Segoe UI', -apple-system, BlinkMacSystemFont, Roboto, sans-serif;
            color: #1e293b;
            line-height: 1.6;
            font-size: 10pt;
            background-color: #ffffff;
            margin: 0;
            padding: 0;
        }}
        h1 {{
            font-size: 17pt;
            color: #0f172a;
            border-bottom: 2.5px solid #2563eb;
            padding-bottom: 6px;
            margin-top: 0;
            margin-bottom: 12px;
            text-transform: uppercase;
        }}
        h2 {{
            font-size: 13pt;
            color: #1e3a8a;
            border-bottom: 1.5px solid #cbd5e1;
            padding-bottom: 4px;
            margin-top: 20px;
            margin-bottom: 10px;
            page-break-after: avoid;
        }}
        h3 {{
            font-size: 11pt;
            color: #0369a1;
            margin-top: 14px;
            margin-bottom: 6px;
            page-break-after: avoid;
        }}
        p, li {{
            color: #334155;
            text-align: justify;
        }}
        ul, ol {{
            margin-top: 4px;
            margin-bottom: 10px;
            padding-left: 20px;
        }}
        li {{
            margin-bottom: 3px;
        }}
        table {{
            width: 100%;
            border-collapse: collapse;
            margin: 12px 0;
            font-size: 9pt;
            page-break-inside: avoid;
        }}
        th, td {{
            border: 1px solid #cbd5e1;
            padding: 7px 9px;
            text-align: left;
            vertical-align: top;
        }}
        th {{
            background-color: #f1f5f9;
            color: #0f172a;
            font-weight: 600;
            text-align: center;
        }}
        tr:nth-child(even) {{
            background-color: #f8fafc;
        }}
        code {{
            background-color: #f1f5f9;
            color: #0f172a;
            padding: 1px 5px;
            border-radius: 4px;
            font-family: 'Consolas', monospace;
            font-size: 8.5pt;
            border: 1px solid #e2e8f0;
        }}
        pre {{
            background-color: #0f172a;
            color: #f8fafc;
            padding: 10px 12px;
            border-radius: 6px;
            overflow-x: auto;
            font-size: 8pt;
            line-height: 1.4;
            page-break-inside: avoid;
        }}
        pre code {{
            background-color: transparent;
            color: inherit;
            border: none;
            padding: 0;
        }}
        .header-box {{
            background: linear-gradient(135deg, #0f172a 0%, #1e3a8a 100%);
            color: #ffffff;
            padding: 16px 20px;
            border-radius: 8px;
            margin-bottom: 18px;
        }}
        .header-box h1 {{
            color: #ffffff;
            border-bottom: 2px solid #38bdf8;
            margin: 0 0 8px 0;
            font-size: 15pt;
        }}
        .header-box p {{
            color: #cbd5e1;
            margin: 3px 0;
            font-size: 9pt;
        }}
        .badge {{
            display: inline-block;
            background-color: #059669;
            color: #ffffff;
            padding: 2px 7px;
            border-radius: 4px;
            font-size: 7.5pt;
            font-weight: bold;
            margin-bottom: 6px;
        }}
    </style>
</head>
<body>
    <div class="header-box">
        <span class="badge">{badge}</span>
        <h1>{doc_title}</h1>
        <p style="font-size: 10.5pt; color: #93c5fd; font-weight: bold;">{subtitle}</p>
        <p><strong>PT GOGIGA MEDIA TEKNOLOGI — GOGIGANET FIBER BROADBAND</strong></p>
    </div>
    {html_content}
</body>
</html>
"""

    with open(html_file, "w", encoding="utf-8") as f:
        f.write(full_html)

    edge_path = r"C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe"
    cmd = [
        edge_path,
        "--headless",
        "--disable-gpu",
        "--run-all-compositor-stages-before-draw",
        f"--print-to-pdf={pdf_file}",
        "--no-pdf-header-footer",
        html_file
    ]

    res = subprocess.run(cmd, capture_output=True, text=True)
    if res.returncode == 0 and os.path.exists(pdf_file):
        size_kb = os.path.getsize(pdf_file) / 1024
        print(f"SUCCESS: {pdf_file} ({size_kb:.2f} KB)")
    else:
        print("ERROR:", res.stderr)

if __name__ == "__main__":
    convert_md_to_pdf(
        r"c:\Users\62811\Documents\ISP\MATRIKS_TUGAS_DAN_TANGGUNG_JAWAB_SDM_GOGIGANET.md",
        r"c:\Users\62811\Documents\ISP\MATRIKS_TUGAS_DAN_TANGGUNG_JAWAB_SDM_GOGIGANET.pdf",
        "MATRIKS TUGAS, WEWENANG & TANGGUNG JAWAB (JOB DESCRIPTION)",
        "STRUKTUR ORGANISASI & OPERASIONAL 3 ENGINE TELEKOMUNIKASI",
        "OFFICIAL HR & JOB DESCRIPTION"
    )
