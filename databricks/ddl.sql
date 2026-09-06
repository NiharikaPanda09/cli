-- Storage for `entire handoff --json` packets.
--
-- Merge key is (repo_key, checkpoint_id, session_id) -- never repo_key alone.
-- A repo without a remote gets a synthetic `local/<basename>` key, so two
-- developers' local clones collide on `local/cli`. Row-level uniqueness needs
-- section_name and item_index on top of the merge key, because one checkpoint
-- contributes many rows.
--
-- item_index = -1 marks a section that produced no items; section_note then
-- carries the reason it degraded. Those rows are kept deliberately: "the graph
-- was degenerate" is itself a finding, and dropping them makes an empty
-- section indistinguishable from one that was never run.

CREATE VOLUME IF NOT EXISTS main.default.handoff
  COMMENT 'Landing zone for handoff packet NDJSON uploaded by handoff-databricks.';

CREATE TABLE IF NOT EXISTS main.default.handoff_packets (
  repo_key           STRING  NOT NULL COMMENT 'Packet repo field; may be local/<basename> and is NOT unique on its own',
  checkpoint_id      STRING           COMMENT 'Cited checkpoint; empty for item_index = -1 rows',
  session_id         INT              COMMENT 'Session index within the checkpoint',
  head_checkpoint_id STRING           COMMENT 'HEAD Entire-Checkpoint trailer at generation time',
  generated_at       TIMESTAMP        COMMENT 'When the packet was produced',
  section_name       STRING  NOT NULL COMMENT 'intent | open_items | dead_ends | surface | stopped',
  item_index         INT     NOT NULL COMMENT '-1 when the section was empty',
  item_text          STRING           COMMENT 'The cited line',
  cite_line          INT              COMMENT 'transcript.jsonl offset; dead_ends only',
  section_note       STRING           COMMENT 'Degradation reason when the section is empty',
  ingested_at        TIMESTAMP        COMMENT 'Ingest time, set by the uploader'
)
USING DELTA
PARTITIONED BY (repo_key)
COMMENT 'Handoff packets, one row per (section, item, citation).';

-- Vector Search source view: the two sections worth retrieving across repos.
-- Citation columns are carried through on purpose -- a retrieval result without
-- a checkpoint id is the unverifiable assertion the packet exists to avoid.
CREATE OR REPLACE VIEW main.default.handoff_retrievable AS
SELECT
  repo_key,
  checkpoint_id,
  session_id,
  cite_line,
  section_name,
  item_text,
  generated_at
FROM main.default.handoff_packets
WHERE section_name IN ('dead_ends', 'intent')
  AND item_index >= 0
  AND item_text IS NOT NULL;

-- ---------------------------------------------------------------------------
-- Vector Search: managed embeddings, so no client ever computes a vector.
--
-- Delta Sync keeps the index following the table. The CLI only POSTs a query
-- string to /api/2.0/vector-search/indexes/<name>/query; Databricks embeds it
-- with the serving endpoint named below. That is what keeps the Go side on the
-- standard library with no SDK and no go.mod change.
--
-- Change Data Feed is required for a Delta Sync index. Enable it before
-- creating the index, or the sync fails with a non-obvious error.
ALTER TABLE main.default.handoff_packets
  SET TBLPROPERTIES (delta.enableChangeDataFeed = true);

-- The index itself is not SQL. Create it once, either from the Databricks UI
-- (Compute -> Vector Search) or via REST:
--
--   POST /api/2.0/vector-search/endpoints
--     {"name": "entire-handoff", "endpoint_type": "STANDARD"}
--
--   POST /api/2.0/vector-search/indexes
--     {"name": "main.default.handoff_idx",
--      "endpoint_name": "entire-handoff",
--      "primary_key": "row_id",
--      "index_type": "DELTA_SYNC",
--      "delta_sync_index_spec": {
--        "source_table": "main.default.handoff_packets",
--        "pipeline_type": "TRIGGERED",
--        "embedding_source_columns": [
--          {"name": "item_text", "embedding_model_endpoint_name": "databricks-gte-large-en"}
--        ]}}
--
-- Then point the CLI at it:
--   export DATABRICKS_VECTOR_INDEX=main.default.handoff_idx
--
-- A Delta Sync index needs a stable primary key. The ingest binary does not
-- emit one, so add it here rather than changing the row shape:
ALTER TABLE main.default.handoff_packets
  ADD COLUMN IF NOT EXISTS row_id STRING
  COMMENT 'Stable key for Vector Search: repo_key|checkpoint_id|session_id|section_name|item_index';

UPDATE main.default.handoff_packets
  SET row_id = concat_ws('|', repo_key, checkpoint_id,
                         cast(session_id AS STRING), section_name,
                         cast(item_index AS STRING))
  WHERE row_id IS NULL;
