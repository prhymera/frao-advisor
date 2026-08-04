BINARY=frao-advisor
GOBUILD=go build -ldflags="-s -w"
GOTEST=go test -v -count=1
COMPOSE_DASH=docker compose -f docker-compose.dashboard.yml

.PHONY: build test vet clean run run-dashboard docker-up-dashboard docker-down-dashboard docker-rebuild-dashboard docker-logs-dashboard docker-ps-dashboard docker-seed-dashboard

build:
	$(GOBUILD) -o $(BINARY) .

test:
	$(GOTEST) ./...

vet:
	go vet ./...

clean:
	rm -f $(BINARY)
	rm -f advisor.db advisor.db-wal advisor.db-shm

run:
	DEEPSEEK_API_KEY=$$DEEPSEEK_API_KEY ADVISOR_DASHBOARD_PORT=9753 ./$(BINARY)

run-dashboard:
	ADVISOR_DASHBOARD_PORT=9753 ./$(BINARY) dashboard

# ─── Dashboard via Docker (single instance) ─────────────────────────
docker-up-dashboard:
	$(COMPOSE_DASH) up -d --build dashboard

docker-down-dashboard:
	$(COMPOSE_DASH) down

docker-rebuild-dashboard:
	$(COMPOSE_DASH) up -d --build --force-recreate dashboard

docker-logs-dashboard:
	$(COMPOSE_DASH) logs -f dashboard

docker-ps-dashboard:
	$(COMPOSE_DASH) ps

docker-seed-dashboard:
	@echo "Stopping dashboard, seeding history from advisor.db, restarting..."
	$(COMPOSE_DASH) down
	python3 -c "import sqlite3;c=sqlite3.connect('advisor.db');c.execute('PRAGMA wal_checkpoint(TRUNCATE)');c.close()"
	cp advisor.db data/advisor.db
	rm -f data/advisor.db-wal data/advisor.db-shm
	$(COMPOSE_DASH) up -d --build dashboard
	@echo "Seeded. Dashboard back up on 10.64.0.5:9753."
