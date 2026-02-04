FROM docker.io/library/golang:1.25.6-alpine AS build

WORKDIR /src

RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" \
    -o /out/komodo-exporter ./cmd/komodo-exporter


FROM gcr.io/distroless/static:nonroot

WORKDIR /

COPY --from=build /out/komodo-exporter /komodo-exporter

EXPOSE 9109

ENTRYPOINT ["/komodo-exporter"]
