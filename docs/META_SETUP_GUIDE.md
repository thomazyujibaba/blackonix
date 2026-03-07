# Guia de Configuração Meta WhatsApp Cloud API + BlackOnix

Este guia cobre todo o processo de configuração da Meta WhatsApp Cloud API para integração com o BlackOnix, incluindo a geração de token permanente.

---

## Pré-requisitos

- Conta no [Meta Business Manager](https://business.facebook.com)
- Um número de telefone para vincular ao WhatsApp Cloud API
- BlackOnix rodando e acessível via HTTPS (ex: `https://server.marinapresentes.com.br/blackonix/`)
- Acesso ao banco de dados PostgreSQL (Supabase)

---

## Parte 1: Criar o App na Meta

### 1.1 Criar um novo App

1. Acesse [developers.facebook.com](https://developers.facebook.com)
2. Clique em **Meus apps** > **Criar app**
3. Selecione o tipo **Business** (Empresa)
4. Preencha o nome do app (ex: "BlackOnix Marina")
5. Vincule ao seu **Business Portfolio** (ex: "Marina")
6. Clique em **Criar app**

### 1.2 Adicionar o produto WhatsApp

1. No painel do app, role até **Adicionar um produto**
2. Encontre o card **WhatsApp** e clique em **Configurar**
3. Selecione sua **WhatsApp Business Account** (WABA) quando solicitado

> **IMPORTANTE:** Se o card WhatsApp não aparecer, o tipo do app pode estar errado. Verifique em **Configurações do app** > **Básico** se o tipo é "Business". Se não for, crie um novo app do tipo correto.

---

## Parte 2: Registrar o Número de Telefone

### 2.1 Desvincular do WhatsApp Business App (se necessário)

Se o número já está registrado no WhatsApp Business App (celular), é preciso desvinculá-lo primeiro:

1. No celular, abra o **WhatsApp Business**
2. Vá em **Configurações** > **Conta** > **Excluir minha conta**
3. Confirme a exclusão
4. **Aguarde até 3 minutos** para a Meta liberar o número

> **ATENÇÃO:** Isso desconecta permanentemente o número do app de celular. Todas as conversas serão perdidas. O número passará a funcionar exclusivamente via Cloud API.

### 2.2 Adicionar o número no painel da Meta

1. No app, vá em **WhatsApp** > **Configuração da API**
2. Na seção **Etapa 1: Selecione números de telefone**, clique para adicionar
3. Insira o número com código do país (ex: +55 27 93618-1191)
4. Escolha verificação por **SMS** ou **Ligação**
5. Insira o código recebido

Após verificação, anote:
- **Identificação do número de telefone** (Phone Number ID): ex: `931696136704755`
- **Identificação da conta do WhatsApp Business** (WABA ID): ex: `1619387585998131`

> **NOTA:** O WABA ID que aparece na Configuração da API pode ser diferente do `entry.id` que chega no webhook. Veja a Parte 5 para descobrir o valor correto.

---

## Parte 3: Configurar o Webhook

### 3.1 Definir URL de callback

1. No app, vá em **WhatsApp** > **Configuração**
2. Na seção **Webhook**, preencha:
   - **URL de callback:** `https://seu-dominio.com/blackonix/webhook`
   - **Verificar token:** o mesmo valor de `META_VERIFY_TOKEN` no `.env` (ex: `blackonix-marina-2026`)
3. Clique em **Verificar e salvar**

> O BlackOnix precisa estar rodando para responder ao challenge de verificação da Meta.

### 3.2 Assinar o campo "messages"

1. Na mesma página, abaixo do webhook, há a lista **Campos do webhook**
2. Encontre o campo **messages**
3. **Ative o toggle** na coluna "Assinar"

> **SEM ESTE PASSO, nenhum webhook de mensagem será enviado.** Todos os campos começam desassinados por padrão.

---

## Parte 4: Gerar Token de Acesso

### 4.1 Token temporário (para testes)

1. Na página **Configuração da API**, clique em **Gerar token de acesso**
2. Selecione a conta WhatsApp Business
3. Copie o token gerado

> Este token expira em **24 horas**. Use apenas para testes iniciais.

### 4.2 Token permanente (System User Token) - RECOMENDADO

Para produção, crie um token que não expira:

#### Passo 1: Criar System User

1. Acesse [business.facebook.com](https://business.facebook.com)
2. Vá em **Configurações do negócio** (Business Settings)
3. No menu lateral: **Usuários** > **Usuários do sistema** (System Users)
4. Clique em **Adicionar** (Add)
5. Nome: `blackonix-bot` (ou qualquer nome)
6. Função: **Admin**
7. Clique em **Criar usuário do sistema**

#### Passo 2: Atribuir ativos ao System User

1. Selecione o System User criado
2. Clique em **Atribuir ativos** (Add Assets)
3. Selecione **Apps** > escolha o app do BlackOnix
4. Marque **Controle total** (Full Control)
5. Clique em **Salvar alterações**
6. Repita para **Contas do WhatsApp** > selecione a WABA > **Controle total**

#### Passo 3: Gerar o token permanente

1. Selecione o System User
2. Clique em **Gerar novo token** (Generate New Token)
3. Selecione o **App** do BlackOnix
4. Marque as permissões:
   - `whatsapp_business_messaging` (obrigatório)
   - `whatsapp_business_management` (obrigatório)
5. Clique em **Gerar token**
6. **COPIE E SALVE o token imediatamente** — ele não será mostrado novamente

> Este token **não expira** e é o recomendado para produção.

---

## Parte 5: Configurar o BlackOnix

### 5.1 Arquivo .env

```env
# Server
SERVER_PORT=3000

# Supabase / PostgreSQL
DATABASE_URL=postgresql://usuario:senha@host:5432/banco

# Meta WhatsApp Cloud API
META_VERIFY_TOKEN=seu-verify-token-aqui
META_APP_SECRET=seu-app-secret-aqui

# LLM Provider
LLM_PROVIDER=openai
LLM_API_KEY=sk-proj-sua-chave-openai
LLM_MODEL=gpt-4.1-mini

# Logging
LOG_LEVEL=info
```

Onde encontrar cada valor:
| Variável | Onde encontrar |
|----------|---------------|
| `META_VERIFY_TOKEN` | Você escolhe (qualquer string secreta) |
| `META_APP_SECRET` | Meta App Dashboard > Configurações do app > Básico > Chave Secreta do App |
| `LLM_API_KEY` | [platform.openai.com](https://platform.openai.com) > API Keys |

### 5.2 Inserir Tenant no banco

O BlackOnix identifica cada cliente (tenant) pelo WABA ID que chega no webhook. Para descobrir o WABA ID correto:

1. Inicie o servidor: `go run ./cmd/server/`
2. Mande uma mensagem de teste (veja Parte 6)
3. Verifique os logs — procure por `tenant not found for WABA XXXXXXXXX`
4. O número `XXXXXXXXX` é o WABA ID real

Crie o tenant no banco:

```sql
INSERT INTO tenants (name, waba_id, meta_token, rocketchat_url, rocketchat_token)
VALUES (
  'Nome da Empresa',
  'WABA_ID_DOS_LOGS',
  'TOKEN_PERMANENTE_DO_SYSTEM_USER',
  '',
  ''
);
```

Ou use o script seed em `cmd/seed/main.go`.

> **IMPORTANTE:** O WABA ID que aparece no painel da Meta pode ser diferente do que chega no webhook (`entry.id`). Sempre use o valor dos logs.

### 5.3 Atualizar o token quando necessário

Se precisar trocar o token (ex: gerou um permanente):

```sql
UPDATE tenants SET meta_token = 'NOVO_TOKEN' WHERE name = 'Nome da Empresa';
```

O servidor pega o token do banco a cada request, então não precisa reiniciar.

---

## Parte 6: Testar

### 6.1 Modo Desenvolvimento

Em modo desenvolvimento, a Meta só envia webhooks de números cadastrados como testadores.

Para testar:

1. Na **Configuração da API**, seção "Enviar e receber mensagens"
2. Em **"Até"**, selecione o país (BR +55) e insira o número pessoal de teste
3. Clique em **Enviar mensagem** — envia o template "hello_world" para seu número
4. **Responda a mensagem no WhatsApp** — isso gera o webhook que chega no BlackOnix
5. Verifique nos logs se o fluxo completou

### 6.2 Verificação rápida do endpoint

```bash
# Health check
curl https://seu-dominio.com/blackonix/health

# Simular verificação do webhook (GET)
curl "https://seu-dominio.com/blackonix/webhook?hub.mode=subscribe&hub.verify_token=SEU_TOKEN&hub.challenge=teste123"
# Deve retornar: teste123
```

### 6.3 Modo Produção (Live)

Para receber mensagens de qualquer número:

1. No topo do painel da Meta, clique em **"Ao vivo"** (ao lado de "desenvolvimento")
2. A Meta pode exigir:
   - Política de privacidade (URL)
   - Verificação do negócio
   - Forma de pagamento configurada
3. Após aprovação, qualquer número pode enviar mensagens para o número registrado

---

## Parte 7: Configuração do nginx (Reverse Proxy)

Se usar nginx para expor o BlackOnix:

### 7.1 Adicionar upstream

No `nginx.conf`, na seção `http`:

```nginx
upstream blackonix { server host.docker.internal:3000; }
```

### 7.2 Adicionar location

Dentro do server block HTTPS existente:

```nginx
# BlackOnix Agentic Middleware (WhatsApp webhook)
location /blackonix/ {
    proxy_pass http://blackonix/;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_http_version 1.1;
}
```

> A barra final em `proxy_pass http://blackonix/;` faz o strip do prefixo `/blackonix/`, então `/blackonix/webhook` vira `/webhook` para o BlackOnix.

### 7.3 Reiniciar nginx

```bash
docker compose restart nginx
```

---

## Troubleshooting

| Problema | Causa | Solução |
|----------|-------|---------|
| 502 Bad Gateway | BlackOnix não está rodando | Inicie o servidor na porta 3000 |
| Webhook não chega | Campo "messages" não assinado | Ative o toggle em Configuração > Campos do webhook |
| Webhook não chega | App em modo desenvolvimento | Adicione número de teste ou publique o app |
| "tenant not found for WABA X" | WABA ID no banco não corresponde | Atualize o `waba_id` no banco com o valor dos logs |
| "meta API returned status 400" | Token inválido ou expirado | Gere novo token e atualize no banco |
| "meta API returned status 400" | Número de destino não existe | Normal para testes simulados com números falsos |
| Número já registrado | Número ainda vinculado ao WhatsApp Business App | Exclua a conta no app do celular e aguarde 3 min |
| Card WhatsApp não aparece | Tipo do app não é Business | Crie um novo app do tipo Business |

---

## Referência Rápida

| Item | Valor (Marina Presentes) |
|------|--------------------------|
| Webhook URL | `https://server.marinapresentes.com.br/blackonix/webhook` |
| Verify Token | `blackonix-marina-2026` |
| WABA ID (webhook) | `2801340923547916` |
| Phone Number ID | `931696136704755` |
| Número WhatsApp | +55 27 93618-1191 |
| Servidor | `localhost:3000` |
| Banco | Supabase (sa-east-1) |
