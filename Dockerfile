FROM golang:1.26 AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bpc-api ./cmd/server

FROM chromedp/headless-shell:latest
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=build /bpc-api /bpc-api
COPY --from=build /app/bpc-src/bypass-paywalls-chrome-clean-master /app/bpc-src
COPY --from=build /app/config_sites.json /app/config_sites.json
ENV BPC_EXTENSION_PATH=/app/bpc-src
EXPOSE 8080
ENTRYPOINT ["/bpc-api", "-browsers", "2", "-addr", ":8080", "-config", "/app/config_sites.json"]
