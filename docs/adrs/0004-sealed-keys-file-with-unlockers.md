# ADR 0004 — Sealed keys file with pluggable unlockers

- Status: accepted
- Date: 2026-11-06

## Context

The owner wants the keys under `~/.config/ataecina/` in a JSON file. A
plain `0600` file stops other users but not any process running as the owner
— including coding agents that read files in the home directory and can be
prompt-injected. The desktop Secret Service (KWallet/ksecretd on Plasma,
GNOME Keyring elsewhere) is unlocked at login, but it is not reachable over
SSH or on a text console. On the reference machine the
`org.freedesktop.secrets` name is owned by GNOME Keyring even under Plasma,
so the design must not assume KWallet.

## Decision

- `~/.config/ataecina/keys.json` (`0600`, directory `0700`) stores every
  provider key **sealed** with AES-256-GCM under a random 32-byte master key,
  with `ataecina:v1:<provider>` as associated data.
- The master key is never stored in clear. It is wrapped by one or more
  **unlockers**:
  - `secret-service` — an item in the session's Secret Service (standard
    D-Bus API, so KWallet, ksecretd, GNOME Keyring or KeePassXC all work);
  - `passphrase` — PBKDF2-HMAC-SHA256 from the standard library
    (≥ 600 000 iterations, random salt), prompted without echo;
  - `systemd-creds` — `systemd-creds encrypt --user`, bound to the TPM2 when
    present, delivered to the collector unit with `LoadCredentialEncrypted=`.
- Keys enter only through stdin (`ataecina keys set <provider>`). Ataecina
  refuses to run if the directory or files are readable by others or owned
  by another user.

## Consequences

- Reading `keys.json` alone reveals nothing; an attacker also needs the
  unlocked keyring, the passphrase or the TPM-bound credential.
- A process running as the owner inside the unlocked session can still ask
  the Secret Service; this is the desktop's trust model and is documented,
  not hidden.
- From a terminal inside the desktop session, the CLI reaches the Secret
  Service over the session D-Bus with no window; over SSH it uses the
  passphrase.

## Alternatives

- Keys directly in the Secret Service, no file: fails the "keys in
  `~/.config/ataecina`" requirement and leaves SSH sessions without keys.
- Plain JSON `0600` (like many CLIs): readable by any process of the owner.
  Rejected.
- `age` or Argon2id: better KDF, but adds `golang.org/x/crypto`; PBKDF2 in the
  standard library is adequate with a high iteration count (ADR 0009).
