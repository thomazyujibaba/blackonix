# BlackOnix Dashboard Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build an admin dashboard with Go REST API + Next.js frontend for managing tenants, sessions, contacts, and viewing conversation history.

**Architecture:** Go/Fiber REST API under `/api/v1/` with JWT auth. Next.js 15 App Router frontend in `web/` consuming the API. Two independent processes — Go on :3000, Next.js on :3001.

**Tech Stack:** Go 1.25, Fiber v2, GORM, bcrypt, JWT | Next.js 15, shadcn/ui, Tailwind CSS, TypeScript

---

## Phase 1: Go Backend REST API

### Task 1: User Domain Model

**Files:**
- Create: `internal/domain/user.go`

**Step 1: Create the User model**

```go
package domain

import "time"

type UserRole string

const (
	UserRoleAdmin  UserRole = "ADMIN"
	UserRoleTenant UserRole = "TENANT"
)

type User struct {
	ID           string   `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Email        string   `gorm:"uniqueIndex;not null" json:"email"`
	PasswordHash string   `gorm:"not null" json:"-"`
	Role         UserRole `gorm:"type:varchar(10);not null;default:'TENANT'" json:"role"`
	TenantID     *string  `gorm:"type:uuid" json:"tenant_id,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	Tenant *Tenant `gorm:"foreignKey:TenantID" json:"tenant,omitempty"`
}
```

**Step 2: Add User to AutoMigrate**

Modify `internal/repository/database.go` — add `&domain.User{}` to the AutoMigrate call.

**Step 3: Commit**

```bash
git add internal/domain/user.go internal/repository/database.go
git commit -m "feat: add User domain model with role-based access"
```

---

### Task 2: Config — Add JWT Secret and CORS Origin

**Files:**
- Modify: `internal/config/config.go`

**Step 1: Add JWT and CORS fields to Config struct**

Add to the `Config` struct:
```go
JWTSecret    string
CORSOrigin   string
```

Add to `Load()`:
```go
JWTSecret:    getEnv("JWT_SECRET", ""),
CORSOrigin:   getEnv("CORS_ORIGIN", "http://localhost:3001"),
```

Add validation after DatabaseURL check:
```go
if cfg.JWTSecret == "" {
    return nil, fmt.Errorf("JWT_SECRET is required")
}
```

**Step 2: Add to .env**

```
JWT_SECRET=your-secret-key-at-least-32-chars-long
CORS_ORIGIN=http://localhost:3001
```

**Step 3: Commit**

```bash
git add internal/config/config.go
git commit -m "feat: add JWT_SECRET and CORS_ORIGIN config"
```

---

### Task 3: User Repository

**Files:**
- Modify: `internal/repository/interfaces.go`
- Create: `internal/repository/user_repo.go`

**Step 1: Add UserRepository interface**

Add to `interfaces.go`:
```go
type UserRepository interface {
    FindByEmail(ctx context.Context, email string) (*domain.User, error)
    FindByID(ctx context.Context, id string) (*domain.User, error)
    Create(ctx context.Context, user *domain.User) error
}
```

**Step 2: Implement user_repo.go**

```go
package repository

import (
    "context"
    "blackonix/internal/domain"
    "gorm.io/gorm"
)

type userRepo struct {
    db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
    return &userRepo{db: db}
}

func (r *userRepo) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
    var user domain.User
    if err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error; err != nil {
        return nil, err
    }
    return &user, nil
}

func (r *userRepo) FindByID(ctx context.Context, id string) (*domain.User, error) {
    var user domain.User
    if err := r.db.WithContext(ctx).First(&user, "id = ?", id).Error; err != nil {
        return nil, err
    }
    return &user, nil
}

func (r *userRepo) Create(ctx context.Context, user *domain.User) error {
    return r.db.WithContext(ctx).Create(user).Error
}
```

**Step 3: Commit**

```bash
git add internal/repository/interfaces.go internal/repository/user_repo.go
git commit -m "feat: add UserRepository with FindByEmail, FindByID, Create"
```

---

### Task 4: Extend Existing Repos with List + Pagination

**Files:**
- Modify: `internal/repository/interfaces.go`
- Modify: `internal/repository/tenant_repo.go`
- Modify: `internal/repository/session_repo.go`
- Modify: `internal/repository/contact_repo.go`
- Modify: `internal/repository/message_repo.go`

