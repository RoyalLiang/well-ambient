-- Additive and backward compatible: NULL retains the legacy enabled default.
BEGIN;
SET LOCAL lock_timeout = '10s';
ALTER TABLE public.code_review_policies ADD COLUMN IF NOT EXISTS review_commits boolean;
ALTER TABLE public.code_review_policies ADD COLUMN IF NOT EXISTS review_mrs boolean;
COMMIT;
