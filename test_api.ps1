Write-Host "--- 1. Testing Health Endpoint ---" -ForegroundColor Cyan
$health = Invoke-RestMethod -Uri "http://localhost:8081/health" -Method Get
$health | ConvertTo-Json

Write-Host "`n--- 2. Testing ODP Map List ---" -ForegroundColor Cyan
$odps = Invoke-RestMethod -Uri "http://localhost:8081/api/v1/public/odps" -Method Get
$odps | ConvertTo-Json -Depth 3

Write-Host "`n--- 3. Testing Coverage Check (Within 250m Golden Net) ---" -ForegroundColor Cyan
$covBody = @{
    latitude = -0.2849
    longitude = 100.4303
} | ConvertTo-Json

$coverage = Invoke-RestMethod -Uri "http://localhost:8081/api/v1/public/coverage-check" -Method Post -ContentType "application/json" -Body $covBody
$coverage | ConvertTo-Json -Depth 3

Write-Host "`n--- 4. Testing Public Registration ---" -ForegroundColor Cyan
$regBody = @{
    full_name = "Budi Hartono"
    email = "budi@example.com"
    phone = "081288887777"
    id_card_number = "3216091234560001"
    address = "Jl. Raya Bukittinggi - Payakumbuh, Biaro"
    latitude = -0.2849
    longitude = 100.4303
    selected_plan_id = "plan-home-50m"
    selected_plan_name = "Giga Home 50 Mbps"
} | ConvertTo-Json

$reg = Invoke-RestMethod -Uri "http://localhost:8081/api/v1/public/register" -Method Post -ContentType "application/json" -Body $regBody
$reg | ConvertTo-Json -Depth 3

$regNo = $reg.data.registration_no

Write-Host "`n--- 5. Testing Tracking Registration Status ($regNo) ---" -ForegroundColor Cyan
$track = Invoke-RestMethod -Uri "http://localhost:8081/api/v1/public/track/$regNo" -Method Get
$track | ConvertTo-Json -Depth 3

Write-Host "`n--- ALL API TESTS COMPLETED SUCCESSFULLY! ---" -ForegroundColor Green
