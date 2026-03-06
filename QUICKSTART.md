# BlackOnix - Guia Rápido de Instalação

## Pré-requisitos

1. **Go 1.21+** - [Download](https://go.dev/dl/)
2. **PostgreSQL** - Pode ser local ou via [Supabase](https://supabase.com) (gratuito)
3. **Conta Meta Developer** - Para a WhatsApp Cloud API

Verifique se o Go está instalado:

```bash
go version
```

## 1. Clonar o projeto

```bash
git clone <url-do-repositorio>
cd blackonix
```

## 2. Configurar variáveis de ambiente

Copie o arquivo de exemplo e edite com seus dados:

```bash
cp .env.example .env
```

Abra o `.env` e preencha:

```env
# Porta do servidor (padrão: 3000)
SERVER_PORT=3000

# URL de conexão do PostgreSQL (Supabase ou local)
# Supabase: vá em Project Settings > Database > Connection string > URI
DATABASE_URL=postgresql://postgres:SUA_SENHA@db.SEU_PROJETO.supabase.co:5432/postgres

# Token de verificação do webhook (você inventa, ex: meu-token-secreto-123)
META_VERIFY_TOKEN=meu-token-secreto-123

# App Secret da Meta (Meta Developer > App > Settings > Basic)
META_APP_SECRET=seu-app-secret

# Chave da OpenAI (https://platform.openai.com/api-keys)
LLM_API_KEY=sk-sua-chave-aqui

# Modelo da LLM (padrão: gpt-4o)
LLM_MODEL=gpt-4o

# Nível de log (info, debug, warn, error)
LOG_LEVEL=info
```

### De onde tirar cada valor?

| Variável | Onde encontrar |
|---|---|
| `DATABASE_URL` | Supabase > Project Settings > Database > Connection string (URI) |
| `META_VERIFY_TOKEN` | Você inventa (qualquer string). Vai usar o mesmo na config do webhook na Meta |
| `META_APP_SECRET` | Meta for Developers > Seu App > Settings > Basic > App Secret |
| `LLM_API_KEY` | OpenAI > API Keys > Create new secret key |

## 3. Instalar dependências

```bash
go mod download
```

## 4. Rodar o servidor

```bash
go run cmd/server/main.go
```

Você verá:

```
BlackOnix starting on :3000
```

## 5. Verificar se está funcionando

Acesse no navegador ou com curl:

```bash
curl http://localhost:3000/health
```

Resposta esperada:

```json
{"service":"blackonix","status":"ok"}
```

## 6. Configurar o Webhook na Meta

1. Vá em [Meta for Developers](https://developers.facebook.com)
2. Seu App > WhatsApp > Configuration > Webhook
3. Clique em **Edit**
4. **Callback URL**: `https://seu-dominio.com/webhook` (precisa ser HTTPS público)
5. **Verify token**: O mesmo valor que você colocou em `META_VERIFY_TOKEN`
6. Clique em **Verify and save**
7. Em **Webhook fields**, inscreva-se em `messages`

### Para testes locais (sem domínio público)

Use o [ngrok](https://ngrok.com) para expor sua porta local:

```bash
# Instale o ngrok (https://ngrok.com/download)
ngrok http 3000
```

O ngrok vai gerar uma URL tipo `https://abc123.ngrok.io`. Use essa URL + `/webhook` na configuração da Meta.

## 7. Cadastrar um Tenant no banco

O sistema é multi-tenant. Você precisa cadastrar sua empresa no banco para que o webhook funcione.

Execute este SQL no Supabase (SQL Editor) ou em qualquer cliente PostgreSQL:

```sql
INSERT INTO tenants (id, name, waba_id, meta_token, rocketchat_url, rocketchat_token)
VALUES (
  gen_random_uuid(),
  'Minha Empresa',
  'SEU_WABA_ID',          -- Meta > WhatsApp > API Setup > WhatsApp Business Account ID
  'SEU_META_ACCESS_TOKEN', -- Meta > WhatsApp > API Setup > Temporary access token
  'https://seu-rocketchat.com', -- URL do Rocket.Chat (opcional)
  'token-do-rocketchat'         -- Token do Rocket.Chat (opcional)
);
```

O `WABA_ID` e o `Access Token` estão em: Meta for Developers > Seu App > WhatsApp > API Setup.

## Pronto!

Envie uma mensagem para o número de teste do WhatsApp configurado na Meta. O BlackOnix vai:

1. Receber a mensagem via webhook
2. Identificar o tenant pelo WABA ID
3. Criar o contato e sessão automaticamente
4. Processar com a LLM (modo BOT)
5. Responder via WhatsApp

## Estrutura do Projeto

```
blackonix/
├── cmd/server/main.go          # Ponto de entrada
├── internal/
│   ├── config/                 # Variáveis de ambiente
│   ├── domain/                 # Modelos (Tenant, Contact, Session, Message)
│   ├── repository/             # Acesso ao banco (PostgreSQL/Gorm)
│   ├── ports/                  # Interfaces para serviços externos
│   ├── adapters/               # Implementações (Meta, RocketChat, OpenAI)
│   ├── core/
│   │   ├── agent/              # Tool Registry + Orchestrator (cérebro)
│   │   └── state/              # Máquina de estados (BOT <-> HUMAN)
│   ├── handlers/               # Rotas do webhook (Fiber)
│   └── plugins/                # Tools expansíveis (CheckStock, TransferToHuman)
└── .env.example
```

## Troubleshooting

| Problema | Solução |
|---|---|
| `DATABASE_URL is required` | Preencha a variável `DATABASE_URL` no `.env` |
| Webhook não verifica | Confirme que `META_VERIFY_TOKEN` no `.env` é igual ao configurado na Meta |
| Mensagem não chega | Verifique se o ngrok está rodando e a URL no webhook da Meta está correta |
| Erro de LLM | Verifique se `LLM_API_KEY` é válida e tem créditos |
| Tabelas não existem | O servidor cria as tabelas automaticamente ao iniciar (AutoMigrate) |
