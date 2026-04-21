# AD IP Certificate Rollout

This project can run with an internal Active Directory certificate chain instead of a browser-approved one-off self-signed certificate.

The target setup is:

- clients open the system by IP over `https://...`
- the backend serves a server certificate signed by your internal CA
- domain computers trust the CA root through GPO
- browsers stop prompting after policy is applied

## What To Build

Use this chain:

1. Internal Microsoft CA (`AD CS`)
2. Server certificate for the exact IP used by clients
3. CA root distributed to domain clients through Group Policy

Do not use a plain self-signed server certificate for this scenario. Browsers will keep warning unless the issuing root is trusted.

## AD CS Template

Create or duplicate a template from `Web Server`.

Recommended template settings:

- `General`:
  - clear name like `HelpDeskWebIP`
- `Request Handling`:
  - allow private key export only if your security policy permits it
- `Subject Name`:
  - `Supply in the request`
- `Extensions`:
  - `Server Authentication`
- `Security`:
  - allow enrollment for the account that will submit the CSR

Issue the template on the CA after saving it.

## Generate Key And CSR

From the backend host or the admin workstation:

```powershell
powershell -ExecutionPolicy Bypass -File .\tools\certs\New-AdIpServerCsr.ps1 `
  -PrimaryIP 10.10.10.20 `
  -AdditionalIPs 127.0.0.1 `
  -CertificateTemplate HelpDeskWebIP
```

This creates:

- `server.key`
- `server.csr`
- `openssl-ip-san.cnf`

Default output directory:

- `.\certs\ad`

The generated CSR includes the IP addresses in `SAN`, which is the important part for browser validation.

## Submit To AD CS

If you use `certreq`:

```powershell
certreq -submit -attrib "CertificateTemplate:HelpDeskWebIP" .\certs\ad\server.csr .\certs\ad\server.crt
```

If multiple CAs exist, use the CA config form:

```powershell
certreq -submit -config "CA-SERVER\CA-NAME" -attrib "CertificateTemplate:HelpDeskWebIP" .\certs\ad\server.csr .\certs\ad\server.crt
```

If you use the AD CS web enrollment page, export the issued certificate as:

- `Base-64 encoded X.509 (.CER)`

Then save it as:

- `.\certs\ad\server.crt`

The backend already expects PEM-style file paths, so `server.key` and `server.crt` are enough.

## Backend Configuration

Point the backend to the issued certificate and the generated key:

```env
SSL_CERT_PATH=./certs/ad/server.crt
SSL_KEY_PATH=./certs/ad/server.key
SERVER_BASE_URL=https://10.10.10.20:8091
```

Then restart the backend.

## GPO Trust Distribution

Distribute the internal CA root certificate to domain clients:

1. Open `Group Policy Management`
2. Edit the target GPO
3. Go to:
   `Computer Configuration -> Policies -> Windows Settings -> Security Settings -> Public Key Policies -> Trusted Root Certification Authorities`
4. Import the root CA certificate
5. Apply policy to client machines

After policy refresh, supported browsers on domain clients should trust certificates issued by that CA.

## Validation

Validate from a domain client:

1. Run `gpupdate /force`
2. Open `https://<your-ip>:<port>/ping`
3. Verify the browser shows a trusted connection
4. Check that the certificate SAN contains the exact IP address being used

## Important Limits With IP Certificates

- If the IP changes, the certificate must be reissued
- Users must open the same IP that exists in the certificate SAN
- If you later move to DNS, issue a new certificate with DNS SAN entries

## Project Notes

- The current `make cert-gen` flow is for local development only
- Production or domain rollout should use the AD-issued certificate path above
- Certificate and key files are ignored by git and must not be committed
