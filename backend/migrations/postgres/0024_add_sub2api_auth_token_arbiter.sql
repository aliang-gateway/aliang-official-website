-- Refresh arbiter support: track the upstream access_token's expiry and the
-- immediately-previous refresh_token (grace window) so the refresh endpoint can
-- dedupe concurrent/multi-device refreshes and serve the current pair without
-- re-calling sub2api — which would otherwise trip sub2api's refresh-token
-- replay detection and revoke the whole token family across all devices.
--
-- Both columns are nullable so existing rows (and tests that INSERT without
-- them) keep working; the arbiter treats a NULL access_expires_at as expired
-- (forces one rotation to populate it).

ALTER TABLE als_sub2api_auth_tokens ADD COLUMN access_expires_at TIMESTAMPTZ;
ALTER TABLE als_sub2api_auth_tokens ADD COLUMN prev_refresh_token TEXT;
