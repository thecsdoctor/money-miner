-- V3__rollup_job_notes.sql — no schema change; documents that rollup and
-- retention of metric_samples (aggregate hourly into metric_rollups, delete
-- raw rows older than 48 h) is backend-side Go (internal/metrics), run
-- hourly. Keeping it out of SQL keeps the retention policy testable in Go.
COMMENT ON TABLE metric_rollups IS
  'Hourly aggregates of metric_samples, populated by the backend retention job (internal/metrics). Raw samples are deleted after 48 h; rollups are kept forever.';
