LOCK TABLE outbox, events, subscriptions IN ACCESS EXCLUSIVE MODE;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM outbox) OR EXISTS (SELECT 1 FROM events) OR EXISTS (SELECT 1 FROM subscriptions) THEN
        RAISE EXCEPTION 'refusing to remove nonempty relaybox tables';
    END IF;
END
$$;
DROP TABLE outbox;
DROP TABLE subscriptions;
DROP TABLE events;
