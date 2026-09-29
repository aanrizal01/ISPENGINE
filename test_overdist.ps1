$bodyFar = @{
    full_name = "Calon Pelanggan Survei Khusus"
    phone = "081255554444"
    id_card_number = "3216091234567899"
    address = "Pinggiran Kota Payakumbuh > 250m"
    latitude = -0.2500
    longitude = 100.6500
    selected_plan_id = "plan-home-20m"
    selected_plan_name = "Giga Home 20 Mbps"
} | ConvertTo-Json

$res = Invoke-RestMethod -Uri 'http://localhost:8081/api/v1/public/register' -Method Post -ContentType 'application/json' -Body $bodyFar

Write-Host "Status Registrasi:" $res.data.status -ForegroundColor Green
Write-Host "No Registrasi:" $res.data.registration_no -ForegroundColor Cyan
Write-Host "Jarak ke ODP:" $res.data.distance_to_odp_meters "meter" -ForegroundColor Cyan
