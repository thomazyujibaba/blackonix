# BlackOnix

Agentic Middleware para multi-atendimento via WhatsApp. Orquestra mensagens entre a WhatsApp Cloud API (Meta), Rocket.Chat (Omnichannel) e LLMs com Function Calling, usando Supabase (PostgreSQL) como banco de dados.

## O que é

BlackOnix é um SaaS B2B que funciona como um middleware inteligente entre o WhatsApp e seus sistemas internos. Ele:

- Recebe mensagens do WhatsApp via webhook da Meta
- Processa com IA (LLM + Function Calling) no modo BOT
- Transfere para atendentes humanos no Rocket.Chat quando necessário
- Suporta múltiplas empresas (multi-tenant)
- É extensível via sistema de plugins

## Arquitetura

```
WhatsApp ──webhook──▸ BlackOnix ──▸ LLM (OpenAI)
                         │              │
                         │         Tool Calls
                         │              │
                         ▼              ▼
                    Rocket.Chat    Plugins (Whisper, PIX, etc.)
                    (Omnichannel)
                         │
                    Atendente Humano
```

### Padrões

- **Hexagonal Architecture** (Ports & Adapters)
- **Injeção de Dependência** via interfaces
- **Plugin/Tool Registry** para extensão dinâmica
- **State Machine** para controle de fluxo (BOT ↔ HUMAN)

### Estrutura

```
blackonix/
├── cmd/server/main.go              # Ponto de entrada + DI
├── internal/
│   ├── config/                     # Variáveis de ambiente (.env)
│   ├── domain/                     # Modelos: Tenant, Contact, Session, Message
│   ├── repository/                 # Interfaces + implementações Gorm (PostgreSQL)
│   ├── ports/                      # Interfaces: MetaAPI, RocketChatAPI, LLMClient
│   ├── adapters/
│   │   ├── meta/                   # Cliente WhatsApp Cloud API
│   │   ├── rocketchat/             # Cliente Rocket.Chat Livechat
│   │   └── llm/                    # Cliente OpenAI (Chat Completions)
│   ├── core/
│   │   ├── agent/                  # AgentTool interface, ToolRegistry, Orchestrator
│   │   └── state/                  # Máquina de estados (BOT ↔ HUMAN)
│   ├── handlers/                   # Webhook handlers (Fiber)
│   └── plugins/                    # Tools: TransferToHuman, AudioTranscriber
```

## Stack

- **Go** 1.21+
- **Fiber** v2 (web framework)
- **Gorm** (ORM para PostgreSQL)
- **Supabase** PostgreSQL
- **OpenAI** API (Function Calling)

## Quick Start

```bash
# 1. Clone
git clone https://github.com/thomazyujibaba/blackonix.git
cd blackonix

# 2. Configure
cp .env.example .env
# Edite o .env com suas credenciais (veja QUICKSTART.md para detalhes)

# 3. Instale dependências
go mod download

# 4. Rode
go run cmd/server/main.go
```

Veja o [QUICKSTART.md](QUICKSTART.md) para o guia completo com screenshots e troubleshooting.

## Fluxo do Webhook

1. `POST /webhook` recebe mensagem da Meta
2. Identifica o Tenant pelo WABA ID
3. Cria/carrega Contact e Session
4. **Se áudio** → transcreve via Whisper automaticamente
5. **Se estado = HUMAN** → encaminha para Rocket.Chat
6. **Se estado = BOT** → envia para LLM com Tools registradas
7. Se a LLM pedir Tool Call → executa via Registry → devolve resultado
8. Responde ao cliente via WhatsApp

## Sistema de Plugins

Criar um novo plugin é simples. Implemente a interface `AgentTool`:

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
registry.Register(meuNovoPPlugin)
```

### Plugins incluídos

| Plugin | Descrição |
|---|---|
| `transcribe_audio` | Transcreve áudios do WhatsApp via OpenAI Whisper (automático) |
| `transfer_to_human` | Transfere conversa para atendente no Rocket.Chat |

### Ideias para expansão

- `PixGenerator` - Gerar cobranças PIX
- `SalesCopilot` - Copiloto de vendas com contexto do cliente
- `OrderTracker` - Rastreamento de pedidos

## Endpoints

| Método | Rota | Descrição |
|---|---|---|
| `GET` | `/health` | Health check |
| `GET` | `/webhook` | Verificação do webhook (Meta) |
| `POST` | `/webhook` | Recebe mensagens do WhatsApp |

## Licença

MIT
