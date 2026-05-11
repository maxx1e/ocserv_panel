-- Add state to historical_sessions for tracking session outcome.
-- Existing rows default to 'disconnected' (safe fallback).
ALTER TABLE historical_sessions ADD COLUMN state TEXT NOT NULL DEFAULT 'disconnected'
  CHECK (state IN ('authenticated', 'disconnected', 'timed_out', 'error'));
