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
* Versioned CRL snapshots with canonical serial normalization
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

Audit logs immutable (append-only) and support bounded filter queries for forensic review.

---

## 8. Secure Defaults

* HTTPS required
* No plaintext fallback
* No unauthenticated ingestion endpoints

---

## 9. Control-Plane RBAC Baseline

Roles:

* `admin`
* `operator`
* `viewer`

Minimum authorization model:

* deny by default
* explicit action-based grants
* unknown role/action rejected

Baseline intent:

* `admin`: full control-plane access including RBAC policy administration
* `operator`: operational write access (retention/config/enrollment/revocation/routing) without RBAC administration
* `viewer`: read-only diagnostics visibility (dashboard/search/live-tail/artifact/audit views)

---

## 10. Optional OIDC Authentication

OIDC integration is optional and disabled by default.

When enabled:

* Issuer and audience must be explicitly configured.
* Provider metadata issuer and JWKS URL must validate as HTTPS endpoints.
* Token claims must pass issuer/audience/time validation before identity use.
* External role claims are mapped to local RBAC roles; unknown roles are ignored.

Failure handling:

* Misconfigured OIDC settings fail closed.
* Invalid or expired tokens are rejected.
* If OIDC is disabled, local/default auth mechanisms remain in effect.

---

End Security Document
