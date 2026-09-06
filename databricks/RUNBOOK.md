# Standing up the Vector Search lane

`ddl.sql` says *what* to create. This says *in what order*, *what to check between
steps*, and *what will go wrong*. It exists because until now nothing in the
Databricks lane had been run against a real workspace — only against the fake
HTTP server in `globalmemory_test.go` and `upload_test.go`.

Everything below is one-time setup except step 6.

---

## Before you start: the index will be empty, and that is not a bug

Measured on this repo, 2026-09-06, across its whole checkpoint history:

```
$ entire handoff --limit 150 --json | handoff-databricks --dry-run \
    | jq -r 'select(.section_name=="intent" or .section_name=="dead_ends")
             | select(.item_index >= 0)' | wc -l
0
```

`handoff_retrievable` selects `section_name IN ('dead_ends','intent') AND
item_index >= 0`. This repo's checkpoints carry **no `Summary`** (so `intent`
produces only an `item_index = -1` degradation row) and **no failed tool calls
in range** (so `dead_ends` does the same). Nine rows ingest; zero are
retrievable.

Narrower still: `restVectorSearcher.Query` hardcodes
`filters_json = {"section_name": "dead_ends"}`, so **only dead-end rows are ever
returned** — the `intent` half of the view is indexed but unreachable from the
CLI today. Option 1 below therefore needs a repo with recorded *tool failures*,
not merely one with summaries. If you want intent rows to answer `--ask` too,
that filter is the line to change.

So creating the endpoint proves the wire protocol — real auth, real index, real
round-trip — but `--ask` will correctly return *"no prior work found for this
task"* until the table has retrievable rows. Get them one of three ways, in
descending order of honesty:

1. Ingest packets from a repo whose checkpoints have summaries.
2. Turn summarization on and accumulate new checkpoints here.
3. Seed representative rows by hand for a demo — in which case **say they are
   seeded**, because a demo that looks like retrieval over real history and is
   not is the one thing this project's whole citation design is against.

---

## 1. Credentials

The Databricks MCP server already holds these; the CLI and the ingest binary
read them from the environment separately.

```bash
export DATABRICKS_HOST=https://<workspace>.cloud.databricks.com   # https:// required
export DATABRICKS_TOKEN=<pat>
export DATABRICKS_WAREHOUSE_ID=<sql-warehouse-id>                 # COPY INTO needs it
export DATABRICKS_VOLUME_PATH=/Volumes/main/default/handoff
```

`VectorConfigFromEnv` silently returns `ok=false` if `DATABRICKS_HOST` lacks the
`https://` prefix, and `entire handoff` then just omits the `prior_art` section
with no error. If the section is missing, check the prefix first.

## 2. Tables, volume, view

Run `ddl.sql` top to bottom on the warehouse. Order matters in one place:
`delta.enableChangeDataFeed` must be set **before** the index is created, or the
Delta Sync pipeline fails with an error that does not mention CDF.

Verify:

```sql
DESCRIBE DETAIL main.default.handoff_packets;   -- expect properties.delta.enableChangeDataFeed = true
SELECT count(*) FROM main.default.handoff_retrievable;
```

## 3. Endpoint

```bash
curl -sS -X POST "$DATABRICKS_HOST/api/2.0/vector-search/endpoints" \
  -H "Authorization: Bearer $DATABRICKS_TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"entire-handoff","endpoint_type":"STANDARD"}'
```

A STANDARD endpoint is **billed and always-on**. Provisioning takes roughly
10–20 minutes; poll until `endpoint_status.state` is `ONLINE`:

```bash
curl -sS "$DATABRICKS_HOST/api/2.0/vector-search/endpoints/entire-handoff" \
  -H "Authorization: Bearer $DATABRICKS_TOKEN" | jq .endpoint_status
```

Do not create the index until it is `ONLINE`.

## 4. Index

Managed embeddings: Databricks embeds both the indexed column and the query, so
the CLI never calls an embeddings API and `go.mod` gains no SDK.

