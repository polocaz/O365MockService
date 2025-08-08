# PowerShell script to sign the O365MockService executable
# This helps reduce Windows Defender false positives

param(
    [string]$ExecutablePath = ".\main.exe",
    [string]$CertificateSubject = "CN=O365MockService"
)

Write-Host "🔐 Code Signing Script for O365MockService" -ForegroundColor Cyan
Write-Host "=========================================" -ForegroundColor Cyan

# Check if certificate exists
$cert = Get-ChildItem -Path Cert:\CurrentUser\My | Where-Object {$_.Subject -like "*$CertificateSubject*"}

if (-not $cert) {
    Write-Host "❌ Certificate not found. Creating a new self-signed certificate..." -ForegroundColor Yellow
    
    # Create self-signed certificate
    $cert = New-SelfSignedCertificate -Type CodeSigning -Subject $CertificateSubject -KeyUsage DigitalSignature -FriendlyName "O365MockService Code Signing" -CertStoreLocation Cert:\CurrentUser\My -TextExtension @("2.5.29.37={text}1.3.6.1.5.5.7.3.3") -KeyLength 2048
    
    Write-Host "✅ Created new certificate with thumbprint: $($cert.Thumbprint)" -ForegroundColor Green
}
else {
    Write-Host "✅ Found existing certificate with thumbprint: $($cert.Thumbprint)" -ForegroundColor Green
}

# Check if executable exists
if (-not (Test-Path $ExecutablePath)) {
    Write-Host "❌ Executable not found: $ExecutablePath" -ForegroundColor Red
    Write-Host "💡 Building executable first..." -ForegroundColor Yellow
    
    # Build the executable
    go build -o $ExecutablePath main.go
    
    if (-not (Test-Path $ExecutablePath)) {
        Write-Host "❌ Failed to build executable" -ForegroundColor Red
        exit 1
    }
    
    Write-Host "✅ Built executable: $ExecutablePath" -ForegroundColor Green
}

# Sign the executable
Write-Host "🔏 Signing executable: $ExecutablePath" -ForegroundColor Yellow

try {
    $signResult = Set-AuthenticodeSignature -FilePath $ExecutablePath -Certificate $cert
    
    if ($signResult.Status -eq "Valid" -or $signResult.Status -eq "UnknownError") {
        Write-Host "✅ Successfully signed executable!" -ForegroundColor Green
        Write-Host "📋 Signature Status: $($signResult.Status)" -ForegroundColor Cyan
        Write-Host "🔑 Certificate: $($signResult.SignerCertificate.Subject)" -ForegroundColor Cyan
        
        # Verify signature
        $verification = Get-AuthenticodeSignature -FilePath $ExecutablePath
        Write-Host "🔍 Verification Status: $($verification.Status)" -ForegroundColor Cyan
        
        Write-Host "" -ForegroundColor White
        Write-Host "📝 Notes:" -ForegroundColor Yellow
        Write-Host "  • Self-signed certificates may still show warnings" -ForegroundColor Gray
        Write-Host "  • For production, consider purchasing a commercial certificate" -ForegroundColor Gray
        Write-Host "  • This reduces but may not eliminate all antivirus false positives" -ForegroundColor Gray
        
    }
    else {
        Write-Host "⚠️ Signing completed with status: $($signResult.Status)" -ForegroundColor Yellow
    }
}
catch {
    Write-Host "❌ Failed to sign executable: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

Write-Host "" -ForegroundColor White
Write-Host "🎯 Next steps to reduce false positives:" -ForegroundColor Cyan
Write-Host "  1. Submit your signed executable to antivirus vendors for whitelisting" -ForegroundColor White
Write-Host "  2. Consider purchasing a commercial code signing certificate" -ForegroundColor White
Write-Host "  3. Build reputation by distributing signed executables over time" -ForegroundColor White