**Step 1: Add a shared Pagination type and List methods to interfaces**

Add to `interfaces.go`:
```go
type PaginationParams struct {
    Page  int
    Limit int
}

type PaginatedResult[T any] struct {
    Data  []T   `json:"data"`
    Total int64 `json:"total"`
    Page  int   `json:"page"`
    Limit int   `json:"limit"`
}
```

Add `List` to each interface:
```go
// TenantRepository
List(ctx context.Context, params PaginationParams) (*PaginatedResult[domain.Tenant], error)
Create(ctx context.Context, tenant *domain.Tenant) error
Update(ctx context.Context, tenant *domain.Tenant) error
Delete(ctx context.Context, id string) error

// SessionRepository
List(ctx context.Context, tenantID string, state string, params PaginationParams) (*PaginatedResult[domain.Session], error)
FindByID(ctx context.Context, id string) (*domain.Session, error)

// ContactRepository
List(ctx context.Context, tenantID string, params PaginationParams) (*PaginatedResult[domain.Contact], error)

// MessageRepository (already has FindBySession, no changes needed)
```

**Step 2: Implement List in tenant_repo.go**

Add methods:
```go
func (r *tenantRepo) List(ctx context.Context, params repository.PaginationParams) (*repository.PaginatedResult[domain.Tenant], error) {
    var tenants []domain.Tenant
    var total int64

    r.db.WithContext(ctx).Model(&domain.Tenant{}).Count(&total)

    offset := (params.Page - 1) * params.Limit
    if err := r.db.WithContext(ctx).Order("created_at DESC").Offset(offset).Limit(params.Limit).Find(&tenants).Error; err != nil {
        return nil, err
    }

    return &repository.PaginatedResult[domain.Tenant]{
        Data: tenants, Total: total, Page: params.Page, Limit: params.Limit,
    }, nil
}

func (r *tenantRepo) Create(ctx context.Context, tenant *domain.Tenant) error {
    return r.db.WithContext(ctx).Create(tenant).Error
}

func (r *tenantRepo) Update(ctx context.Context, tenant *domain.Tenant) error {
    return r.db.WithContext(ctx).Save(tenant).Error
}

func (r *tenantRepo) Delete(ctx context.Context, id string) error {
    return r.db.WithContext(ctx).Delete(&domain.Tenant{}, "id = ?", id).Error
}
```

**Step 3: Implement List in session_repo.go**

```go
func (r *sessionRepo) List(ctx context.Context, tenantID string, state string, params repository.PaginationParams) (*repository.PaginatedResult[domain.Session], error) {
    var sessions []domain.Session
    var total int64

    q := r.db.WithContext(ctx).Model(&domain.Session{})
    if tenantID != "" {
        q = q.Where("tenant_id = ?", tenantID)
    }
    if state != "" {
        q = q.Where("state = ?", state)
    }

    q.Count(&total)

    offset := (params.Page - 1) * params.Limit
    if err := q.Preload("Contact").Order("updated_at DESC").Offset(offset).Limit(params.Limit).Find(&sessions).Error; err != nil {
        return nil, err
    }

    return &repository.PaginatedResult[domain.Session]{
        Data: sessions, Total: total, Page: params.Page, Limit: params.Limit,
    }, nil
}

func (r *sessionRepo) FindByID(ctx context.Context, id string) (*domain.Session, error) {
    var session domain.Session
    if err := r.db.WithContext(ctx).Preload("Contact").Preload("Tenant").First(&session, "id = ?", id).Error; err != nil {
        return nil, err
    }
    return &session, nil
}
```

**Step 4: Implement List in contact_repo.go**

```go
func (r *contactRepo) List(ctx context.Context, tenantID string, params repository.PaginationParams) (*repository.PaginatedResult[domain.Contact], error) {
    var contacts []domain.Contact
    var total int64

    q := r.db.WithContext(ctx).Model(&domain.Contact{})
    if tenantID != "" {
        q = q.Where("tenant_id = ?", tenantID)
    }

    q.Count(&total)

    offset := (params.Page - 1) * params.Limit
    if err := q.Order("created_at DESC").Offset(offset).Limit(params.Limit).Find(&contacts).Error; err != nil {
        return nil, err
    }

    return &repository.PaginatedResult[domain.Contact]{
        Data: contacts, Total: total, Page: params.Page, Limit: params.Limit,
    }, nil
}
```

