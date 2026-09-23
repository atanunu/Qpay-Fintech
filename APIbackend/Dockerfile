# Patch-pinned build baseline; production image digests require release qualification.
FROM golang:1.27.1-bookworm AS build
WORKDIR /src/APIbackend
COPY APIbackend/go.mod APIbackend/go.sum ./
RUN go mod download && go mod verify
COPY APIbackend/ ./
RUN CGO_ENABLED=0 go build -trimpath -o /out/api ./cmd/api && \
    CGO_ENABLED=0 go build -trimpath -o /out/worker ./cmd/worker && \
    CGO_ENABLED=0 go build -trimpath -o /out/scheduler ./cmd/scheduler && \
    CGO_ENABLED=0 go build -trimpath -o /out/migrate ./cmd/migrate && \
    CGO_ENABLED=0 go build -trimpath -o /out/qpfctl ./cmd/qpfctl
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/ /app/bin/
USER 65532:65532
WORKDIR /app
EXPOSE 8080
ENTRYPOINT ["/app/bin/api"]
