# Local database helpers. Needs Docker with either the compose plugin ("docker compose")
# or the standalone "docker-compose". Override with: make COMPOSE="docker-compose" db-up
COMPOSE ?= $(shell docker compose version >/dev/null 2>&1 && echo "docker compose" || echo "docker-compose")
PSQL = $(COMPOSE) exec -T db psql -U macker -d macker -v ON_ERROR_STOP=1 -q

.PHONY: db-up db-down db-migrate db-reset db-psql

db-up:        ## start Postgres+PostGIS and wait until it accepts connections
	$(COMPOSE) up -d
	@echo "waiting for Postgres..."
	@for i in $$(seq 1 40); do \
	  $(COMPOSE) exec -T db pg_isready -U macker -d macker >/dev/null 2>&1 && echo "ready" && exit 0; \
	  sleep 2; \
	done; echo "Postgres did not become ready" >&2; exit 1

db-down:      ## stop the database (data is kept)
	$(COMPOSE) down

db-migrate:   ## apply the "Up" section of every migration, in order
	@for f in migrations/*.sql; do \
	  echo "applying $$f"; \
	  sed -n '/-- +goose Up/,/-- +goose Down/p' $$f | sed '/-- +goose/d' | $(PSQL) || exit 1; \
	done

db-reset:     ## wipe all local data and re-apply migrations
	$(COMPOSE) down -v
	$(MAKE) db-up
	$(MAKE) db-migrate

db-psql:      ## open a psql shell
	$(COMPOSE) exec db psql -U macker -d macker
