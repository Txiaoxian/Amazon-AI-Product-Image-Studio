# MySQL Init Placeholder

Place idempotent local-only MySQL initialization scripts here when needed.

The backend owns schema migrations through its GORM migration runner during API
startup. This directory should stay limited to empty database initialization or
local-only notes.

Do not put production data, credentials, or non-repeatable migrations in this
directory.
