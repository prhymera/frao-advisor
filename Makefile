BINARY=frao-advisor
GOBUILD=go build -ldflags="-s -w"
GOTEST=go test -v -count=1

.PHONY: build test vet clean run

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
