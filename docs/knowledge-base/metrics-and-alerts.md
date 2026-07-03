# Metrics and basic alerts

Go server exposes **Prometheus text exposition** on:

- `GET /metrics`
- `GET /api/metrics`

Endpoint is **public** (no auth) — on production restrict access by network (firewall / internal scrape only).

---

## Metrics

| Name | Type | Description |
|------|------|-------------|
| `garden_http_requests_total` | counter | All HTTP requests |
| `garden_http_responses_2xx_total` | counter | 2xx responses |
| `garden_http_responses_4xx_total` | counter | 4xx responses |
| `garden_http_responses_5xx_total` | counter | 5xx responses |
| `garden_llm_errors_total` | counter | LLM API call errors |
| `garden_rag_requests_total` | counter | RAG answer attempts, incl. soft fails (via `logRAGTrace`) |
| `garden_rag_verify_pass_total` | counter | Answers that passed number verify |
| `garden_rag_verify_fail_total` | counter | Answers that failed verify |
| `garden_rag_soft_fail_total` | counter | Soft fail (no context / verify) |
| `garden_rag_retrieval_ms_total` | counter | Sum of retrieval latency (ms) |
| `garden_rag_llm_ms_total` | counter | Sum of LLM latency (ms) |

How counters are recorded (`server/metrics.go`):

- **HTTP counters** — `metricsMiddleware` (registered in `main.go`) calls `recordHTTPStatus` for every response except `/metrics` itself.
- **`garden_llm_errors_total`** — `recordLLMError` on any failed LLM call: chat completion, streaming, photo recommendation, and the claim-verify judge.
- **RAG counters** — `recordRAGTraceMetrics(trace)` called from `logRAGTrace` for each completed RAG answer (both `/message` and `/message/stream`).

There are no separate claim-verification metrics: when the optional LLM claim judge (`RAG_VERIFY_CLAIMS_ENABLED`, see [server-rag_chat.md](./server-rag_chat.md)) rejects an answer, it is counted in `garden_rag_verify_fail_total` and `garden_rag_soft_fail_total`; a failed judge call itself (fail-open) increments `garden_llm_errors_total`.

Average retrieval latency (PromQL):

```promql
rate(garden_rag_retrieval_ms_total[5m]) / rate(garden_rag_requests_total[5m])
```

---

## Example scrape (Prometheus)

```yaml
scrape_configs:
  - job_name: garden-server
    metrics_path: /metrics
    static_configs:
      - targets: ["server:8080"]
```

---

## Example alerts (Alertmanager)

```yaml
groups:
  - name: garden-server
    rules:
      - alert: GardenHigh5xxRate
        expr: rate(garden_http_responses_5xx_total[5m]) > 0.1
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Garden server 5xx rate elevated"

      - alert: GardenLLMErrors
        expr: increase(garden_llm_errors_total[10m]) > 5
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "LLM API errors in garden server"

      - alert: GardenRAGVerifyFailRate
        expr: |
          rate(garden_rag_verify_fail_total[15m])
          / rate(garden_rag_requests_total[15m]) > 0.3
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "High RAG verify failure rate"

      - alert: GardenSlowRAGRetrieval
        expr: |
          rate(garden_rag_retrieval_ms_total[5m])
          / rate(garden_rag_requests_total[5m]) > 15000
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "RAG retrieval avg > 15s"
```

Tune thresholds for pilot load.

---

## Feedback + RAG in admin

`GET /admin/feedback` returns a `summary` (`likes`/`dislikes`) and, for each rating, an optional `rag` field (if a `rag_answer` event exists in `analytics_events` with the same `message_id`): category, fragments, verify_pass, verify_reason, soft_fail, latency.

---

## Related documents

- [quality-eval-and-rag-logs.md](./quality-eval-and-rag-logs.md)
- [server-admin-and-ux-api.md](./server-admin-and-ux-api.md)