**Step 5: Build and verify**

```bash
cd c:/Users/yuji/blackonix && go build ./...
```

**Step 6: Commit**

```bash
git add internal/repository/
git commit -m "feat: add List with pagination to all repositories"
```

---

### Task 5: JWT Auth Middleware

**Files:**
- Create: `internal/middleware/auth.go`

**Step 1: Create the JWT middleware**

Uses `golang.org/x/crypto` (already in go.mod) for bcrypt, and a simple HMAC-SHA256 JWT implementation (no external JWT library needed for this scope).

```go
package middleware

import (
    "context"
    "crypto/hmac"
    "crypto/sha256"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "strings"
    "time"

    "blackonix/internal/repository"
    "github.com/gofiber/fiber/v2"
)

type contextKey string

const UserContextKey contextKey = "user"

type JWTClaims struct {
    UserID   string `json:"user_id"`
    Role     string `json:"role"`
    TenantID string `json:"tenant_id,omitempty"`
    Exp      int64  `json:"exp"`
}

func GenerateJWT(secret string, claims JWTClaims) (string, error) {
    claims.Exp = time.Now().Add(24 * time.Hour).Unix()

    header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))

    claimsJSON, err := json.Marshal(claims)
    if err != nil {
        return "", err
    }
    payload := base64.RawURLEncoding.EncodeToString(claimsJSON)

    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write([]byte(header + "." + payload))
    signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

    return header + "." + payload + "." + signature, nil
}

func ValidateJWT(secret, tokenStr string) (*JWTClaims, error) {
    parts := strings.Split(tokenStr, ".")
    if len(parts) != 3 {
        return nil, fmt.Errorf("invalid token format")
    }

    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write([]byte(parts[0] + "." + parts[1]))
    expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

    if !hmac.Equal([]byte(parts[2]), []byte(expectedSig)) {
        return nil, fmt.Errorf("invalid token signature")
    }

    claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
    if err != nil {
        return nil, fmt.Errorf("decode claims: %w", err)
    }

    var claims JWTClaims
    if err := json.Unmarshal(claimsJSON, &claims); err != nil {
        return nil, fmt.Errorf("unmarshal claims: %w", err)
    }

    if time.Now().Unix() > claims.Exp {
        return nil, fmt.Errorf("token expired")
    }

    return &claims, nil
}

func RequireAuth(secret string, userRepo repository.UserRepository) fiber.Handler {
    return func(c *fiber.Ctx) error {
        auth := c.Get("Authorization")
        if !strings.HasPrefix(auth, "Bearer ") {
            return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing token"})
        }

        token := strings.TrimPrefix(auth, "Bearer ")
        claims, err := ValidateJWT(secret, token)
        if err != nil {
            return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token"})
        }

        user, err := userRepo.FindByID(context.Background(), claims.UserID)
        if err != nil {
            return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "user not found"})
        }

        c.Locals("user", user)
        c.Locals("claims", claims)
        return c.Next()
    }
}

func RequireAdmin() fiber.Handler {
    return func(c *fiber.Ctx) error {
        claims, ok := c.Locals("claims").(*JWTClaims)
        if !ok || claims.Role != "ADMIN" {
            return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "admin access required"})
        }
        return c.Next()
    }
}
```

**Step 2: Build and verify**

```bash
go build ./...
```

**Step 3: Commit**

```bash
git add internal/middleware/auth.go
git commit -m "feat: add JWT auth middleware with HMAC-SHA256"
```

---

### Task 6: API Handler — Auth Endpoints

**Files:**
- Create: `internal/handlers/api.go`

**Step 1: Create the API handler with login and me endpoints**

