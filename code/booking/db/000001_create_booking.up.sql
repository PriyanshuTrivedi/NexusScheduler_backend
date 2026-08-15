CREATE EXTENSION IF NOT EXISTS pgcrypto; -- gen_random_uuid()

CREATE TABLE booking (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reference_code     TEXT NOT NULL,
    user_id            UUID NOT NULL,
    resource_id        UUID NOT NULL,        -- foreign concept, owned by Resource service; no DB FK
    start_time         TIMESTAMPTZ NOT NULL,
    end_time           TIMESTAMPTZ NOT NULL,
    title              TEXT NOT NULL,
    subtitle           TEXT NOT NULL,
    status             TEXT NOT NULL,         -- confirmed | waitlisted | cancelled | rescheduled | failed
    parent_booking_id  UUID,                   -- set only when this row resulted from a reschedule
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_parent_booking FOREIGN KEY (parent_booking_id) REFERENCES booking (id),
    CONSTRAINT chk_booking_time_order CHECK (end_time > start_time)
);

-- Prevents double-booking: only one ACTIVE booking per resource+time.
CREATE UNIQUE INDEX uq_resource_active_slot ON booking (resource_id, start_time)
    WHERE status IN ('confirmed', 'waitlisted');

-- Lets reference_code be reused across a cancel->rebook or a reschedule
-- lineage, while still being unique among currently-active bookings.
CREATE UNIQUE INDEX uq_reference_code_active ON booking (reference_code)
    WHERE status IN ('confirmed', 'waitlisted');

CREATE INDEX idx_booking_user     ON booking (user_id);
CREATE INDEX idx_booking_resource ON booking (resource_id);
CREATE INDEX idx_booking_reference ON booking (reference_code);
