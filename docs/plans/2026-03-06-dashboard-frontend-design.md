# BlackOnix Dashboard Frontend Design

**Date:** 2026-03-06
**Status:** Approved

## Summary

Admin dashboard for BlackOnix built with Next.js 15 (App Router) + shadcn/ui, fully decoupled from the Go backend. Communicates exclusively via REST API. Lives in `web/` within the monorepo but has zero coupling to Go code.

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Framework | Next.js 15 App Router | Server Components for fast loads, standard for new projects |
| UI Library | shadcn/ui + Tailwind | Copy-paste components, no heavy dependency, dark mode native |
| Auth | JWT with httpOnly cookies | More secure than localStorage, supports role-based access |
| Users | Super-admin + tenant-scoped | Super-admin sees all tenants, tenant users see only their data |
| Project location | `web/` in monorepo | Keeps deployment simple while maintaining full separation |
| API communication | REST via `NEXT_PUBLIC_API_URL` | Frontend only knows the API URL, no Go imports |

## Project Structure

```
web/
├── app/
│   ├── layout.tsx              # Root layout (sidebar + topbar)
│   ├── page.tsx                # Redirect to /dashboard
│   ├── login/
│   │   └── page.tsx            # Login form
│   ├── dashboard/
│   │   └── page.tsx            # Stats cards + activity chart
│   ├── tenants/
│   │   ├── page.tsx            # Tenant list table
│   │   └── [id]/
│   │       └── page.tsx        # Tenant detail + edit form
│   ├── sessions/
│   │   ├── page.tsx            # Session list (filterable)
│   │   └── [id]/
│   │       └── page.tsx        # Conversation viewer (message thread)
│   └── contacts/
│       └── page.tsx            # Contact list (filterable)
├── components/
│   ├── ui/                     # shadcn/ui generated components
│   ├── sidebar.tsx
│   ├── topbar.tsx
│   └── data-table.tsx          # Reusable table with sorting/pagination
├── lib/
│   ├── api.ts                  # Typed fetch wrapper for Go backend
│   ├── auth.ts                 # JWT cookie helpers
│   └── types.ts                # TypeScript types mirroring Go domain models
├── middleware.ts                # Auth check on protected routes
├── .env.local
└── package.json
```

## REST API Contract (Go backend additions)

All routes under `/api/v1/`, protected by JWT `Authorization: Bearer` header.

### Auth
```
POST /api/v1/auth/login          → { email, password } → { token }
GET  /api/v1/auth/me             → current user info + role
```

### Dashboard
```
GET  /api/v1/dashboard/stats     → { totalSessions, activeSessions, messagesToday, botVsHumanRatio }
```

### Tenants (super-admin only)
```
GET    /api/v1/tenants           → paginated tenant list
GET    /api/v1/tenants/:id       → tenant detail
POST   /api/v1/tenants           → create tenant
PUT    /api/v1/tenants/:id       → update tenant
DELETE /api/v1/tenants/:id       → delete tenant
```

### Sessions
```
GET    /api/v1/sessions               → list (query: ?tenant_id=&state=&page=&limit=)
GET    /api/v1/sessions/:id           → session detail
GET    /api/v1/sessions/:id/messages  → message thread
POST   /api/v1/sessions/:id/state     → force state change (BOT↔HUMAN)
```

### Contacts
```
GET    /api/v1/contacts          → list (query: ?tenant_id=&page=&limit=)
GET    /api/v1/contacts/:id      → contact detail
```

## Go Backend Changes Required

1. **New `User` domain model** — `id`, `email`, `password_hash`, `role` (admin/tenant), `tenant_id` (nullable), timestamps
2. **User repository** — `FindByEmail`, `FindByID`, `Create`
3. **JWT middleware** — Fiber middleware that validates token, extracts user, sets in context
4. **API handler** — New `internal/handlers/api.go` with all REST endpoints
5. **Extended repository interfaces** — Add `List` with pagination to tenant, session, contact, message repos
6. **CORS middleware** — Allow requests from Next.js dev server origin

## Pages

### Dashboard
- 4 stat cards: Total Sessions, Active Now, Messages Today, Bot vs Human %
- Activity chart: messages per day (last 7 days)

### Tenants (super-admin)
- Data table: Name, WABA ID, Status, Created
- Detail page: edit form for all tenant fields

### Sessions
- Data table: Contact, Tenant, State (BOT/HUMAN badge), Message count, Last Activity
- Filters: tenant dropdown, state toggle
- Detail: WhatsApp-style conversation viewer (inbound left, outbound right)

### Contacts
- Data table: Name, Phone, Tenant, Sessions count, Last seen
- Filter by tenant

## UX
- Sidebar navigation (Dashboard, Tenants, Sessions, Contacts)
- Tenant selector in topbar for super-admins (filters all views)
- Dark mode toggle (shadcn/ui native)
- Reusable `DataTable` component with server-side pagination
