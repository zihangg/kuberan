# Database Setup

Kuberan supports two database backends in production. Choose the one that fits your setup.

| | Self-Hosted PostgreSQL | Supabase |
|---|---|---|
| **Data location** | Your VPS (Docker volume) | Supabase's cloud |
| **Setup complexity** | Low (runs in Docker) | Low (managed service) |
| **Free tier** | Unlimited | Limited (paused after inactivity) |
| **Backups** | Manual (pg_dump) or cron | Automatic (Supabase dashboard) |
| **Best for** | Full self-hosting, privacy-first | Managed convenience |

---

## Option A: Self-Hosted PostgreSQL

PostgreSQL runs as a Docker Compose service alongside the API and web app. Data is persisted in a named Docker volume on your VPS.

### 1. Configure `.env.prod`

Uncomment the Option A block and comment out (or remove) Option B:

```bash
COMPOSE_PROFILES=postgres
DB_HOST=postgres
DB_PORT=5432
DB_USER=kuberan
DB_PASSWORD=your_strong_password_here
DB_NAME=kuberan
DB_SSLMODE=disable
```

Generate a strong password:
```bash
openssl rand -hex 32
```

### 2. Deploy

No additional steps needed. The `postgres` service starts automatically because `COMPOSE_PROFILES=postgres` is set:

```bash
cd /opt/kuberan
./deploy/deploy.sh
```

The deploy script:
1. Starts the `postgres` container and waits for it to be healthy
2. Runs migrations directly against it (port 5432)
3. Starts the API and web services

### 3. Verify

```bash
# Check postgres container is running
docker compose -f docker-compose.prod.yml ps postgres

# Check API health (includes DB ping)
curl http://localhost:8080/api/health
```

### Data Location

Data is stored in a Docker named volume:
```bash
# Inspect volume location
docker volume inspect kuberan_postgres_data

# List volume contents (mounts at /var/lib/postgresql/data inside container)
docker compose -f docker-compose.prod.yml exec postgres ls /var/lib/postgresql/data
```

### Backups

**Manual backup:**
```bash
docker compose -f docker-compose.prod.yml exec postgres \
  pg_dump -U kuberan kuberan --no-owner --no-acl -Fc \
  > kuberan_backup_$(date +%Y%m%d_%H%M%S).dump
```

**Restore from backup:**
```bash
docker compose -f docker-compose.prod.yml exec -T postgres \
  pg_restore -U kuberan -d kuberan --no-owner --clean \
  < kuberan_backup_YYYYMMDD_HHMMSS.dump
```

**Automated daily backup (add to crontab):**
```bash
0 2 * * * cd /opt/kuberan && docker compose -f docker-compose.prod.yml exec -T postgres \
  pg_dump -U kuberan kuberan --no-owner --no-acl -Fc \
  > /opt/kuberan/backups/kuberan_$(date +\%Y\%m\%d).dump \
  && find /opt/kuberan/backups -name "*.dump" -mtime +30 -delete
```

---

## Option B: Supabase

Supabase provides a managed PostgreSQL instance with a web dashboard, automatic backups, and connection pooling via Supavisor.

### 1. Create a Supabase Project

1. Go to [supabase.com](https://supabase.com) and create a new project
2. Wait for the project to be provisioned

### 2. Get Your Credentials

In the Supabase dashboard:
1. Go to **Settings > Database**
2. Find **Connection string** section
3. Select **Transaction** mode (port 6543) for the app connection
4. Copy the credentials

### 3. Configure `.env.prod`

Keep the Option B block active (it is the default in `.env.prod.example`):

```bash
COMPOSE_PROFILES=
DB_HOST=db.xxxxxxxxxxxx.supabase.co
DB_PORT=6543
DB_USER=postgres.xxxxxxxxxxxx
DB_PASSWORD=your_supabase_db_password
DB_NAME=postgres
DB_SSLMODE=require
```

> **Why port 6543?** The app connects via Supavisor (connection pooler, transaction mode) for efficiency. The deploy script automatically uses port 5432 (direct connection) for migrations, since `golang-migrate` requires advisory locks that Supavisor's transaction mode does not support.

### 4. Deploy

```bash
cd /opt/kuberan
./deploy/deploy.sh
```

No local postgres container is started. Migrations run via the direct Supabase connection (port 5432), then the API connects via the pooler (port 6543).

### Backups

Supabase provides automatic daily backups on paid plans. For free plans:
- Go to **Supabase Dashboard > Database > Backups**
- Or use `pg_dump` with the direct connection (port 5432):

```bash
pg_dump "postgres://postgres.xxxx:PASSWORD@db.xxxx.supabase.co:5432/postgres" \
  --no-owner --no-acl -Fc -f kuberan_backup_$(date +%Y%m%d).dump
```

---

## Switching Between Options

If you want to move data from one backend to the other:

### Supabase → Self-Hosted

```bash
# 1. Dump from Supabase (use direct port 5432)
pg_dump "postgres://postgres.xxxx:PASSWORD@db.xxxx.supabase.co:5432/postgres" \
  --no-owner --no-acl -Fc -f kuberan_backup.dump

# 2. Update .env.prod to Option A (self-hosted) and redeploy
./deploy/deploy.sh

# 3. Restore into the local container
docker compose -f docker-compose.prod.yml exec -T postgres \
  pg_restore -U kuberan -d kuberan --no-owner --clean \
  < kuberan_backup.dump
```

### Self-Hosted → Supabase

```bash
# 1. Dump from local container
docker compose -f docker-compose.prod.yml exec -T postgres \
  pg_dump -U kuberan kuberan --no-owner --no-acl -Fc \
  > kuberan_backup.dump

# 2. Restore into Supabase (use direct port 5432)
pg_restore "postgres://postgres.xxxx:PASSWORD@db.xxxx.supabase.co:5432/postgres" \
  --no-owner --no-acl --clean -1 < kuberan_backup.dump

# 3. Update .env.prod to Option B (Supabase) and redeploy
./deploy/deploy.sh
```