```go
package handlers

import (
    "context"
    "strconv"

    "blackonix/internal/domain"
    "blackonix/internal/middleware"
    "blackonix/internal/repository"

    "github.com/gofiber/fiber/v2"
    "golang.org/x/crypto/bcrypt"
)

type APIHandler struct {
    userRepo    repository.UserRepository
    tenantRepo  repository.TenantRepository
    sessionRepo repository.SessionRepository
    contactRepo repository.ContactRepository
    messageRepo repository.MessageRepository
    jwtSecret   string
}

type APIHandlerConfig struct {
    UserRepo    repository.UserRepository
    TenantRepo  repository.TenantRepository
    SessionRepo repository.SessionRepository
    ContactRepo repository.ContactRepository
    MessageRepo repository.MessageRepository
    JWTSecret   string
}

func NewAPIHandler(cfg APIHandlerConfig) *APIHandler {
    return &APIHandler{
        userRepo:    cfg.UserRepo,
        tenantRepo:  cfg.TenantRepo,
        sessionRepo: cfg.SessionRepo,
        contactRepo: cfg.ContactRepo,
        messageRepo: cfg.MessageRepo,
        jwtSecret:   cfg.JWTSecret,
    }
}

func (h *APIHandler) Login(c *fiber.Ctx) error {
    var body struct {
        Email    string `json:"email"`
        Password string `json:"password"`
    }
    if err := c.BodyParser(&body); err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
    }

    user, err := h.userRepo.FindByEmail(context.Background(), body.Email)
    if err != nil {
        return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid credentials"})
    }

    if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(body.Password)); err != nil {
        return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid credentials"})
    }

    tenantID := ""
    if user.TenantID != nil {
        tenantID = *user.TenantID
    }

    token, err := middleware.GenerateJWT(h.jwtSecret, middleware.JWTClaims{
        UserID:   user.ID,
        Role:     string(user.Role),
        TenantID: tenantID,
    })
    if err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to generate token"})
    }

    return c.JSON(fiber.Map{"token": token})
}

func (h *APIHandler) Me(c *fiber.Ctx) error {
    user := c.Locals("user").(*domain.User)
    return c.JSON(user)
}

// parsePagination extracts page/limit from query params with defaults.
func parsePagination(c *fiber.Ctx) repository.PaginationParams {
    page, _ := strconv.Atoi(c.Query("page", "1"))
    limit, _ := strconv.Atoi(c.Query("limit", "20"))
    if page < 1 {
        page = 1
    }
    if limit < 1 || limit > 100 {
        limit = 20
    }
    return repository.PaginationParams{Page: page, Limit: limit}
}
```

**Step 2: Commit**

```bash
git add internal/handlers/api.go
git commit -m "feat: add API handler with login and me endpoints"
```

---

### Task 7: API Handler — CRUD Endpoints

**Files:**
- Modify: `internal/handlers/api.go`

**Step 1: Add tenant CRUD, session list, contact list, dashboard stats**

Append to `api.go`:

