FROM golang:1.13 AS builder

WORKDIR /usr/src/app

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./
RUN env CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"'

FROM alpine
RUN mkdir -p /run/docker/plugins /mnt/state /mnt/volumes
COPY --from=builder /usr/src/app/docker-dobs-volume-driver /
CMD ["/docker-dobs-volume-driver"]
