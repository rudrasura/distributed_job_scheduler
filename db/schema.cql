-- Key Space
CREATE KEYSPACE IF NOT EXISTS scheduler 
WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1};

USE scheduler;

-- Main jobs table
-- Partitioned by job_id for standard lookups by ID. 
-- We need shard_id and next_fire_at in the PK if we want them in the MV's PK.
-- However, we can't just add them to PK of 'jobs' because that changes the lookup nature (would need all 3 to find a job).
-- 
-- ALTERNATIVE: The MV must select from columns available.
-- Scylla MV restriction: "The view primary key must contain all the base table's primary key columns."
-- AND "The view primary key can only contain columns that are part of the base table's primary key." -> INCORRECT.
-- 
-- CORRECTION: Scylla MV Primary Key:
-- 1. Must include ALL columns from the base table's Primary Key.
-- 2. Can include other columns *only if they are NOT null* (WHERE clause).
-- 
-- LET'S RETRY the MV definition. 
-- Issue was: "Cannot include more than one non-primary key column 'next_fire_at' in materialized view primary key"
-- This error usually means we are trying to add a column seamlessly but hitting a specific restriction or I misunderstood the error.
-- 
-- Wait, the error message `Cannot include more than one non-primary key column ...` is specific.
-- Actually, a better approach for the "Picker" pattern in Scylla is:
-- Don't use MV if it's complex. Use a separate `time_buckets` table and manage it via dual-writes (or "Writer" service doing batch writes).
-- 
-- BUT, for Phase 2, let's try to get MV working for simplicity if possible.
-- If `jobs` PK is `(job_id)`, then MV PK must be `(shard_id, next_fire_at, job_id)`.
-- `job_id` is the base PK. `shard_id` and `next_fire_at` are regular columns.
-- Check Scylla docs: "A materialized view's primary key must include all of the base table's primary key columns." - Yes.
-- "You can use any column of the base table in the view's primary key." - Yes, but subject to key restrictions.
--
-- The specific error "Cannot include more than one non-primary key column" suggests a limitation in older Scylla versions or specific config? 
-- Or maybe I just need to be careful with the order.
-- 
-- Let's try to minimalize:
-- 
-- Base: PRIMARY KEY (job_id)
-- View: PRIMARY KEY ((shard_id), next_fire_at, job_id)
-- 
-- This *should* work if `shard_id` and `next_fire_at` are restricted to be NOT NULL in the WHERE clause.
--
-- Let's re-verify the error: "Cannot include more than one non-primary key column 'next_fire_at' in materialized view primary key"
-- This implies `shard_id` was accepted? Or it failed on the count.
-- 
-- Let's try partitioning by `(shard_id, next_fire_at)`? No, `next_fire_at` should be a clustering key for range queries.
-- 
-- WORKAROUND:
-- Use a separate table `job_queue` for the picker.
-- This is often cleaner for "Queue" patterns anyway.
-- 
-- Table `jobs`: Source of truth.
-- Table `job_queue`: The schedule index.
-- PK: ((shard_id), next_fire_at, job_id)
-- 
-- We will manage this dual-write in the implementation code (Job Submission).
-- This avoids MV complexity and "ghost row" issues in MVs.
-- 
-- DECISION: Switch to manual dual-write table `job_queue`.

CREATE TABLE IF NOT EXISTS jobs (
    job_id UUID,
    project_id TEXT,
    user_id TEXT,
    payload TEXT,
    cron_schedule TEXT,
    next_fire_at TIMESTAMP,
    status TEXT,
    shard_id INT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    max_retries INT,
    retry_count INT,
    -- We add these to allow efficient filtering if needed, but lookup is by job_id
    PRIMARY KEY ((job_id))
);

CREATE TABLE IF NOT EXISTS job_runs (
    job_id UUID,
    run_id UUID,
    user_id TEXT,
    status TEXT,
    output TEXT,
    error_message TEXT,
    worker_id TEXT,
    triggered_at TIMESTAMP,
    completed_at TIMESTAMP,
    PRIMARY KEY ((job_id), run_id)
) WITH CLUSTERING ORDER BY (run_id DESC);

CREATE TABLE IF NOT EXISTS idempotency_lookup (
    idempotency_key TEXT,
    job_id UUID,
    created_at TIMESTAMP,
    PRIMARY KEY ((idempotency_key))
);

-- Manual Index Table (instead of MV)
-- Used by Picker to find jobs due for execution.
-- Partition: shard_id (0-1024 or similar)
-- Cluster: next_fire_at (asc) -> unique job_id
    shard_id INT,
    next_fire_at TIMESTAMP,
    job_id UUID,
    status TEXT,
    PRIMARY KEY ((shard_id), next_fire_at, job_id)
) WITH CLUSTERING ORDER BY (next_fire_at ASC, job_id ASC);

-- Manual Lookup Table for Users (Efficient)
CREATE TABLE IF NOT EXISTS user_jobs (
    user_id TEXT,
    created_at TIMESTAMP,
    job_id UUID,
    status TEXT,
    next_fire_at TIMESTAMP,
    PRIMARY KEY ((user_id), created_at, job_id)
) WITH CLUSTERING ORDER BY (created_at DESC, job_id ASC);
