FROM golang:1.26-alpine AS builder
WORKDIR /build
COPY paywall-api/ .
RUN go mod tidy && go build -o paywall-api .

FROM alpine:3.21
RUN apk add --no-cache chromium
WORKDIR /app
COPY --from=builder /build/paywall-api .
ENV BPC_EXTENSION_PATH=/app/plugin
ENV PROXY_SERVER=""
EXPOSE 8080
CMD ["./paywall-api"]
