# syntax=docker/dockerfile:1
FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go test ./... && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/party-api ./cmd/api && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/party-migrate ./cmd/migrate
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/party-api /party-api
COPY --from=build /out/party-migrate /party-migrate
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/party-api"]
