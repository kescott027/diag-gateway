# SECURITY.md

# Security Model

## 1. Threat Model

Assumptions:

* Private network deployment
* Potential compromised test machine
* Network eavesdropping possible

Threats mitigated:

* MITM attacks
* Unauthorized agent registration
* Replay attacks
* Credential theft persistence

---

## 2. Enrollment Flow

1. Operator generates enrollment token.
2. Agent started with token.
3. Agent authenticates over TLS.
4. Collector validates token.
5. Collector issues:

   * client certificate OR
   * long-lived credential
6. Token invalidated.

Tokens:

* Single use
* TTL-bound

---

## 3. Transport Security

* TLS 1.2+ required
* Mutual TLS after enrollment
* Strong cipher suites only
* Certificate rotation supported

---

## 4. Credential Storage

Agent:

* OS keychain/secure store if available
* Otherwise file with restricted permissions

Collector:

* CA private key stored in protected directory
* File permissions enforced

---

## 5. Revocation

Collector supports:

* Immediate revocation list
* Expired cert rejection
* Source disable flag

---

## 6. UI Security

* UI bound to localhost by default
* Optional auth layer
* CSRF protection required if exposed externally

---

## 7. Logging & Audit

Collector logs:

* Enrollment events
* Credential issuance
* Revocations
* Config changes

Audit logs immutable (append-only).

---

## 8. Secure Defaults

* HTTPS required
* No plaintext fallback
* No unauthenticated ingestion endpoints

---

End Security Document
