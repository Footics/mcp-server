# footics-mcp — multi-stage build → tiny static binary (same pattern as footics-api).

# ---- build ----
FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ENV CGO_ENABLED=0 GOOS=linux
RUN go build -trimpath -ldflags="-s -w" -o /mcp ./cmd/mcp

# ---- runtime ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /mcp /mcp
EXPOSE 8080
ENTRYPOINT ["/mcp"]
