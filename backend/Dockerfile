# libvips is a cgo dependency, so CGO_ENABLED=0 and a scratch image are both
# off the table. Debian slim is used rather than Alpine because libvips on musl
# has historically shipped without the AVIF/HEIC codecs we rely on.
FROM golang:1.26-bookworm AS builder

RUN apt-get update && apt-get install -y --no-install-recommends \
        libvips-dev \
        pkg-config \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -o openimg-server .

FROM debian:bookworm-slim
# Runtime needs the shared libraries only, not the -dev headers.
RUN apt-get update && apt-get install -y --no-install-recommends \
        libvips42 \
        ca-certificates \
        tzdata \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /app/openimg-server .

EXPOSE 8080

CMD ["./openimg-server"]
