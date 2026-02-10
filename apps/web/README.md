# Kuberan Web

The frontend application for Kuberan, built with Next.js 15 (App Router), React 19, and Tailwind CSS v4.

## Tech Stack

| Technology | Purpose |
|---|---|
| Next.js 15 (App Router) | Framework with Turbopack dev server |
| React 19 | UI library |
| TypeScript (strict) | Type safety, never use `any` |
| Tailwind CSS v4 | Styling |
| ShadCN UI (`new-york`) | Component library (Radix UI + Lucide icons) |
| @tanstack/react-query v5 | Server state management with query key factories |
| react-hook-form + zod | Form handling and schema validation |
| Recharts | Dashboard charts (pie, bar, area) |
| next-themes | Light/dark/system theme switching |
| Sonner | Toast notifications |
| pnpm | Package manager |

## Development

```bash
# Start dev server (standalone)
pnpm dev

# Or via Docker Compose from repo root
npm run dev
```

The dev server runs at http://localhost:3000 with Turbopack for fast refresh.

## Scripts

| Command | Description |
|---|---|
| `pnpm dev` | Start dev server |
| `pnpm build` | Production build |
| `pnpm start` | Start production server |
| `pnpm lint` | Run ESLint |

## Project Structure

```
src/
├── app/
│   ├── (auth)/                   # Auth route group (login, register)
│   │   └── layout.tsx            # Centered card layout, redirects authenticated users
│   ├── (dashboard)/              # Dashboard route group (all protected pages)
│   │   ├── layout.tsx            # Sidebar + header layout, auth guard
│   │   ├── page.tsx              # Dashboard home
│   │   ├── accounts/             # Accounts list + [id] detail
│   │   ├── transactions/         # Cross-account transactions
│   │   ├── categories/           # Category management
│   │   ├── budgets/              # Budget cards with progress
│   │   ├── investments/          # Portfolio overview + [id] detail
│   │   └── securities/           # Securities browse + [id] detail
│   ├── layout.tsx                # Root layout (providers chain)
│   └── globals.css               # Tailwind CSS v4 + theme variables
├── components/
│   ├── ui/                       # ShadCN UI primitives (24 components)
│   ├── layout/                   # App sidebar, header, theme toggle
│   ├── accounts/                 # Account dialogs (create, edit)
│   ├── transactions/             # Transaction dialogs (create, edit)
│   ├── categories/               # Category dialogs (create, edit, delete)
│   ├── budgets/                  # Budget dialogs (create, edit, delete)
│   ├── investments/              # Investment action dialogs (add, buy, sell, dividend, split)
│   └── dashboard/                # Dashboard charts (expenditure, income/expenses, spending trend)
├── hooks/                        # React Query hooks (one per domain, with query key factories)
├── providers/                    # ThemeProvider, QueryProvider, AuthProvider
├── lib/
│   ├── api-client.ts             # HTTP client with auto token refresh
│   ├── auth.ts                   # JWT parsing, token storage, auth cookies
│   ├── format.ts                 # Currency (cents->display), date, percentage formatters
│   └── utils.ts                  # cn() Tailwind utility
├── types/
│   ├── models.ts                 # Domain model types matching backend
│   └── api.ts                    # API request/response DTOs
└── middleware.ts                  # Next.js middleware for cookie-based route protection
```

## Key Patterns

- **Route groups**: `(auth)` for public pages (login, register), `(dashboard)` for protected pages with sidebar layout
- **Provider chain**: Root layout wraps `ThemeProvider` > `QueryProvider` > `AuthProvider` > `Toaster`
- **Data fetching**: All API calls go through `lib/api-client.ts`, consumed by React Query hooks in `src/hooks/`. Each hook file exports a query key factory for structured cache management
- **Auth flow**: JWT access tokens (localStorage) + auth flag cookie (for Next.js middleware). Auto-refresh on 401 with concurrent request deduplication
- **Component organization**: ShadCN primitives in `ui/`, feature-specific dialogs in domain folders (`accounts/`, `transactions/`, etc.)
- **Forms**: react-hook-form with zod schemas for validation, ShadCN Form components for UI
- **Theming**: CSS variables with oklch color space, toggled via `next-themes` (light/dark/system)
