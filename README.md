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
docker run -p 8008:8008 remanence:latest
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
docker run -p 8008:8008 remanence:latest
```

### 🔨 Using Make

```bash
make build
./bin/remanence
```

### 💻 Local development

```bash
go mod download
go run cmd/server/main.go
```

Your instance will be available at: `http://localhost:8008`

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
| `content`   | Secret message (required, max 1024 chars)       |
| `lifeLimit` | Lifetime in minutes, `1–1440` (default: `1440`) |
| `isOneShot` | Destroy after first read (default: `true`)      |

### Example

```bash
curl -X POST https://remanence.app/api/v1/messages \
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

### Example

```bash
curl https://remanence.app/api/v1/messages/aBcDeFgHiJkLmNoP
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
| `PORT`        | `8008`  | HTTP listening port                     |
| `LOG_LEVEL`   | `ERROR` | `DEBUG`, `INFO`, `WARN`, `ERROR`        |
| `TRUST_PROXY` | `false` | Enable behind Nginx/Caddy/reverse proxy |

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