```bash
curl -sS -X POST "$DATABRICKS_HOST/api/2.0/vector-search/indexes" \
  -H "Authorization: Bearer $DATABRICKS_TOKEN" -H 'Content-Type: application/json' \
  -d '{
    "name": "main.default.handoff_idx",
    "endpoint_name": "entire-handoff",
    "primary_key": "row_id",
    "index_type": "DELTA_SYNC",
    "delta_sync_index_spec": {
      "source_table": "main.default.handoff_packets",
      "pipeline_type": "TRIGGERED",
      "embedding_source_columns": [
        {"name": "item_text",
         "embedding_model_endpoint_name": "databricks-gte-large-en"}
      ]}}'
```

Two mismatches to expect between this and `ddl.sql`:

- `ddl.sql` points `source_table` at `handoff_packets`, but the columns the CLI
  asks for (`repo_key, checkpoint_id, session_id, section_name, item_text,
  cite_line` — see `priorArtColumns`) are exactly the `handoff_retrievable`
  view's. Indexing the base table means the `dead_ends` filter in
  `restVectorSearcher.Query` is doing the filtering the view would otherwise do,
  and `item_index = -1` degradation rows become retrievable — they have empty
  `item_text`, which `hitsFrom` drops, so they waste `num_results` slots rather
  than corrupting output. Prefer indexing the view if Delta Sync accepts it in
  your workspace; otherwise keep the base table and accept the wasted slots.
- `row_id` must be non-null for every row before the first sync. `ddl.sql`
  backfills it with an `UPDATE ... WHERE row_id IS NULL`, but the ingest binary
  does not emit it, so **that UPDATE has to be re-run after every ingest** until
  `row_id` becomes a generated column. This is the sharpest edge here.

## 5. Point the CLI at it

```bash
export DATABRICKS_VECTOR_INDEX=main.default.handoff_idx
entire handoff --ask "why did the reftable test start failing"
```

Expect a `prior_art` section. Failure modes are all soft by design — the section
carries the reason and the rest of the packet still builds:

| Section note | Cause |
| --- | --- |
| `vector search not configured` | one of the three env vars is unset, or host lacks `https://` |
| `vector search unavailable: ... 404` | index name wrong, or not finished its first sync |
| `vector search unavailable: ... 403` | token lacks access to the endpoint |
| `no prior work found for this task` | reached the index; it has no matching rows (see the top of this file) |

## 6. Ingest (repeat per packet)

```bash
entire handoff --limit 50 --json | handoff-databricks
```

`--dry-run` prints the NDJSON rows without uploading and needs no credentials —
use it to check row shape before spending a round-trip. Then re-run the `row_id`
backfill from step 4, and trigger a sync:

```bash
curl -sS -X POST \
  "$DATABRICKS_HOST/api/2.0/vector-search/indexes/main.default.handoff_idx/sync" \
  -H "Authorization: Bearer $DATABRICKS_TOKEN"
```

---

## What the query actually sends

Worth knowing before pointing this at a sensitive repo. `globalmemory.go` sends
one JSON body containing `query_text` and nothing else from the session — no
transcript, no prompt, no file contents. That text is either `--ask` verbatim or
a fallback built from up to two checkpoints' `Summary.Intent`, and both go
through `redact.String` before the request is built, then are truncated to 400
characters.

`redact.String` is the local, non-network layer set — it makes no outbound call
of its own. Its coverage is not total: it passes bare AWS access key ids through
untouched, including `AKIAIOSFODNN7EXAMPLE`. See the note on `redactableSecret`
in `privacy_test.go`.

## Teardown

The endpoint bills until deleted.

```bash
curl -sS -X DELETE \
  "$DATABRICKS_HOST/api/2.0/vector-search/indexes/main.default.handoff_idx" \
  -H "Authorization: Bearer $DATABRICKS_TOKEN"
curl -sS -X DELETE "$DATABRICKS_HOST/api/2.0/vector-search/endpoints/entire-handoff" \
  -H "Authorization: Bearer $DATABRICKS_TOKEN"
```
