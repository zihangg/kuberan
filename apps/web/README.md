# Kuberan Web

The frontend application for Kuberan, built with Next.js 15 (App Router), React 19, and Tailwind CSS v4.

> **Status**: Scaffolded with `create-next-app`. Custom UI development has not yet started.

## Tech Stack

- **Next.js** 15 with App Router and Turbopack
- **React** 19
- **TypeScript** (strict mode)
- **Tailwind CSS** v4
- **Font**: Geist (via `next/font`)

## Development

```bash
# Start dev server (standalone)
npm run dev

# Or via Docker Compose from repo root
npm run dev
```

The dev server runs at http://localhost:3000 with Turbopack for fast refresh.

## Scripts

| Command         | Description              |
|-----------------|--------------------------|
| `npm run dev`   | Start dev server         |
| `npm run build` | Production build         |
| `npm run start` | Start production server  |
| `npm run lint`  | Run ESLint               |

## Project Structure

```
src/
└── app/
    ├── layout.tsx      # Root layout (Geist fonts)
    ├── page.tsx        # Home page
    ├── globals.css     # Tailwind CSS v4 styles
    └── favicon.ico
```

## Planned Architecture

The frontend will follow these patterns (not yet implemented):

- **Data Fetching**: @tanstack/react-query
- **Components**: ShadCN UI with atomic design (atoms/molecules/organisms/templates/pages)
- **API Layer**: Service files called from custom hooks in `src/hooks/`
- **Server components** by default, client components only when interactivity is needed
- **Strict TypeScript** -- never use `any`