```go
// --- Tenants ---

func (h *APIHandler) ListTenants(c *fiber.Ctx) error {
    result, err := h.tenantRepo.List(context.Background(), parsePagination(c))
    if err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list tenants"})
    }
    return c.JSON(result)
}

func (h *APIHandler) GetTenant(c *fiber.Ctx) error {
    tenant, err := h.tenantRepo.FindByID(context.Background(), c.Params("id"))
    if err != nil {
        return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "tenant not found"})
    }
    return c.JSON(tenant)
}

func (h *APIHandler) CreateTenant(c *fiber.Ctx) error {
    var tenant domain.Tenant
    if err := c.BodyParser(&tenant); err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
    }
    if err := h.tenantRepo.Create(context.Background(), &tenant); err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create tenant"})
    }
    return c.Status(fiber.StatusCreated).JSON(tenant)
}

func (h *APIHandler) UpdateTenant(c *fiber.Ctx) error {
    existing, err := h.tenantRepo.FindByID(context.Background(), c.Params("id"))
    if err != nil {
        return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "tenant not found"})
    }
    if err := c.BodyParser(existing); err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
    }
    if err := h.tenantRepo.Update(context.Background(), existing); err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update tenant"})
    }
    return c.JSON(existing)
}

func (h *APIHandler) DeleteTenant(c *fiber.Ctx) error {
    if err := h.tenantRepo.Delete(context.Background(), c.Params("id")); err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete tenant"})
    }
    return c.SendStatus(fiber.StatusNoContent)
}

// --- Sessions ---

func (h *APIHandler) ListSessions(c *fiber.Ctx) error {
    claims := c.Locals("claims").(*middleware.JWTClaims)
    tenantID := c.Query("tenant_id")
    if claims.Role != "ADMIN" {
        tenantID = claims.TenantID
    }
    state := c.Query("state")

    result, err := h.sessionRepo.List(context.Background(), tenantID, state, parsePagination(c))
    if err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list sessions"})
    }
    return c.JSON(result)
}

func (h *APIHandler) GetSession(c *fiber.Ctx) error {
    session, err := h.sessionRepo.FindByID(context.Background(), c.Params("id"))
    if err != nil {
        return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "session not found"})
    }
    return c.JSON(session)
}

func (h *APIHandler) GetSessionMessages(c *fiber.Ctx) error {
    limit, _ := strconv.Atoi(c.Query("limit", "100"))
    if limit < 1 || limit > 500 {
        limit = 100
    }
    messages, err := h.messageRepo.FindBySession(context.Background(), c.Params("id"), limit)
    if err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list messages"})
    }
    return c.JSON(messages)
}

func (h *APIHandler) ChangeSessionState(c *fiber.Ctx) error {
    var body struct {
        State string `json:"state"`
    }
    if err := c.BodyParser(&body); err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
    }
    session, err := h.sessionRepo.FindByID(context.Background(), c.Params("id"))
    if err != nil {
        return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "session not found"})
    }
    session.State = domain.SessionState(body.State)
    if err := h.sessionRepo.Update(context.Background(), session); err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update session"})
    }
    return c.JSON(session)
}

// --- Contacts ---

func (h *APIHandler) ListContacts(c *fiber.Ctx) error {
    claims := c.Locals("claims").(*middleware.JWTClaims)
    tenantID := c.Query("tenant_id")
    if claims.Role != "ADMIN" {
        tenantID = claims.TenantID
    }

    result, err := h.contactRepo.List(context.Background(), tenantID, parsePagination(c))
    if err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list contacts"})
    }
    return c.JSON(result)
}

func (h *APIHandler) GetContact(c *fiber.Ctx) error {
    contact, err := h.contactRepo.FindByID(context.Background(), c.Params("id"))
    if err != nil {
        return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "contact not found"})
    }
    return c.JSON(contact)
}

// --- Dashboard ---

func (h *APIHandler) DashboardStats(c *fiber.Ctx) error {
    claims := c.Locals("claims").(*middleware.JWTClaims)
    tenantID := c.Query("tenant_id")
    if claims.Role != "ADMIN" {
        tenantID = claims.TenantID
    }

    allSessions, _ := h.sessionRepo.List(context.Background(), tenantID, "", repository.PaginationParams{Page: 1, Limit: 1})
    botSessions, _ := h.sessionRepo.List(context.Background(), tenantID, "BOT", repository.PaginationParams{Page: 1, Limit: 1})

    totalSessions := int64(0)
    botCount := int64(0)
    if allSessions != nil {
        totalSessions = allSessions.Total
    }
    if botSessions != nil {
        botCount = botSessions.Total
    }

    humanCount := totalSessions - botCount
    ratio := float64(0)
    if totalSessions > 0 {
        ratio = float64(botCount) / float64(totalSessions) * 100
    }

    return c.JSON(fiber.Map{
        "total_sessions":   totalSessions,
        "bot_sessions":     botCount,
        "human_sessions":   humanCount,
        "bot_vs_human_pct": ratio,
    })
}
```

**Step 2: Build and verify**

```bash
go build ./...
```

**Step 3: Commit**

```bash
git add internal/handlers/api.go
git commit -m "feat: add tenant CRUD, session/contact list, dashboard stats endpoints"
```

---

### Task 8: Wire API Routes into main.go

**Files:**
- Modify: `cmd/server/main.go`

**Step 1: Add CORS, user repo, auth middleware, and API routes**

Add imports for `middleware` package and `fiber/v2/middleware/cors`.

After existing repo setup, add:
```go
userRepo := repository.NewUserRepository(db)
```

After Fiber app setup, add CORS:
```go
app.Use(cors.New(cors.Config{
    AllowOrigins: cfg.CORSOrigin,
    AllowHeaders: "Origin, Content-Type, Authorization",
    AllowMethods: "GET, POST, PUT, DELETE, OPTIONS",
}))
```

