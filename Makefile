# Local database helpers. Needs Docker with the compose plugin.
PSQL = docker compose exec -T db psql -U macker -d macker -v ON_ERROR_STOP=1 -q

.PHONY: db-up db-down db-migrate db-reset db-psql

db-up:        ## start Postgres+PostGIS and wait until healthy
	docker compose up -d --wait

db-down:      ## stop the database (data is kept)
	docker compose down

db-migrate:   ## apply the "Up" section of every migration, in order
	@for f in migrations/*.sql; do \
	  echo "applying $$f"; \
	  sed -n '/-- +goose Up/,/-- +goose Down/p' $$f | sed '/-- +goose/d' | $(PSQL) || exit 1; \
	done

db-reset:     ## wipe all local data and re-apply migrations
	docker compose down -v
	docker compose up -d --wait
	$(MAKE) db-migrate

db-psql:      ## open a psql shell
	docker compose exec db psql -U macker -d macker
