Write-Host "--- TEST 1: Registrasi Jarak Jauh (> 250m) Harus Ditolak ---" -ForegroundColor Yellow
try {
    $bodyFar = @{
        full_name = "Pelanggan Jauh"
        phone = "081299991111"
        id_card_number = "3216091234567890"
        address = "Lokasi Jauh > 250m"
        latitude = -0.2500
        longitude = 100.6500
        selected_plan_id = "plan-home-20m"
        selected_plan_name = "Giga Home 20 Mbps"
    } | ConvertTo-Json

    $res = Invoke-RestMethod -Uri "http://localhost:8081/api/v1/public/register" -Method Post -ContentType "application/json" -Body $bodyFar
    Write-Host "Unexpected Success!" -ForegroundColor Red
} catch {
    $resp = $_.ErrorDetails.Message
    Write-Host "Hasil: Berhasil Ditolak!" -ForegroundColor Green
    Write-Host "Pesan Error dari Server:" $resp -ForegroundColor Cyan
}

Write-Host "`n--- TEST 2: Registrasi Jarak Dekat (<= 250m dari ODP Payakumbuh) Harus Diterima ---" -ForegroundColor Yellow
# ODP-PYK-0237 ada di lat: -0.235595, lng: 100.620797
$bodyNear = @{
    full_name = "Warga Payakumbuh Dekat ODP"
    phone = "081288889999"
    id_card_number = "3216091234567891"
    address = "Jl. Sudirman Payakumbuh dekat tiang"
    latitude = -0.235600
    longitude = 100.620800
    selected_plan_id = "plan-home-50m"
    selected_plan_name = "Giga Home 50 Mbps"
} | ConvertTo-Json

$resNear = Invoke-RestMethod -Uri "http://localhost:8081/api/v1/public/register" -Method Post -ContentType "application/json" -Body $bodyNear
Write-Host "Hasil: Berhasil Diterima!" -ForegroundColor Green
Write-Host "No Registrasi:" $resNear.data.registration_no -ForegroundColor Cyan
Write-Host "Jarak ke ODP:" $resNear.data.distance_to_odp_meters "meter" -ForegroundColor Cyan
Write-Host "ODP:" $resNear.data.nearest_odp_code -ForegroundColor Cyan
