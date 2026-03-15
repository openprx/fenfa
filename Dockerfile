# Multi-stage, multi-arch build for fenfa
# Frontend build (Node)
FROM node:20-alpine AS node-builder
WORKDIR /src
COPY . .
# Build front and admin (vite)
RUN cd web/front && npm install && npm run build
RUN cd web/admin && npm install && npm run build

# Go build (needs CGO + sqlite headers)
FROM golang:1.25-alpine AS go-builder
RUN apk add --no-cache build-base sqlite-dev
ENV CGO_ENABLED=1
WORKDIR /src
COPY . .
# Bring in the built frontend artifacts from node-builder stage
COPY --from=node-builder /src/internal/web/dist /src/internal/web/dist
# Build server binary
ARG VERSION=dev
ARG COMMIT=none
RUN go build -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" -o /out/fenfa ./cmd/server

# Runtime image
FROM alpine:3.22.2
RUN apk add --no-cache ca-certificates sqlite-libs tzdata su-exec \
    && adduser -D -H -u 10001 appuser
WORKDIR /app
# Copy binary and entrypoint
COPY --from=go-builder /out/fenfa /app/fenfa
COPY scripts/entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh
# Declare volumes for persistence (db under /data, uploads under /app/uploads)
VOLUME ["/data", "/app/uploads"]
ENV FENFA_DATA_DIR=/data
EXPOSE 8000
# Start as root, entrypoint chowns then drops to appuser
ENTRYPOINT ["/app/entrypoint.sh"]
