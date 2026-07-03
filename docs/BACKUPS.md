# Backups (Docker volumes)

Recommended schedule for **production**: daily backup + 7–14 day retention.  
Volume names are defined in [`docker-compose.yml`](../docker-compose.yml) (project `union_ai_apple`).

| Compose volume | Contents | Criticality |
|----------------|----------|-------------|
| `postgres_data` | Sessions, messages, feedback, analytics | **High** |
| `chroma_data` | RAG vector index (Chroma) | **High** (rebuildable from `data/`, but slow) |
| `bm25_data` | BM25 index | **High** (rebuildable from `data/`) |
| `uploads_data` | User photos in chat | Medium |
| `models` | CV weights `.pth` | Low (if no custom weights) |

---

## PostgreSQL

```bash
# Create dump (postgres container must be running)
docker compose -p union_ai_apple exec -T postgres \
  pg_dump -U gardener -d gardener -Fc > backups/postgres_$(date +%Y%m%d).dump

# Restore (caution: overwrites DB)
docker compose -p union_ai_apple exec -T postgres \
  pg_restore -U gardener -d gardener --clean --if-exists < backups/postgres_YYYYMMDD.dump
```

Alternative without `exec`: `pg_dump` from host via `DATABASE_URL` / port `5432` (if exposed).

---

## Chroma and BM25

Indexes live in classifier service volumes. Backup — archive directories from the container or a volume snapshot.

```bash
# Example: tar from running classifier (paths inside image)
docker compose -p union_ai_apple exec classifier \
  tar -czf - /app/chroma_db /app/bm25_db > backups/rag_indexes_$(date +%Y%m%d).tar.gz
```

**Recovery after loss:** mount archive or run reindex:

```bash
make docker-reindex-apply
# or: python scripts/reindex_rag.py (classifier stopped)
```

Reindex from `data/` takes time (e5 + BM25); index backups speed up recovery.

---

## Uploads (chat photos)

```bash
docker compose -p union_ai_apple exec server \
  tar -czf - /data/uploads > backups/uploads_$(date +%Y%m%d).tar.gz
```

---

## Backup verification

Once a month: restore Postgres dump to a test container and open 1–2 sessions; for RAG — smoke `python scripts/run_rag_eval.py --suite apple --fast --in-process` on restored index.

---

## Related documents

- [DEPLOY.md](../DEPLOY.md)
- [knowledge-base/docker-overview.md](./knowledge-base/docker-overview.md)
- [knowledge-base/metrics-and-alerts.md](./knowledge-base/metrics-and-alerts.md)
