# Walkthrough: `.github/workflows/ci.yml` and RAG Eval

**Source files:**
- `.github/workflows/ci.yml` — on every push/PR
- `.github/workflows/rag-eval.yml` — **manual only** (full retrieval eval)

**Platform:** [GitHub Actions](https://docs.github.com/en/actions)

---

## What is CI in simple terms

**CI (Continuous Integration)** — on push or Pull Request GitHub runs a virtual machine (Ubuntu), runs tests and build, shows ✅ or ❌.

---

## When `ci.yml` runs

```yaml
on:
  push:
    branches: [master, main, "feature/**", "fix/**", "feat/**"]
  pull_request:
    branches: [master, main]
```

| Event | Condition |
|-------|-----------|
| **push** | To `master`, `main`, `feature/**`, `fix/**`, `feat/**` |
| **pull_request** | PR into `master` or `main` |

`concurrency` cancels old runs on new push to the same branch — saves Actions minutes.

Result: repository → **Actions** → workflow **CI**.

---

## CI overview (PR)

```mermaid
flowchart TB
    subgraph trigger [Trigger]
        P[push / pull_request]
    end
    subgraph jobs [Five jobs in parallel]
        G[go-test vet + race]
        GL[go-lint golangci-lint]
        PY[python-test]
        PL[python-lint ruff]
        D[docker-build + compose smoke]
    end
    P --> G
    P --> GL
    P --> PY
    P --> PL
    P --> D
```

Typical time: **~10–15 minutes** (no reindex, no full RAG eval).

---

## Job 1: `go-test`

- Go **1.23**, `working-directory: server`
- `go mod tidy` → `go vet ./...` → `go test -v -race -count=1 ./...`
- `CROPS_CONFIG_PATH: ${{ github.workspace }}/config/crops.json`

Coverage: verify (numeric + claims judge), crops, admin, auth, rate limit, feedback report, verify contract.

---

## Job 2: `go-lint`

- `golangci-lint` (pinned version) over `server/` — errcheck, staticcheck, govet, unused, etc.

---

## Job 3: `python-test`

- Python **3.11**, `pytest tests/ -v --tb=short` (env `CROPS_CONFIG_PATH: config/crops.json`)
- Dependencies: `tests/requirements-test.txt` (no PyTorch/Chroma)
- Expected: **45 passed** (39 test functions + 6 parametrized verify-contract cases)

---

## Job 4: `python-lint`

- `ruff check api rag cv scripts tests` (config in `ruff.toml`)

---

## Job 5: `docker-build`

- `scripts/docker_build.sh` — images **server**, **webapp**, **classifier**
- Classifier: `SKIP_HF_BAKE=1` (no baking HF models into image on CI)
- Smoke in container: import torch 2.5 CPU, `load_all_documents()` from `data/`
- **End-to-end compose smoke:** `docker compose up -d --build --wait` (warmup disabled, `TELEGRAM_AUTH_DISABLED=true`) → `scripts/smoke.sh` (health, crops, session, onboarding) → `compose down -v`

**Does not:** reindex, `run_rag_eval.py`, push to registry.

---

## RAG Eval — separate workflow (manual)

**File:** `.github/workflows/rag-eval.yml`  
**Trigger:** `workflow_dispatch` (Actions → **RAG Eval** → Run workflow)

Why not on every PR: reindex + embeddings on CPU in GHA takes **20–45+ minutes**.

### Parameters

| Input | Values |
|-------|--------|
| `suite` | `apple`, `pear`, `plum`, `demo_hr`, `all` |

### What job `rag-eval` does

1. Build classifier image (`SKIP_HF_BAKE=1`)
2. In container: `reindex_rag.py` → `run_rag_eval.py --suite … --in-process --fast --timeout 300 --workers 2`
3. `RAG_RERANK_ENABLED=false` on CI (speed; reranker enabled locally)
4. Secret **`HF_TOKEN`** in repo settings (optional, speeds up HF)

Local equivalent:

```bash
python scripts/run_rag_eval.py --suite all --timeout 300
```

(needs running classifier or `--in-process`)

---

## Comparison

| Workflow | When | Duration | What it checks |
|----------|------|----------|----------------|
| **CI** | every PR | ~10–15 min | lint, unit tests (race), image build, compose smoke |
| **RAG Eval** | manual | up to ~45 min | retrieval regression on JSONL |

---

## Locally before push

```powershell
cd server; go mod tidy; go test ./...
pytest tests/ -v
docker build -f Dockerfile.server -t test-server .
```

Full eval — before release or after changing `data/`:

```powershell
python scripts/run_rag_eval.py --suite all
```

---

## What CI does not include (normal)

- Deploy to server (CD)
- End-to-end eval with LLM (`--full`)

---

## Related documents

| Topic | File |
|-------|------|
| Go tests | `server/*_test.go`, [tests-overview.md](./tests-overview.md) |
| Python tests | [tests-overview.md](./tests-overview.md) |
| Eval suites | [eval/README.md](../../eval/README.md) |
| RAG quality | [quality-eval-and-rag-logs.md](./quality-eval-and-rag-logs.md) |

---

## Brief summary

**CI** — fast safety net: lint (golangci-lint, ruff) + Go/Python unit tests (with race detector) + Docker build + compose smoke. **RAG Eval** — heavy retrieval regression, run manually when corpus or RAG code changes.
