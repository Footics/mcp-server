# footics-mcp — multi-stage build → tiny static binary (same pattern as footics-api).
# Deps are VENDORED (vendor/), so the build is hermetic: no module proxy, no
# network — reproducible on any box/CI (proxy.golang.org has 403'd transitive
# deps from some IPs; vendoring sidesteps it entirely). GOFLAGS pins vendor mode.

# ---- build ----
FROM golang:1.25 AS build
WORKDIR /src
COPY . .
ENV CGO_ENABLED=0 GOOS=linux GOFLAGS=-mod=vendor GOPROXY=off
RUN go build -trimpath -ldflags="-s -w" -o /mcp ./cmd/mcp

# ---- runtime ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /mcp /mcp
EXPOSE 8080
ENTRYPOINT ["/mcp"]
