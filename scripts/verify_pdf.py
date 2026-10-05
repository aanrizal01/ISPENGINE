import pypdf

for fname in ['web/FORM_SCORECARD_EVALUASI_KPI_BULANAN.pdf', 'web/FORM_SCORECARD_EVALUASI_KPI_BULANAN_DEV.pdf']:
    print('Checking:', fname)
    r = pypdf.PdfReader(fname)
    for i, p in enumerate(r.pages):
        print(f'--- PAGE {i+1} ---')
        for line in p.extract_text().splitlines()[:10]:
            print('  ', line.encode('ascii', errors='replace').decode('ascii'))
