# Decisions

## Data Model
- **Partitioning**: `jobs` table uses `job_id` as partition key for O(1) lookups by ID.
- **Queueing**: Switched from Scylla Materialized View (MV) to a manual side-table `job_queue` (`shard_id`, `next_fire_at`) for picking jobs. 
  - *Reason*: Scylla MV complained about multiple non-PK columns in the View key. Manual table is robust and explicit, though requires dual-writes (handled by `LOGGED BATCH` in Scylla).
- **Sharding**: `shard_id` (0-N) allows parallel Pickers to consume the schedule without contention.
