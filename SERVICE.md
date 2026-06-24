# Future Private Fabric Service

Status: design note only. No server, network client, authentication, account,
or encryption implementation is part of Local V1.

## Purpose

The service is an optional end-to-end encrypted relay for Fabric protocol
events. It exists to synchronize opaque repository memory across machines,
teams, and providers when Git-common local sharing is not enough.

It must preserve the Local V1 event envelope and semantics. Local Git-backed
operation remains complete and first-class with no account or network.

## Non-goals

The service is not:

- A source-code host, index, search engine, or analysis service.
- A prompt or transcript store.
- An LLM gateway or provider proxy.
- The authority for matching, projections, trust, or explanation.
- Required for Fabric to operate.
- A place to escrow repository credentials or plaintext encryption keys.

## Privacy Promise

The service must never be able to read:

- Source code, patches, diffs, or file contents.
- Plaintext direction, rationale, evidence, or relations.
- Prompts, model responses, or transcripts.
- Repository credentials, provider tokens, or Git credentials.
- Decrypted projections or explanations.

Payloads are encrypted on a client and decrypted only on authorized clients.
Matching, projection, conflict materialization, and explanation remain
client-side.

The service may observe only the metadata required to route encrypted blobs:

- Accounts and authenticated devices.
- Opaque repository, thread, event, and blob identifiers.
- Repository membership and device public keys.
- Blob timestamps, sizes, and delivery receipts.
- Network metadata necessarily visible to the deployment.

This leakage must be documented in product UI and deployment guidance. Claims
such as "zero knowledge" must not hide traffic-analysis limits.

## Threat Model

The design should protect plaintext protocol data against:

- A curious or compromised service operator.
- Database snapshots and storage-provider compromise.
- Accidental server logging or analytics capture.
- One repository member attempting to read another repository.
- A revoked device attempting to read events created after revocation.

The first service version does not promise protection against:

- A compromised authorized client after decryption.
- A malicious repository member who can already read repository keys.
- Endpoint screenshots, copied plaintext, or model-provider retention.
- Traffic analysis based on timing, membership, IP address, and blob size.
- Rollback or withholding by the relay unless clients verify local event history.

Clients should retain the immutable local ledger as the audit and recovery
boundary. The service is a relay, not the sole source of truth.

## Cryptographic Shape

Each repository receives a random client-generated repository data key. The
plaintext key never reaches the service.

For every authorized device:

1. The device has a public/private encryption key pair.
2. A client wraps the repository key to that device's public key.
3. The service stores the device-specific wrapped key.
4. The device unwraps it locally and decrypts repository blobs.

Protocol envelopes and payloads are encrypted together, or identifiers visible
to the relay are replaced with separate opaque routing identifiers. The exact
construction must use an audited modern library and authenticated encryption;
it must not invent custom cryptography.

Associated data should bind ciphertext to the opaque repository, protocol
version, blob ID, and key epoch so ciphertext cannot be moved silently between
contexts.

## Authentication and Devices

Accounts authenticate access to encrypted blobs and membership metadata, not
access to plaintext. Authentication options may include passkeys, enterprise
identity, or scoped service tokens.

Each device has a separate key and identity. Device enrollment requires approval
from an already authorized device or an explicit repository administrator flow.
Recovery mechanisms must state clearly whether they weaken the privacy promise.

The service must not accept GitHub, GitLab, model-provider, or repository
credentials as a shortcut for decryption.

## Repository Membership

Membership grants access to wrapped repository keys and encrypted blobs for an
opaque repository ID. Roles may control inviting devices, rotating keys,
retention policy, and deletion.

The server can enforce metadata-level authorization but cannot determine
whether plaintext direction is appropriate. Clients and Git review remain the
semantic trust boundary.

Member removal stops future blob delivery and triggers key rotation. It cannot
revoke plaintext or keys already copied to an authorized device.

## Key Rotation

Repository keys use explicit epochs. Rotation occurs when:

- A member or device is revoked.
- A key is suspected compromised.
- Policy requires periodic rotation.
- Repository ownership moves between deployments.

New events use the newest epoch. Historical re-encryption may be optional
because it is expensive and cannot erase copies already decrypted. Clients must
retain enough wrapped historical keys to read retained history.

Rotation operations themselves should be signed or otherwise attributable to
authorized devices once cryptographic identity is introduced.

## Retention

Default retention should be explicit and repository-scoped. Options may include
keep-until-deleted, bounded age, or bounded storage. The service must not inspect
plaintext to apply content-based retention.

Delivery receipts may have a shorter retention period than event blobs. Server
logs should be minimized, scrub opaque IDs where possible, and have a documented
short lifetime.

Local clients remain responsible for durable Git history and backup.

## Deletion

Deletion has three distinct meanings:

1. Remove a blob from active relay storage.
2. Remove repository membership and wrapped keys.
3. Delete account and metadata held by the service.

The API and UI must distinguish them. Deletion cannot retract committed Git
events, client exports, backups, or plaintext already decrypted on devices.

Cryptographic erasure may make retained ciphertext inaccessible by deleting all
wrapped key material, subject to backup policy. The service must document any
delay or immutable backup window.

## Export

An authorized client can export decrypted Local V1 event files and runtime
receipts into the normal repository layout. Export is performed client-side.
The server may provide only opaque ciphertext archives.

Export must preserve event bytes or canonical event meaning, IDs, extensions,
and parent relations so a repository can leave the service without losing
provenance.

## Self-hosting and Packaging

The future service should ship as a single binary and container image with a
small external database/object-store contract. A private deployment should be
reasonable for an individual, team, or enterprise network.

Configuration should support:

- Base URL and trusted origin.
- Database and blob storage.
- Identity provider or local passkey mode.
- Retention and log policy.
- Backup and restore.
- Metrics that never include payload plaintext.

Self-hosting must use the same protocol and export format as a hosted deployment.
No proprietary server-only event semantics.

## Migration Path

The local public interfaces are designed so a remote encrypted transport can be
added without changing protocol meaning:

- `protocol.EventStore` handles immutable repository events.
- `protocol.RuntimeStore` handles thread, projection, and receipt events.
- Event envelopes are transport-neutral.
- Opaque node references avoid a server dependency on provider content.

A future client can layer remote synchronization beside local storage:

1. Create or choose an opaque repository ID.
2. Generate a repository key locally.
3. Wrap it to authorized devices.
4. Encrypt existing immutable events and upload opaque blobs.
5. Continue writing local Git state as configured.
6. Reconcile by event ID and report divergent immutable content as a conflict.

Service adoption must never convert a repository into a service-only format.

## Deferred Decisions

Before implementation, write a separate cryptographic and operational spec for:

- Exact authenticated-encryption and key-wrapping algorithms.
- Device enrollment, recovery, signing, and revocation UX.
- Opaque routing-ID derivation and metadata minimization.
- Replay, rollback, withholding, and fork detection.
- Multi-device concurrent writes and quota behavior.
- Legal deletion, backup windows, abuse handling, and audit logs.
- Hosted billing and enterprise identity.

Until those choices are reviewed, Fabric should implement no partial network or
encryption feature that could create a misleading privacy promise.
