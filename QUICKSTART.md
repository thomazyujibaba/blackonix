# BlackOnix - Guia Rapido de Instalacao

## Pre-requisitos

1. **Go 1.25+** - [Download](https://go.dev/dl/)
2. **PostgreSQL** - Local ou via [Supabase](https://supabase.com) (gratuito)
3. **Conta Meta Developer** - Para WhatsApp Cloud API (opcional)
4. **Bot do Telegram** - Via @BotFather (opcional)

Verifique se o Go esta instalado:

```bash
go version
```

## 1. Clonar o projeto

```bash
git clone https://github.com/thomazyujibaba/blackonix.git
cd blackonix
```

## 2. Configurar variaveis de ambiente

Copie o arquivo de exemplo e edite com seus dados:

```bash
cp .env.example .env
```

Abra o `.env` e preencha:

```env
# Porta do servidor (padrao: 3000)
SERVER_PORT=3000

# URL de conexao do PostgreSQL
DATABASE_URL=postgresql://postgres:SUA_SENHA@localhost:5432/blackonix

# Token de verificacao do webhook Meta (voce inventa)
META_VERIFY_TOKEN=meu-token-secreto-123

# App Secret da Meta (Meta Developer > App > Settings > Basic)
META_APP_SECRET=seu-app-secret

# Chave da OpenAI (https://platform.openai.com/api-keys)
LLM_API_KEY=sk-sua-chave-aqui

# Modelo da LLM (padrao: gpt-4o)
LLM_MODEL=gpt-4o

# JWT Secret para autenticacao do dashboard
JWT_SECRET=um-secret-forte-aqui

# Nivel de log (info, debug, warn, error)
LOG_LEVEL=info
```

### De onde tirar cada valor?

| Variavel | Onde encontrar |
|---|---|
| `DATABASE_URL` | PostgreSQL local ou Supabase > Project Settings > Database > Connection string (URI) |
| `META_VERIFY_TOKEN` | Voce inventa (qualquer string). Usa o mesmo na config do webhook na Meta |
| `META_APP_SECRET` | Meta for Developers > Seu App > Settings > Basic > App Secret |
| `LLM_API_KEY` | OpenAI > API Keys > Create new secret key |
| `JWT_SECRET` | Qualquer string forte (use `openssl rand -hex 32`) |

## 3. Instalar dependencias

```bash
go mod download
```

## 4. Rodar o servidor

```bash
go run cmd/server/main.go
```

Voce vera:

```
BlackOnix starting on :3000
```

## 5. Verificar se esta funcionando

```bash
curl http://localhost:3000/health
```

Resposta esperada:

```json
{"service":"blackonix","status":"ok"}
```

## 6. Cadastrar Tenant e Canais

O sistema e multi-tenant com multiplos canais por tenant. Use o seed script:

```bash
# Somente tenant (sem canais)
go run cmd/seed/main.go

# Com canal WhatsApp
WABA_ID=seu-waba-id META_TOKEN=seu-meta-token go run cmd/seed/main.go

# Com canal Telegram
TELEGRAM_BOT_TOKEN=123456:ABC-DEF go run cmd/seed/main.go

# Com ambos
WABA_ID=seu-waba-id META_TOKEN=seu-meta-token TELEGRAM_BOT_TOKEN=123456:ABC-DEF go run cmd/seed/main.go
```

## 7. Configurar Webhook do WhatsApp

1. Va em [Meta for Developers](https://developers.facebook.com)
2. Seu App > WhatsApp > Configuration > Webhook
3. Clique em **Edit**
4. **Callback URL**: `https://seu-dominio.com/webhook/whatsapp` (precisa ser HTTPS publico)
5. **Verify token**: O mesmo valor que voce colocou em `META_VERIFY_TOKEN`
6. Clique em **Verify and save**
7. Em **Webhook fields**, inscreva-se em `messages`

### Para testes locais (sem dominio publico)

Use o [ngrok](https://ngrok.com) para expor sua porta local:

```bash
ngrok http 3000
```

O ngrok vai gerar uma URL tipo `https://abc123.ngrok.io`. Use essa URL + `/webhook/whatsapp` na configuracao da Meta.

## 8. Configurar Webhook do Telegram

1. Crie um bot no Telegram via [@BotFather](https://t.me/BotFather)
2. Copie o token do bot (formato: `123456:ABC-DEF`)
3. Registre o webhook:

```bash
TELEGRAM_BOT_TOKEN=123456:ABC-DEF \
TELEGRAM_WEBHOOK_URL=https://seu-dominio.com/webhook/telegram/123456:ABC-DEF \
go run cmd/telegram-setup/main.go
```

A URL do webhook inclui o token do bot para validacao.

## Pronto!

Envie uma mensagem para o WhatsApp ou Telegram configurado. O BlackOnix vai:

1. Receber a mensagem via webhook
2. Identificar o canal pelo ExternalID (WABA ID ou Bot ID)
3. Criar o contato e sessao automaticamente
4. Se for audio, transcrever via Whisper
5. Processar com a LLM (modo BOT)
6. Responder via o canal de origem

## Migracao de Dados

Se voce ja tinha tenants com campos `waba_id` e `meta_token` na tabela antiga, rode o script de migracao:

```bash
go run cmd/migrate-channels/main.go
```

Isso cria registros na tabela `channels` e remove as colunas antigas do `tenants`.

## Troubleshooting

| Problema | Solucao |
|---|---|
| `DATABASE_URL is required` | Preencha a variavel `DATABASE_URL` no `.env` |
| `JWT_SECRET is required` | Preencha a variavel `JWT_SECRET` no `.env` |
| Webhook WhatsApp nao verifica | Confirme que `META_VERIFY_TOKEN` no `.env` e igual ao configurado na Meta |
| Webhook Telegram nao funciona | Verifique se o bot token na URL corresponde ao registrado no banco |
| Mensagem nao chega | Verifique se o ngrok esta rodando e a URL no webhook esta correta |
| Erro de LLM | Verifique se `LLM_API_KEY` e valida e tem creditos |
| Tabelas nao existem | O servidor cria as tabelas automaticamente ao iniciar (AutoMigrate) |
| Canal nao encontrado | Rode o seed com as variaveis de ambiente do canal (`WABA_ID`, `TELEGRAM_BOT_TOKEN`) |
