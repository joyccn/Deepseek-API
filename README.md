# Deepseek-API

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go)](https://go.dev)
[![Build Status](https://img.shields.io/badge/build-passing-brightgreen.svg)]()

High-performance, 100% native **Golang** OpenAI & Anthropic compatible API relay server (`http://localhost:8000`) powered by personal DeepSeek Web accounts.

Designed specifically to serve as a fast, reliable backend provider for **Agentic Coding CLIs** (e.g., Claude Code, OpenCode, OpenAI Codex CLI, Cursor, Aider, Continue, etc.) with support for **Anthropic Messages API**, **OpenAI Chat Completions**, **DeepThink R1 Reasoning**, **Web Search**, and **Tool Calling**.

---

## Features

- **Pure Go WASM PoW Solver (`wazero`)** — Zero CGo dependencies. Solves DeepSeek `sha3_wasm_bg.wasm` challenges natively in ~0.16s.
- **Dual API Compatibility (Anthropic + OpenAI)** — Dual endpoint support for both Anthropic Messages API (`POST /v1/messages`) and OpenAI Chat (`POST /v1/chat/completions`).
- **Claude Code Integration** — Native support for Claude Code CLI and Anthropic SDKs via `/v1/messages` with `thinking` blocks (`budget_tokens`) and `tool_use`.
- **OpenAI Codex CLI Compatibility** — Native discovery schema for `GET /v1/models` including reasoning levels, slug, and context windows.
- **Clean Reasoning Output Parser** — Separates `THINK` and `RESPONSE` stream fragments cleanly. `reasoning_content` & `thinking` are delivered in standard delta formats without corrupting main answer text.
- **Agentic Tool Calling Bridge** — Converts OpenAI `tools` and Anthropic `tools` definitions into prompt instructions and parses XML/JSON `<tool_call>` outputs back into standard `tool_calls` / `tool_use`.
- **Stateful Multi-turn Context & Reset Triggers** — Thread-safe session cache maps client message arrays to DeepSeek `chat_session_id` continuously. Automatically resets session on `/clear`, `/reset`, `/new`, or `/compact`.

---

## Compatibility

Deepseek-API is a full drop-in replacement for OpenAI and Anthropic endpoints:

| Endpoint | Standard API | Supported Features |
| :--- | :--- | :--- |
| `POST /v1/messages` | **Anthropic Messages API** | Claude Code CLI, Anthropic SDKs, `thinking` blocks, `tool_use` |
| `POST /v1/chat/completions` | **OpenAI Chat API** | OpenCode, Cursor, Aider, `reasoning_content`, `tool_calls`, SSE Streaming |
| `GET /v1/models` | **OpenAI / Codex Models API** | Model Discovery (`deepseek-chat`, `deepseek-expert`) |
| `GET /healthz` | **Health Endpoint** | Server status check (`{"status":"ok"}`) |

---

## Architecture

- **`cmd/server`** — High-performance HTTP server entry point (`net/http` & `log/slog`).
- **`pkg/pow`** — WebAssembly PoW solver executing `sha3_wasm_bg.wasm` via `wazero`.
- **`pkg/auth`** — Session storage and bearer token validator (`session/session.json`).
- **`pkg/client`** — Direct HTTP client driver & real-time SSE stream fragment parser.
- **`pkg/agentic`** — Thread-safe session cache, prompt injector, thinking extractor, and tool call parser.
- **`pkg/server`** — OpenAI & Anthropic REST API handlers (`/v1/messages`, `/v1/chat/completions`, `/v1/models`, `/healthz`).

---

## Quick Start

### 1. Requirements

- **Go 1.22+**
- A **DeepSeek Account** (free account from [chat.deepseek.com](https://chat.deepseek.com))

### 2. Capturing Session Token & Cookies

Create `session/session.json` containing your signed-in DeepSeek browser session:

```json
{
  "token": "YOUR_DEEPSEEK_BEARER_TOKEN",
  "cookies": {
    "ds_session_id": "YOUR_SESSION_ID"
  },
  "user_agent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36...",
  "captured_at": 1786623559
}
```

> **How to get your session token manually:**
> 1. Open [chat.deepseek.com](https://chat.deepseek.com) in your browser and log in.
> 2. Open Developer Tools (F12) -> Console.
> 3. Type `JSON.parse(localStorage.getItem('userToken')).value` to get your bearer `token`.
> 4. In Application -> Cookies, copy `ds_session_id`.

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

## Examples

Run any of the included examples in the [`examples/`](examples) directory:

```bash
# Direct chat completion
go run ./examples/01_direct_chat

# Direct SSE streaming
go run ./examples/02_direct_stream

# DeepThink R1 Reasoning
go run ./examples/03_deepthink_reasoning

# Web Search integration
go run ./examples/04_web_search

# Tool Calling parser
go run ./examples/05_tool_calling

# Claude Code / Anthropic Messages API
go run ./examples/06_claude_code_anthropic

# Multi-turn Long Conversation & Reset Triggers
go run ./examples/07_long_conversation_test

# End-to-End Snake Game Generation & Test Creation
go run ./examples/08_generate_snake_game

# Refinement & Code Fixing via API
go run ./examples/09_refine_snake_game
```

### HTTP Curl Examples

#### Anthropic Messages API (Claude Code CLI / Anthropic SDK)
```bash
curl http://localhost:8000/v1/messages \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-sonnet-20241022",
    "max_tokens": 1024,
    "messages": [{"role": "user", "content": "Hello Claude Code!"}],
    "thinking": {"type": "enabled", "budget_tokens": 1024}
  }'
```

#### Standard OpenAI Chat Completion
```bash
curl http://localhost:8000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek-chat",
    "messages": [{"role": "user", "content": "Hello Golang server!"}]
  }'
```

---

## Development

Run unit tests across all packages:

```bash
go test ./... -v
```

---

## License

[MIT](https://github.com/joyccn/Deepseek-API/blob/main/LICENSE) — Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software.

© 2026-Present Joy.
