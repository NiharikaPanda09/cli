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
