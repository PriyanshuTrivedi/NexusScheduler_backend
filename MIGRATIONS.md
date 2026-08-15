# Nexus Scheduler database migrations

Each service has exactly one migration pair:

- `code/identity/db/000001_create_identity.up.sql`
- `code/identity/db/000001_create_identity.down.sql`
- `code/resource/db/000001_create_resource.up.sql`
- `code/resource/db/000001_create_resource.down.sql`
- `code/booking/db/000001_create_booking.up.sql`
- `code/booking/db/000001_create_booking.down.sql`

From `nexusSchedulerBackend/`, after starting PostgreSQL:

```bash
make migrate-up
```

For a clean local reset, roll everything down and then up:

```bash
make migrate-down
make migrate-up
```

The Resource migration creates `resource_type` before `resource`, so resource-type registration works on a fresh database.

Resource type and organization names are protected by case-insensitive unique indexes.
