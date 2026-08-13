# Deepseek-API

High-performance, 100% native **Golang** OpenAI-compatible API relay server (`http://localhost:8000/v1`) powered by personal DeepSeek Web accounts.

Designed specifically to serve as a fast, reliable backend provider for **Agentic Coding CLIs** (e.g., Claude Code, Cursor, Gemini CLI, Qwen Code, etc.) with support for **DeepThink R1 Reasoning**, **Web Search**, and **Tool Calling**.

---

## Features

- **Pure Go WASM PoW Solver (`wazero`)** — Zero CGo dependencies. Solves DeepSeek `sha3_wasm_bg.wasm` challenges natively in ~0.16s.
- **Clean Reasoning Output Parser** — Separates `THINK` and `RESPONSE` stream fragments. `reasoning_content` is delivered in standard OpenAI delta format without corrupting main answer text.
- **Agentic Tool Calling Bridge** — Converts OpenAI `tools` definitions into prompt instructions and parses XML/JSON `<tool_call>` outputs back into standard OpenAI `tool_calls`.
- **OpenAI Drop-in Compatibility** — Full support for `GET /v1/models`, `POST /v1/chat/completions` (JSON & SSE streaming `stream: true`).
- **Stateful Multi-turn Context** — Thread-safe session cache maps OpenAI client message arrays to DeepSeek `chat_session_id` continuously.

---

## Compatibility

Deepseek-API is a full drop-in replacement for OpenAI endpoints. All models and toggles match the official web interface:

| Model Name | DeepSeek Mode | Features Supported |
| :--- | :--- | :--- |
| `deepseek-chat` | Instant (Fast) | Text Completion, DeepThink R1, Web Search, Tool Calling |
| `deepseek-expert` | Expert (Strong) | Complex Reasoning, DeepThink R1, Web Search, Tool Calling |

---

## Architecture

- **`cmd/server`** — High-performance HTTP server entry point (`net/http` & `log/slog`).
- **`pkg/pow`** — WebAssembly PoW solver executing `sha3_wasm_bg.wasm` via `wazero`.
- **`pkg/auth`** — Session storage and bearer token validator (`session/session.json`).
- **`pkg/client`** — Direct HTTP client driver & real-time SSE stream fragment parser.
- **`pkg/agentic`** — Thread-safe session cache, prompt injector, and tool call parser.
- **`pkg/server`** — OpenAI-compatible REST API handlers (`/v1/chat/completions`, `/v1/models`, `/healthz`).

---

## Quick Start

### 1. Requirements

- **Go 1.26+** (or Go 1.22+)
- A **DeepSeek Account** (the free account from [chat.deepseek.com](https://chat.deepseek.com))

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
```

### HTTP Curl Examples

#### Standard Chat Completion
```bash
curl http://localhost:8000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek-chat",
    "messages": [{"role": "user", "content": "Hello Golang server!"}]
  }'
```

#### DeepThink R1 Reasoning Mode
```bash
curl http://localhost:8000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek-chat",
    "thinking": true,
    "messages": [{"role": "user", "content": "Which is bigger: 9.11 or 9.9?"}]
  }'
```

#### Streaming SSE Mode
```bash
curl http://localhost:8000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek-expert",
    "stream": true,
    "messages": [{"role": "user", "content": "Count from 1 to 5."}]
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
