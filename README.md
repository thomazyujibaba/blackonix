# BlackOnix

Agentic Middleware para multi-atendimento via WhatsApp e Telegram. Orquestra mensagens entre plataformas de messaging, Rocket.Chat (Omnichannel) e LLMs com Function Calling, usando PostgreSQL como banco de dados.

## O que e

BlackOnix e um SaaS B2B que funciona como um middleware inteligente entre canais de messaging e seus sistemas internos. Ele:

- Recebe mensagens do WhatsApp e Telegram via webhook
- Processa com IA (LLM + Function Calling) no modo BOT
- Transfere para atendentes humanos no Rocket.Chat quando necessario
- Suporta multiplas empresas (multi-tenant) com multiplos canais por tenant
- E extensivel via sistema de plugins
- Suporta respostas ricas (botoes inline, teclados customizados) no Telegram

## Arquitetura

```
WhatsApp ──webhook──▸ BlackOnix ──▸ LLM (OpenAI)
Telegram ──webhook──▸     │              │
                          │         Tool Calls
                          │              │
                          ▼              ▼
                     Rocket.Chat    Plugins (Whisper, etc.)
                     (Omnichannel)
                          │
                     Atendente Humano
```

### Padroes

- **Hexagonal Architecture** (Ports & Adapters)
- **Channel Abstraction** - Interface `MessagingChannel` para suportar multiplas plataformas
- **Injecao de Dependencia** via interfaces
- **Plugin/Tool Registry** para extensao dinamica
- **State Machine** para controle de fluxo (BOT <-> HUMAN)

### Estrutura

```
blackonix/
├── cmd/
│   ├── server/main.go                 # Ponto de entrada + DI
│   ├── seed/main.go                   # Seed de tenant e channels
│   ├── migrate-channels/main.go       # Migracao de dados para Channel model
│   └── telegram-setup/main.go         # Registro de webhook no Telegram
├── internal/
│   ├── config/                        # Variaveis de ambiente (.env)
│   ├── domain/                        # Modelos: Tenant, Channel, Contact, Session, Message
│   ├── repository/                    # Interfaces + implementacoes GORM (PostgreSQL)
│   ├── ports/                         # Interfaces: MessagingChannel, RocketChatAPI, LLMClient
│   ├── adapters/
│   │   ├── meta/                      # WhatsApp Cloud API (MessagingChannel)
│   │   ├── telegram/                  # Telegram Bot API (MessagingChannel)
│   │   ├── rocketchat/                # Rocket.Chat Livechat
│   │   └── llm/                       # OpenAI (Chat Completions)
│   ├── core/
│   │   ├── agent/                     # AgentTool interface, ToolRegistry, Orchestrator
│   │   └── state/                     # Maquina de estados (BOT <-> HUMAN)
│   ├── handlers/                      # Webhook handlers (Fiber)
│   └── plugins/                       # Tools: TransferToHuman, AudioTranscriber
```

## Stack

- **Go** 1.25+
- **Fiber** v2 (web framework)
- **GORM** (ORM para PostgreSQL)
- **PostgreSQL**
- **OpenAI** API (Function Calling + Whisper)

## Quick Start

```bash
# 1. Clone
git clone https://github.com/thomazyujibaba/blackonix.git
cd blackonix

# 2. Configure
cp .env.example .env
# Edite o .env com suas credenciais

# 3. Instale dependencias
go mod download

# 4. Rode
go run cmd/server/main.go
```

### Configurar Telegram

```bash
# 1. Crie um bot no @BotFather e obtenha o token
# 2. Registre o webhook
TELEGRAM_BOT_TOKEN=123456:ABC-DEF \
TELEGRAM_WEBHOOK_URL=https://seudominio.com/webhook/telegram/123456:ABC-DEF \
go run cmd/telegram-setup/main.go

# 3. Crie o channel no banco
WABA_ID=... META_TOKEN=... TELEGRAM_BOT_TOKEN=123456:ABC-DEF \
go run cmd/seed/main.go
```

## Channel Abstraction

O sistema usa uma interface `MessagingChannel` que abstrai a comunicacao com qualquer plataforma:

```go
type MessagingChannel interface {
    ParseWebhook(body []byte) ([]NormalizedMessage, error)
    SendResponse(ctx context.Context, channel *domain.Channel, to string, response RichResponse) error
    DownloadMedia(ctx context.Context, channel *domain.Channel, mediaID string) ([]byte, error)
    VerifyWebhook(req VerifyRequest) (string, error)
}
```

Cada tenant pode ter multiplos canais. As credenciais ficam no campo `Credentials` (JSONB) do model `Channel`:

- **WhatsApp**: `{"meta_token":"...", "waba_id":"...", "phone_number_id":"..."}`
- **Telegram**: `{"bot_token":"..."}`

## Fluxo do Webhook

1. Webhook recebe mensagem (`POST /webhook/whatsapp` ou `POST /webhook/telegram/:token`)
2. Adapter faz parse para `NormalizedMessage` (formato canal-agnostico)
3. Identifica o Channel pelo ExternalID (WABA ID ou Bot ID)
4. Cria/carrega Contact e Session
5. **Se audio** -> transcreve via Whisper automaticamente
6. **Se estado = HUMAN** -> encaminha para Rocket.Chat
7. **Se estado = BOT** -> envia para LLM com Tools registradas
8. Se a LLM pedir Tool Call -> executa via Registry -> devolve resultado
9. Responde ao cliente via canal de origem (com suporte a botoes no Telegram)

## Sistema de Plugins

Criar um novo plugin e simples. Implemente a interface `AgentTool`:

```go
type AgentTool interface {
    Name() string
    Description() string
    ParametersSchema() interface{}
    Execute(ctx context.Context, params map[string]interface{}) (string, error)
}
```

Registre no `ToolRegistry`:

```go
registry.Register(meuNovoPlugin)
```

### Plugins incluidos

| Plugin | Descricao |
|---|---|
| `transcribe_audio` | Transcreve audios de qualquer canal via OpenAI Whisper (automatico) |
| `transfer_to_human` | Transfere conversa para atendente no Rocket.Chat |

## Endpoints

| Metodo | Rota | Descricao |
|---|---|---|
| `GET` | `/health` | Health check |
| `GET` | `/webhook/whatsapp` | Verificacao do webhook (Meta) |
| `POST` | `/webhook/whatsapp` | Recebe mensagens do WhatsApp |
| `POST` | `/webhook/telegram/:token` | Recebe mensagens do Telegram |

## Tipos de Mensagem Suportados

| Tipo | WhatsApp | Telegram |
|---|---|---|
| Texto | Sim | Sim |
| Audio | Sim (transcricao automatica) | Sim (transcricao automatica) |
| Foto | - | Sim |
| Video | - | Sim |
| Callback (botoes) | - | Sim |

## Licenca

MIT
