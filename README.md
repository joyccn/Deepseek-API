# Deepseek-API

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go)](https://go.dev)
[![Build Status](https://img.shields.io/badge/build-passing-brightgreen.svg)]()
[![OpenAI & Anthropic Compatible](https://img.shields.io/badge/API-OpenAI%20%7C%20Anthropic-orange.svg)]()

High-performance, 100% native **Golang** OpenAI & Anthropic compatible API relay server (`http://localhost:8000`) powered by personal DeepSeek Web accounts.

Designed specifically to serve as a fast, reliable drop-in backend provider for **Agentic Coding CLIs** (e.g., Claude Code, OpenCode, OpenAI Codex CLI, Cursor, Aider, Continue, etc.) with native support for **Anthropic Messages API**, **OpenAI Chat Completions**, **DeepThink R1 Reasoning**, **Live Web Search & Citations**, and **Multi-Turn Agent Trajectory Tool Calling**.

---

## ⚡ Features

- **Pure Go WASM PoW Solver (`wazero`)** — Zero CGo dependencies. Solves DeepSeek `sha3_wasm_bg.wasm` challenges natively in ~0.16s.
- **Dual API Compatibility (Anthropic + OpenAI)** — Dual endpoint support for both Anthropic Messages API (`POST /v1/messages`) and OpenAI Chat (`POST /v1/chat/completions`).
- **Claude Code Integration** — Native support for Claude Code CLI and Anthropic SDKs via `/v1/messages` with `thinking` blocks (`budget_tokens`), `tool_use` JSON objects, and `tool_result` history.
- **OpenAI Codex CLI Compatibility** — Native discovery schema for `GET /v1/models` including reasoning levels, slug, and context windows.
- **Auto Token Exchange & Self-Healing** — Accepts raw `userToken` from LocalStorage, automatically exchanges it for short-lived `accessToken` via `/api/v0/users/current`, and auto-refreshes on expiration.
- **Clean Reasoning Output Parser** — Separates `THINK` and `RESPONSE` stream fragments dynamically. `reasoning_content` & `thinking` are delivered in standard delta formats without corrupting main answer text.
- **Live Web Search & Structured Citations** — Supports `deepseek-search` and `deepseek-reasoner-search` with automatic conversion of `[citation:X]` tags and structured markdown footnote citations.
- **Multi-Format Tool Calling & Trajectory Replay** — Parses `<tool>...</tool>`, `<tool:name>`, `<tool_call>`, and `<parameter>` shapes, while reconstructing multi-turn assistant tool calls and `tool_result` outputs with anchoring instructions to prevent agent amnesia and duplicate calls.
- **Ephemeral Session Auto-Cleanup** — Temporary one-off sessions are automatically deleted from DeepSeek's backend via `/api/v0/chat_session/delete`, keeping your web dashboard clean.
- **CORS & Web Frontend Ready** — Built-in CORS middleware and preflight handlers for browser web interfaces (Open WebUI, Chatbox, etc.).

---

## 🌐 Supported Models & Endpoints

### Endpoints
| Endpoint | Standard API | Supported Features |
| :--- | :--- | :--- |
| `POST /v1/messages` | **Anthropic Messages API** | Claude Code CLI, Anthropic SDKs, `thinking` blocks, `tool_use`, `tool_result` |
| `POST /v1/chat/completions` | **OpenAI Chat API** | OpenCode, Cursor, Aider, `reasoning_content`, `tool_calls`, Web Search, SSE Streaming |
| `GET /v1/models` | **OpenAI / Codex Models API** | Model Discovery (`slug`, `context_window`, `supported_reasoning_levels`) |
| `GET /healthz` | **Health Endpoint** | Server status check (`{"status":"ok"}`) with CORS headers |

### Model Catalog
| Model Name / Slug | Mode | Capabilities | Description |
| :--- | :--- | :--- | :--- |
| `deepseek-chat` / `deepseek-v3` | Fast Chat | Text | Standard high-speed conversational model |
| `deepseek-reasoner` / `deepseek-r1` | Reasoning | Text + DeepThink | Reasoning model streaming `reasoning_content` deltas |
| `deepseek-expert` | Expert | Text + DeepThink | High-precision model for complex architecture |
| `deepseek-search` | Web Search | Text + Live Web Search | Model with real-time internet search and citation links |
| `deepseek-reasoner-search` | Hybrid | DeepThink + Live Search | DeepThink R1 combined with live internet search |

---

## 🛠️ Architecture

- **`cmd/server`** — High-performance HTTP server entry point (`net/http` & `log/slog`).
- **`pkg/pow`** — WebAssembly PoW solver executing `sha3_wasm_bg.wasm` via `wazero`.
- **`pkg/auth`** — Session storage, token extraction, and access token resolution (`session/session.json`).
- **`pkg/client`** — Direct HTTP client driver, `/users/current` auto-exchange, `/chat_session/delete` cleaner, and real-time SSE stream parser.
- **`pkg/agentic`** — Multi-format tool parser, trajectory prompt replay builder, thinking extractor, and session cache.
- **`pkg/server`** — OpenAI & Anthropic REST API handlers (`/v1/messages`, `/v1/chat/completions`, `/v1/models`, `/healthz`).

---

## 🚀 Quick Start

### 1. Requirements

- **Go 1.22+**
- A **DeepSeek Account** (free account from [chat.deepseek.com](https://chat.deepseek.com))

### 2. Setting Up Session Token

Create `session/session.json` (or set `DEEPSEEK_TOKEN` in `.env`):

```json
{
  "token": "YOUR_DEEPSEEK_USER_TOKEN_OR_BEARER",
  "cookies": {
    "ds_session_id": "YOUR_SESSION_ID"
  },
  "user_agent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36...",
  "captured_at": 1786623559
}
```

> **💡 Easy Token Capture:**
> 1. Open [chat.deepseek.com](https://chat.deepseek.com) in your browser and log in.
> 2. Open Developer Tools (F12) → **Application** → **Local Storage** → `chat.deepseek.com`.
> 3. Copy the value of `userToken` (raw string or JSON `{"value":"..."}`).
> 4. Paste it into `"token"` in `session/session.json`. The server will handle authentication and auto-refresh automatically!

### 3. Building & Running

```bash
# Clone the repository
git clone https://github.com/joyccn/Deepseek-API.git
cd Deepseek-API

# Build static binary
CGO_ENABLED=0 go build -o deepseek-api-server ./cmd/server

# Run server (default on http://127.0.0.1:8000)
./deepseek-api-server
```

---

## 🔌 CLI & Tool Integrations

### Claude Code CLI
```bash
export ANTHROPIC_BASE_URL="http://localhost:8000"
export ANTHROPIC_API_KEY="dummy-token"
export ANTHROPIC_MODEL="claude-3-7-sonnet-20250219"

claude
```

### Cursor / Aider / OpenCode
- **OpenAI Base URL**: `http://localhost:8000/v1`
- **API Key**: `dummy-token`
- **Model**: `deepseek-chat`, `deepseek-reasoner`, or `deepseek-search`

---

## 📖 Usage Examples

### 1. Anthropic Messages API with Thinking (Claude Code)
```bash
curl http://localhost:8000/v1/messages \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-7-sonnet-20250219",
    "max_tokens": 1024,
    "messages": [{"role": "user", "content": "Explain quantum computing in 2 sentences."}],
    "thinking": {"type": "enabled", "budget_tokens": 1024}
  }'
```

### 2. Live Web Search with Citations
```bash
curl http://localhost:8000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek-search",
    "messages": [{"role": "user", "content": "What is the latest release version of Go language in 2026?"}]
  }'
```

### 3. DeepThink R1 Real-Time Streaming
```bash
curl -N http://localhost:8000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek-reasoner",
    "stream": true,
    "messages": [{"role": "user", "content": "Solve: 17 * 29 + 143"}]
  }'
```

### 4. Agentic Tool Calling
```bash
curl http://localhost:8000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek-chat",
    "messages": [{"role": "user", "content": "What is the weather in Tokyo?"}],
    "tools": [{
      "type": "function",
      "function": {
        "name": "get_weather",
        "description": "Get weather for a city",
        "parameters": {
          "type": "object",
          "properties": {"city": {"type": "string"}},
          "required": ["city"]
        }
      }
    }]
  }'
```

---

## 🧪 Testing

Run comprehensive unit tests across all packages:

```bash
go test ./... -v -count=1
```

---

## 📄 License

[MIT](https://github.com/joyccn/Deepseek-API/blob/main/LICENSE) — Open source under the MIT License.

© 2026-Present Joy.