Create API handler:
```go
apiHandler := handlers.NewAPIHandler(handlers.APIHandlerConfig{
    UserRepo:    userRepo,
    TenantRepo:  tenantRepo,
    SessionRepo: sessionRepo,
    ContactRepo: contactRepo,
    MessageRepo: messageRepo,
    JWTSecret:   cfg.JWTSecret,
})
```

Add routes:
```go
// Auth (public)
api := app.Group("/api/v1")
api.Post("/auth/login", apiHandler.Login)

// Protected routes
protected := api.Use(middleware.RequireAuth(cfg.JWTSecret, userRepo))
protected.Get("/auth/me", apiHandler.Me)
protected.Get("/dashboard/stats", apiHandler.DashboardStats)

// Tenants (admin only)
tenants := protected.Group("/tenants", middleware.RequireAdmin())
tenants.Get("/", apiHandler.ListTenants)
tenants.Get("/:id", apiHandler.GetTenant)
tenants.Post("/", apiHandler.CreateTenant)
tenants.Put("/:id", apiHandler.UpdateTenant)
tenants.Delete("/:id", apiHandler.DeleteTenant)

// Sessions
protected.Get("/sessions", apiHandler.ListSessions)
protected.Get("/sessions/:id", apiHandler.GetSession)
protected.Get("/sessions/:id/messages", apiHandler.GetSessionMessages)
protected.Post("/sessions/:id/state", apiHandler.ChangeSessionState)

// Contacts
protected.Get("/contacts", apiHandler.ListContacts)
protected.Get("/contacts/:id", apiHandler.GetContact)
```

**Step 2: Update seed script to create admin user**

Modify `cmd/seed/main.go` to also create a default admin user with bcrypt-hashed password.

```go
import "golang.org/x/crypto/bcrypt"

hash, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
db.Exec("INSERT INTO users (email, password_hash, role) VALUES (?, ?, ?) ON CONFLICT (email) DO NOTHING",
    "admin@blackonix.io", string(hash), "ADMIN")
```

**Step 3: Build and verify**

```bash
go build ./...
```

**Step 4: Commit**

```bash
git add cmd/server/main.go cmd/seed/main.go
git commit -m "feat: wire API routes with JWT auth, CORS, and admin seed"
```

---

## Phase 2: Next.js Frontend

### Task 9: Scaffold Next.js Project

