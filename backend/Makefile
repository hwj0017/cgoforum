.PHONY: build run docker-up docker-down test lint clean gen-keys

# Build the binary
build:
	go build -o bin/server ./cmd/server

# Run the server
run: build
	./bin/server

# Start infrastructure
docker-up:
	docker compose -f ../docker-compose.yaml up -d

# Stop infrastructure
docker-down:
	docker compose -f ../docker-compose.yaml down

# Run tests
test:
	go test ./... -v -count=1

# Run linter
lint:
	golangci-lint run ./...

# Clean build artifacts
clean:
	rm -rf bin/ logs/

# Generate RSA key pair for JWT
gen-keys:
	@mkdir -p config/keys
	@openssl genrsa -out config/keys/private.pem 2048
	@openssl rsa -in config/keys/private.pem -pubout -out config/keys/public.pem
	@echo "RSA keys generated in config/keys/"
