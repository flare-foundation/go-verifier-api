 # Google Confidential Space Root Certificate

 This certificate is embedded for JWT attestation token verification.

 Source: [https:confidentialcomputing.googleapis.com/.well-known/attestation-pki-root]()

 **IMPORTANT**: Root certificates rarely change, but if Google updates the root
 certificate you MUST update the embedded PEM in:
   *assets/google_confidential_space_root.crt*

 Update process:
1. Download the latest root certificate from Google’s well-known endpoint:
```bash
curl -o assets/google_confidential_space_root.crt \
https:confidentialcomputing.googleapis.com/.well-known/attestation-pki-root
```
2. Verify fingerprint of the downloaded certificate against Google’s published fingerprint (check documentation or announcements).

3. Commit the new certificate and re-run all tests to ensure attestation validation still works.


## Validity of the Certificate 
Validity can be checked via the following command.

**IMPORTANT**: This only shows the validity period and does **not** check whether the certificate has been revoked.
```bash
openssl x509 -noout -startdate -enddate -in assets/google_confidential_space_root.crt
```