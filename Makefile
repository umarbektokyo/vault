.PHONY: dev dev-backend dev-frontend build test clean docker

# Run backend + frontend in development mode
dev: dev-backend dev-frontend

dev-backend:
	cd backend && CONTENT_ROOT=../contents PORT=8080 go run .

dev-frontend:
	cd frontend && npm run dev

# Build everything
build: build-frontend build-backend

build-frontend:
	cd frontend && npm install && npm run build

build-backend:
	cd backend && CGO_ENABLED=0 go build -o ../vault .

# Run tests
test:
	cd backend && go test -v -cover ./...

# Docker
docker:
	docker compose up --build

# Clean build artifacts
clean:
	rm -rf vault frontend/dist frontend/node_modules
