.PHONY: build run dev clean front admin test

# Build everything
build: front admin
	go build -ldflags="-s -w" -o fenfa ./cmd/server

# Build frontend only
front:
	cd web/front && npm ci && npm run build

admin:
	cd web/admin && npm ci && npm run build

# Run server (builds frontend first if needed)
run: build
	./fenfa

# Development: start backend only (frontend via vite dev)
dev:
	go run ./cmd/server

# Run tests
test:
	go test ./...

# Clean build artifacts
clean:
	rm -f fenfa
	rm -rf internal/web/dist
