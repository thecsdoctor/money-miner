-- V4__join_codes_consumed_by_set_null.sql — deleting a worker must not be
-- blocked by the join code it consumed (audit row keeps the history, the FK
-- simply goes NULL). Found by the v0.1.0 end-to-end smoke test:
-- DELETE /v1/workers/{id} returned 500 (FK violation) before this fix.
ALTER TABLE join_codes
  DROP CONSTRAINT join_codes_consumed_by_fkey,
  ADD CONSTRAINT join_codes_consumed_by_fkey
    FOREIGN KEY (consumed_by) REFERENCES workers(id) ON DELETE SET NULL;
