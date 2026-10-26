# --- frontend ---
FROM node:22-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-fund --no-audit
COPY web/ ./
RUN npm run build

# --- backend ---
FROM golang:1.26-alpine AS build
ARG VERSION=0.2.0
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
COPY --from=web /web/dist ./web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /passman .

# --- runtime ---
FROM alpine:3.22
# psql is included for debugging against the configured database.
RUN apk add --no-cache postgresql17-client ca-certificates tzdata \
 && adduser -D -H -u 10001 passman
COPY --from=build /passman /usr/local/bin/passman
RUN ln -s /usr/local/bin/passman /usr/local/bin/reset-password
USER passman
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/passman"]
