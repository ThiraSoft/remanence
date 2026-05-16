# Rémanence

[![Go Version](https://img.shields.io/badge/Go-1.24.0-00ADD8?logo=golang&logoColor=white)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

> **Ephemeral messaging for people who prefer secrets to disappear.**

Rémanence is a lightweight, self-hosted ephemeral messaging service built for developers, security teams, and privacy-conscious organizations.

Create secure burn-after-read messages in seconds — with zero external dependencies, zero database setup, and a single deployable binary.

---

# 🌫 Why Rémanence?

In a world where everything is archived, indexed, and analyzed, Rémanence chooses to forget.

We believe a message should sometimes exist only for a moment:

- no history,
- no tracking,
- no permanent storage,
- no digital footprint.

Share sensitive information safely, then let it disappear forever.

---

# ✨ Features

## 🔥 Burn-after-read messages

Messages can:

- self-destruct after the first read,
- expire automatically after a chosen delay,
- or both.

Temporary by design.

---

## ⚡ Zero-config deployment

No PostgreSQL.  
No Redis.  
No complex infrastructure.

Start the service with a single command:

```bash
docker run -p 8080:8080 remanence:latest
```

---

## 📦 Single binary architecture

The backend and frontend are embedded into one executable.

Benefits:

- easy deployment,
- tiny operational footprint,
- fast startup,
- low memory usage,
- easy portability.

Run it anywhere:

- VPS,
- Docker,
- Kubernetes,
- bare metal,
- homelab,
- edge infrastructure.

---

## 🎨 User Interface

Rémanence includes a modern and responsive web interface for a seamless user experience.

- **Integrated Delivery**: The UI is served directly by the Go binary, eliminating the need for a separate frontend server or complex build pipeline.
- **Simplified Workflow**: A clean interface allowing users to quickly enter their secret message and generate a secure link.
- **Flexible Configuration**: A dedicated popup allows the fine-tuning of message modalities (expiration time and one-shot settings) before creation.

---


## 🚀 Production ready

Built in Go for:

- high concurrency,
- operational simplicity,
- reliability,
- and speed.

---

# 🚀 Quick Start

## 🛠 Installation & Build

You can run Rémanence in three ways:

### 🐳 Docker (recommended)

```bash
make build
docker run -p 8080:8080 remanence:latest
```

### 🔨 Using Make

```bash
make build
./bin/remanence
```

### 💻 Local development

The web UI styles are compiled with Tailwind CSS into `internal/frontend/web/styles.css`,
which is then embedded into the Go binary. Compile the CSS once before running the server:

```bash
make css                      # npm install + Tailwind build
go mod download
go run cmd/server/main.go
```

> Re-run `make css` whenever you change the HTML in `internal/frontend/web/`.
> Docker builds (`make build`) compile the CSS automatically — no Node.js needed on the host.

Your instance will be available at: `http://localhost:8080`

---

## 🔍 SEO & deployment notes

The frontend ships with Open Graph / Twitter cards, JSON-LD, `robots.txt` and
`sitemap.xml`. These reference the public domain — if you self-host on your own
domain, update the URLs accordingly:

- `<link rel="canonical">` and `og:`/`twitter:` meta tags in
  `internal/frontend/web/index.html` and `about.html`
- `internal/frontend/web/robots.txt` (the `Sitemap:` line)
- `internal/frontend/web/sitemap.xml` (the `<loc>` entries)

Then recompile (`make css` is not required for this, just rebuild the binary).

---

# 🧠 How It Works

Create a confidential message, share the generated link, and let Rémanence handle the rest.

Workflow:

1. The message is stored temporarily
2. A unique access link is generated
3. The message self-destructs:
   - after being read,
   - after expiration,
   - or both

No account required.  
No personal data recorded.

---

# 📡 API

Integrate Rémanence into:

- scripts,
- bots,
- internal tools,
- CI/CD pipelines,
- automation workflows.

---

## Create a message

`POST /api/v1/messages`

### Form parameters

| Parameter   | Description                                     |
| ----------- | ----------------------------------------------- |
| `content`   | Secret message (required, max `MAX_MESSAGE_LENGTH` chars) |
| `lifeLimit` | Lifetime in minutes, `1–1440` (default: `1440`) |
| `isOneShot` | Destroy after first read (default: `true`)      |

### Example

```bash
curl -X POST https://remanence.thirasoft.com/api/v1/messages \
  -d "content=my secret" \
  -d "isOneShot=true" \
  -d "lifeLimit=60"
```

### Response

```json
{
  "id": "aBcDeFgHiJkLmNoP"
}
```

---

## Retrieve a message

`GET /api/v1/messages/{id}`

### Behavior

To prevent ID enumeration, Rémanence uses a **confusion strategy**: if a message is not found or has expired, the system returns a random content string instead of a 404 error.

IDs of **any length** are accepted (only the character set is validated), so an observer cannot infer the expected ID format from the API's behavior.

### Example

```bash
curl https://remanence.thirasoft.com/api/v1/messages/aBcDeFgHiJkLmNoP
```

### Response

```json
{
  "id": "aBcDeFgHiJkLmNoP",
  "content": "my secret"
}
```

---

# 🔐 Security

## End-to-end encryption

Enabled by default. The web interface encrypts every message **in the browser**
before it is sent — the server (and any proxy or CDN in front of it, such as
Cloudflare) only ever sees and stores ciphertext.

- **Cipher:** AES-256-GCM via the native Web Crypto API — no external library.
- **Envelope:** the stored `content` is `base64( IV[12 bytes] || ciphertext+tag )`.
- **Key handling:** a fresh random key is generated per message and placed in
  the URL **fragment** (`...?message=<id>#<key>`). The fragment is never sent
  to the server, so the key stays client-side.
- **Reading:** the recipient's browser reads the key from the `#`, fetches the
  ciphertext and decrypts locally. When decryption fails — message never
  existed, expired, already read, or wrong key — the UI shows a random
  decoy string, so an observer cannot tell a dead link from a live one.

Disable it with `ENCRYPTION_ENABLED=false` to store plaintext (e.g. for a
trusted internal deployment). The web UI reads this setting from
`/api/v1/about` and adapts automatically.

> **API note:** the server is content-agnostic. To produce messages readable
> in the web UI, a client must reproduce the envelope above and put the key in
> the link's `#`. A raw `curl` posting plaintext while encryption is on will be
> stored as-is but will fail to decrypt in the browser. A reference CLI for the
> envelope format is planned in a follow-up.

## Rate limiting

Default:

- `20 requests/minute/IP`

Helps mitigate:

- brute-force attempts,
- endpoint abuse,
- denial-of-service patterns.

---

## Hardened HTTP stack

Includes protections against:

- Slowloris attacks,
- malicious payload flooding,
- protocol abuse.

Configured with strict:

- `ReadTimeout`
- `WriteTimeout`

---

## Minimal container surface

Production images:

- use multi-stage builds,
- run as non-root,
- contain only the compiled application.

Smaller image. Smaller attack surface.

---

# ⚙️ Configuration

Rémanence follows 12-factor app principles.

| Variable      | Default | Description                             |
| ------------- | ------- | --------------------------------------- |
| `PORT`        | `8080`  | HTTP listening port (ex: `-e PORT=9000 -p 9000:9000`) |
| `LOG_LEVEL`   | `ERROR` | `DEBUG`, `INFO`, `WARN`, `ERROR`        |
| `TRUST_PROXY` | `false` | Enable behind Nginx/Caddy/reverse proxy |
| `MESSAGE_ID_LENGTH` | `16` | Length of *generated* message IDs (tune for local vs production entropy) |
| `MAX_MESSAGE_LENGTH` | `2048` | Maximum stored message length, in characters (the encrypted payload — see below) |
| `ENCRYPTION_ENABLED` | `true` | End-to-end encryption. Set to `false` to store plaintext instead |

---

# 🔒 Privacy

Rémanence does not:

- track your activity,
- require registration,
- use advertising cookies,
- build user profiles.

Your data remains under your control — or disappears entirely.

---

# 🎯 Use Cases

- Secure credential sharing
- DevOps & CI/CD workflows
- One-time secret delivery
- Temporary support links
- Incident response operations
- Internal tooling
- Privacy-first communication

---

# 🤝 Join the Project

Rémanence is an independent open-source project.

Contributions, ideas, security reviews, and improvements are welcome.

If you like the project:

- ⭐ star the repository
- 🛠 contribute
- 📢 share it
- 🔐 use it responsibly

---

# 📄 License

MIT — free for personal and commercial use.

---

# 🌫 Why “Rémanence”?

In physics and memory science, _remanence_ describes the trace left behind after the original signal disappears.

Rémanence does the opposite.

Your message disappears too.
