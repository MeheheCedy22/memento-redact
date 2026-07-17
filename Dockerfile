FROM golang:1.20-alpine AS build
WORKDIR /src
COPY go.mod .
COPY . .
RUN go build -o /bin/memento-redact ./cmd/server

FROM alpine:3.18
COPY --from=build /bin/memento-redact /bin/memento-redact
EXPOSE 8080
ENTRYPOINT ["/bin/memento-redact"]