**Step 1: Create Next.js app in web/**

```bash
cd c:/Users/yuji/blackonix
npx create-next-app@latest web --typescript --tailwind --eslint --app --src-dir=false --import-alias="@/*" --no-turbopack
```

**Step 2: Initialize shadcn/ui**

```bash
cd web
npx shadcn@latest init -d
```

**Step 3: Add shadcn components we need**

```bash
npx shadcn@latest add button card input label table badge separator dropdown-menu sheet avatar
```

**Step 4: Create .env.local**

```
NEXT_PUBLIC_API_URL=http://localhost:3000
```

**Step 5: Commit**

```bash
cd c:/Users/yuji/blackonix
git add web/
git commit -m "feat: scaffold Next.js 15 + shadcn/ui in web/"
```

---

### Task 10: API Client + Types

**Files:**
- Create: `web/lib/types.ts`
- Create: `web/lib/api.ts`

**Step 1: Create TypeScript types mirroring Go domain models**

`web/lib/types.ts`:
```typescript
export interface Tenant {
  id: string;
  name: string;
  waba_id: string;
  meta_token: string;
  rocketchat_url: string;
  rocketchat_token: string;
  created_at: string;
  updated_at: string;
}

export interface Contact {
  id: string;
  tenant_id: string;
  phone_number: string;
  name: string;
  created_at: string;
  updated_at: string;
}

export interface Session {
  id: string;
  tenant_id: string;
  contact_id: string;
  state: "BOT" | "HUMAN";
  active_department: string;
  created_at: string;
  updated_at: string;
  contact?: Contact;
  tenant?: Tenant;
}

export interface Message {
  id: string;
  tenant_id: string;
  session_id: string;
  contact_id: string;
  direction: "INBOUND" | "OUTBOUND";
  body: string;
  media_url: string;
  created_at: string;
}

export interface User {
  id: string;
  email: string;
  role: "ADMIN" | "TENANT";
  tenant_id?: string;
  created_at: string;
}

export interface PaginatedResult<T> {
  data: T[];
  total: number;
  page: number;
  limit: number;
}

export interface DashboardStats {
  total_sessions: number;
  bot_sessions: number;
  human_sessions: number;
  bot_vs_human_pct: number;
}
```

**Step 2: Create typed API client**

`web/lib/api.ts`:
```typescript
import type {
  Tenant, Contact, Session, Message, User,
  PaginatedResult, DashboardStats,
} from "./types";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:3000";

async function apiFetch<T>(path: string, options: RequestInit = {}): Promise<T> {
  const token = typeof window !== "undefined"
    ? document.cookie.match(/token=([^;]+)/)?.[1]
    : "";

  const res = await fetch(`${API_URL}${path}`, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...options.headers,
    },
  });

  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || res.statusText);
  }

  if (res.status === 204) return undefined as T;
  return res.json();
}

// Auth
export const login = (email: string, password: string) =>
  apiFetch<{ token: string }>("/api/v1/auth/login", {
    method: "POST",
    body: JSON.stringify({ email, password }),
  });

export const getMe = () => apiFetch<User>("/api/v1/auth/me");

// Dashboard
export const getDashboardStats = (tenantId?: string) =>
  apiFetch<DashboardStats>(`/api/v1/dashboard/stats${tenantId ? `?tenant_id=${tenantId}` : ""}`);

// Tenants
export const listTenants = (page = 1, limit = 20) =>
  apiFetch<PaginatedResult<Tenant>>(`/api/v1/tenants?page=${page}&limit=${limit}`);
export const getTenant = (id: string) => apiFetch<Tenant>(`/api/v1/tenants/${id}`);
export const createTenant = (data: Partial<Tenant>) =>
  apiFetch<Tenant>("/api/v1/tenants", { method: "POST", body: JSON.stringify(data) });
export const updateTenant = (id: string, data: Partial<Tenant>) =>
  apiFetch<Tenant>(`/api/v1/tenants/${id}`, { method: "PUT", body: JSON.stringify(data) });
export const deleteTenant = (id: string) =>
  apiFetch<void>(`/api/v1/tenants/${id}`, { method: "DELETE" });

// Sessions
export const listSessions = (params: { tenant_id?: string; state?: string; page?: number; limit?: number } = {}) => {
  const q = new URLSearchParams();
  if (params.tenant_id) q.set("tenant_id", params.tenant_id);
  if (params.state) q.set("state", params.state);
  q.set("page", String(params.page || 1));
  q.set("limit", String(params.limit || 20));
  return apiFetch<PaginatedResult<Session>>(`/api/v1/sessions?${q}`);
};
export const getSession = (id: string) => apiFetch<Session>(`/api/v1/sessions/${id}`);
export const getSessionMessages = (id: string, limit = 100) =>
  apiFetch<Message[]>(`/api/v1/sessions/${id}/messages?limit=${limit}`);
export const changeSessionState = (id: string, state: string) =>
  apiFetch<Session>(`/api/v1/sessions/${id}/state`, { method: "POST", body: JSON.stringify({ state }) });

// Contacts
export const listContacts = (params: { tenant_id?: string; page?: number; limit?: number } = {}) => {
  const q = new URLSearchParams();
  if (params.tenant_id) q.set("tenant_id", params.tenant_id);
  q.set("page", String(params.page || 1));
  q.set("limit", String(params.limit || 20));
  return apiFetch<PaginatedResult<Contact>>(`/api/v1/contacts?${q}`);
};
export const getContact = (id: string) => apiFetch<Contact>(`/api/v1/contacts/${id}`);
```

**Step 3: Commit**

```bash
git add web/lib/
git commit -m "feat: add typed API client and domain types for frontend"
```

---

### Task 11: Auth — Login Page + Middleware

**Files:**
- Create: `web/app/login/page.tsx`
- Create: `web/middleware.ts`

**Step 1: Create login page**

`web/app/login/page.tsx` — client component with email/password form that calls `login()`, stores token as cookie, redirects to `/dashboard`.

**Step 2: Create Next.js middleware**

`web/middleware.ts` — checks for `token` cookie on all routes except `/login`. Redirects to `/login` if missing.

**Step 3: Commit**

```bash
git add web/app/login/ web/middleware.ts
git commit -m "feat: add login page and auth middleware"
```

---

### Task 12: Layout — Sidebar + Topbar

**Files:**
- Create: `web/components/sidebar.tsx`
- Create: `web/components/topbar.tsx`
- Modify: `web/app/layout.tsx`

**Step 1: Create sidebar** — Navigation links: Dashboard, Tenants, Sessions, Contacts. Active state styling.

**Step 2: Create topbar** — Shows current user email, tenant selector dropdown for admins, dark mode toggle, logout button.

**Step 3: Update root layout** — Wrap children with sidebar + topbar layout. Login page should NOT show sidebar (check pathname).

**Step 4: Commit**

```bash
git add web/components/ web/app/layout.tsx
git commit -m "feat: add sidebar navigation and topbar with dark mode"
```

---

### Task 13: Dashboard Page

**Files:**
- Create: `web/app/dashboard/page.tsx`
- Modify: `web/app/page.tsx` (redirect to /dashboard)

**Step 1: Create dashboard** — Server Component that fetches `getDashboardStats()`. Renders 4 `Card` components with stat values. Root page.tsx redirects to /dashboard.

**Step 2: Commit**

```bash
git add web/app/dashboard/ web/app/page.tsx
git commit -m "feat: add dashboard page with stats cards"
```

---

### Task 14: Reusable DataTable Component

**Files:**
- Create: `web/components/data-table.tsx`

**Step 1: Create generic data table** — Takes columns config + data array + pagination info. Uses shadcn `Table` component. Includes page navigation buttons.

**Step 2: Commit**

```bash
git add web/components/data-table.tsx
git commit -m "feat: add reusable DataTable component with pagination"
```

---

### Task 15: Tenants Page

**Files:**
- Create: `web/app/tenants/page.tsx`
- Create: `web/app/tenants/[id]/page.tsx`

**Step 1: Tenant list page** — DataTable with columns: Name, WABA ID, Created. Click row navigates to detail. "New Tenant" button.

**Step 2: Tenant detail page** — Form with all tenant fields. Save and Delete buttons.

**Step 3: Commit**

```bash
git add web/app/tenants/
git commit -m "feat: add tenants list and detail pages"
```

---

### Task 16: Sessions Page + Conversation Viewer

**Files:**
- Create: `web/app/sessions/page.tsx`
- Create: `web/app/sessions/[id]/page.tsx`

**Step 1: Session list page** — DataTable with columns: Contact name/phone, State badge (BOT green / HUMAN blue), Last Activity. Filter dropdowns for tenant and state.

**Step 2: Conversation viewer** — Fetches messages for session. Renders WhatsApp-style bubbles (inbound gray left-aligned, outbound green right-aligned). Shows state toggle button (BOT↔HUMAN).

**Step 3: Commit**

```bash
git add web/app/sessions/
git commit -m "feat: add sessions list and conversation viewer"
```

---

### Task 17: Contacts Page

**Files:**
- Create: `web/app/contacts/page.tsx`

**Step 1: Contact list** — DataTable with columns: Name, Phone, Tenant, Created. Filter by tenant.

**Step 2: Commit**

```bash
git add web/app/contacts/
git commit -m "feat: add contacts list page"
```

---

### Task 18: Final Build + Integration Test

**Step 1: Build Go backend**

```bash
cd c:/Users/yuji/blackonix && go build ./...
```

**Step 2: Build Next.js frontend**

```bash
cd web && npm run build
```

**Step 3: Run seed to create admin user**

```bash
cd c:/Users/yuji/blackonix && go run cmd/seed/main.go
```

**Step 4: Start both servers and test login flow**

Terminal 1: `go run cmd/server/main.go`
Terminal 2: `cd web && npm run dev`

Open http://localhost:3001, login with admin@blackonix.io / admin123, verify dashboard loads.

**Step 5: Commit everything remaining**

```bash
git add -A
git commit -m "feat: complete BlackOnix dashboard frontend integration"
```

---

## Summary

| Phase | Tasks | What it builds |
|-------|-------|---------------|
| Phase 1 (Go) | Tasks 1-8 | User model, JWT auth, REST API, CORS, routes |
| Phase 2 (Next.js) | Tasks 9-18 | Scaffold, API client, login, layout, all pages |
