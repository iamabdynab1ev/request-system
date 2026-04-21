param(
    [Parameter(Mandatory = $true)]
    [string]$PrimaryIP,

    [string[]]$AdditionalIPs = @(),

    [string]$OutputDir = ".\certs\ad",

    [string]$CommonName = "",

    [string]$Country = "TJ",

    [string]$Organization = "Bank HelpDesk",

    [string]$OrganizationalUnit = "HelpDesk",

    [string]$KeyFileName = "server.key",

    [string]$CsrFileName = "server.csr",

    [string]$ConfigFileName = "openssl-ip-san.cnf",

    [string]$CertificateTemplate = "HelpDeskWebIP",

    [switch]$Force
)

$ErrorActionPreference = "Stop"

function Require-Command {
    param([Parameter(Mandatory = $true)][string]$Name)

    $command = Get-Command $Name -ErrorAction SilentlyContinue
    if (-not $command) {
        throw "Required command '$Name' was not found in PATH."
    }

    return $command
}

function Resolve-OutputPath {
    param([Parameter(Mandatory = $true)][string]$PathValue)

    if ([System.IO.Path]::IsPathRooted($PathValue)) {
        return [System.IO.Path]::GetFullPath($PathValue)
    }

    return [System.IO.Path]::GetFullPath((Join-Path (Get-Location) $PathValue))
}

function Get-UniqueIPs {
    param(
        [Parameter(Mandatory = $true)][string]$FirstIP,
        [string[]]$OtherIPs = @()
    )

    $result = New-Object System.Collections.Generic.List[string]
    foreach ($ip in @($FirstIP) + $OtherIPs) {
        if ([string]::IsNullOrWhiteSpace($ip)) {
            continue
        }

        $null = [System.Net.IPAddress]::Parse($ip)
        if (-not $result.Contains($ip)) {
            $result.Add($ip)
        }
    }

    return $result
}

$openssl = Require-Command -Name "openssl"
$ips = Get-UniqueIPs -FirstIP $PrimaryIP -OtherIPs $AdditionalIPs

if ($ips.Count -eq 0) {
    throw "At least one valid IP address is required."
}

if ([string]::IsNullOrWhiteSpace($CommonName)) {
    $CommonName = $ips[0]
}

$resolvedOutputDir = Resolve-OutputPath -PathValue $OutputDir
New-Item -ItemType Directory -Path $resolvedOutputDir -Force | Out-Null

$keyPath = Join-Path $resolvedOutputDir $KeyFileName
$csrPath = Join-Path $resolvedOutputDir $CsrFileName
$configPath = Join-Path $resolvedOutputDir $ConfigFileName
$certPath = Join-Path $resolvedOutputDir "server.crt"

if (-not $Force) {
    foreach ($path in @($keyPath, $csrPath, $configPath)) {
        if (Test-Path -LiteralPath $path) {
            throw "File already exists: $path. Use -Force to overwrite."
        }
    }
}

$altNames = for ($i = 0; $i -lt $ips.Count; $i++) {
    "IP.$($i + 1) = $($ips[$i])"
}

$opensslConfig = @"
[ req ]
default_bits = 2048
prompt = no
default_md = sha256
distinguished_name = dn
req_extensions = req_ext

[ dn ]
C = $Country
O = $Organization
OU = $OrganizationalUnit
CN = $CommonName

[ req_ext ]
subjectAltName = @alt_names
keyUsage = critical, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth

[ alt_names ]
$($altNames -join "`r`n")
"@

Set-Content -LiteralPath $configPath -Value $opensslConfig -Encoding ascii

& $openssl.Source genrsa -out $keyPath 2048
if ($LASTEXITCODE -ne 0) {
    throw "OpenSSL failed while generating the private key."
}

& $openssl.Source req -new -key $keyPath -out $csrPath -config $configPath
if ($LASTEXITCODE -ne 0) {
    throw "OpenSSL failed while generating the CSR."
}

Write-Host ""
Write-Host "CSR generated successfully." -ForegroundColor Green
Write-Host "Output directory: $resolvedOutputDir"
Write-Host "Private key     : $keyPath"
Write-Host "CSR             : $csrPath"
Write-Host "OpenSSL config  : $configPath"
Write-Host ""
Write-Host "Next steps:"
Write-Host "1. Submit the CSR to AD CS with a template that allows 'Supply in the request'."
Write-Host "2. Save the issued certificate as Base-64 X.509 PEM at: $certPath"
Write-Host "3. Distribute the CA root certificate to domain clients via GPO."
Write-Host "4. Point backend env vars to the issued cert and generated key."
Write-Host ""
Write-Host "Example certreq command:"
Write-Host "certreq -submit -attrib `"CertificateTemplate:$CertificateTemplate`" `"$csrPath`" `"$certPath`""
Write-Host ""
Write-Host "Example backend env values:"
Write-Host "SSL_CERT_PATH=$certPath"
Write-Host "SSL_KEY_PATH=$keyPath"
