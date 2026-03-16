# Decisions (append-only)

- 2026-03-15: Added migration `0002_add_user_role.sql` to introduce `users.role` with default `user`; enables minimal role-based authorization without redesigning existing schema.
- 2026-03-15: Chose header-based auth scheme (`X-User-Id`) for current phase to keep implementation minimal and explicit until full token auth is introduced later.
