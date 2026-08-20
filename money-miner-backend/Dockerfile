# money-miner master server — multi-stage, CGO-free static binary on alpine
# (wget needed for the compose healthcheck; distroless has no shell/tools).
FROM golang:1.26-alpine@sha256:28d89ee9cc0ff9fec75c82ca201e6bf7fdf9a679d4b7b24dfa04f2bb766bb468 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath \
      -ldflags "-s -w -X main.version=${VERSION}" \
      -o /out/money-miner-server ./cmd/server

FROM alpine:3.21@sha256:48b0309ca019d89d40f670aa1bc06e426dc0931948452e8491e3d65087abc07d
RUN apk add --no-cache ca-certificates wget \
 && adduser -D -u 10001 money-miner
USER money-miner
COPY --from=build /out/money-miner-server /usr/local/bin/money-miner-server
EXPOSE 8080
ENTRYPOINT ["money-miner-server"]
