.PHONY: build test run smoke vet report

BINARY := reel-life
BUILD_DIR := bin

build:
	go build -o $(BUILD_DIR)/$(BINARY) ./cmd/reel-life/

test:
	go test ./...

vet:
	go vet ./...

report:
	go run ./cmd/reel-life-report -events "$${EVIDENCE_PATH:-events.jsonl}"

# Run locally with dev config. Expects .env file with secrets.
run: build
	set -a && . ./.env && set +a && ./$(BUILD_DIR)/$(BINARY) -config dev.yaml

# Quick smoke test: start the service, check health, stop it.
smoke: build
	@set -a && . ./.env && set +a && \
	./$(BUILD_DIR)/$(BINARY) -config dev.yaml & PID=$$! && \
	sleep 2 && \
	curl -sf http://localhost:8080/healthz && echo ' smoke ok' && \
	kill $$PID 2>/dev/null || true
