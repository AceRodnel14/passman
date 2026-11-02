# --- frontend ---
FROM node:22.23-alpine3.24 AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-fund --no-audit
COPY web/ ./
RUN npm run build

# --- backend ---
FROM golang:1.26-alpine3.24 AS build
ARG VERSION=1.0.0
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
COPY --from=web /web/dist ./web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /passman .

# --- runtime ---
FROM alpine:3.24
# psql is included for debugging against the configured database.
# apk: prefix constraint (~=), not an exact pin, so rebuilds get patch updates but the mirror dropping old -rN builds cannot break them.
# ca-certificates and tzdata are date-versioned, so they stay unconstrained.
# hadolint ignore=DL3018
RUN apk add --no-cache postgresql18-client~=18 ca-certificates tzdata \
 && adduser -D -H -u 10001 passman
COPY --from=build /passman /usr/local/bin/passman
RUN ln -s /usr/local/bin/passman /usr/local/bin/reset-password
USER passman
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/passman"]
