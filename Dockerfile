# Stage 1: Build frontend
FROM node:22-alpine AS frontend-build
WORKDIR /build
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm ci --ignore-scripts 2>/dev/null || npm install
COPY frontend/ .
RUN npm run build

# Stage 2: Build and test backend
FROM golang:1.22-alpine AS backend-build
WORKDIR /build
COPY backend/go.mod ./
RUN go mod download
COPY backend/ .
RUN go test ./...
RUN CGO_ENABLED=0 go build -o vault .

# Stage 3: Minimal runtime image
FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=backend-build /build/vault .
COPY --from=frontend-build /build/dist ./frontend
RUN mkdir -p /contents
EXPOSE 8080
ENTRYPOINT ["./vault"]
