FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/notesd .

FROM alpine:3.20
RUN apk add --no-cache git ca-certificates tini && \
    addgroup -S -g 1000 notesd && adduser -S -G notesd -u 1000 notesd
COPY --from=build /out/notesd /usr/local/bin/notesd
ENV STORAGE_PATH=/data PORT=3333
RUN mkdir -p /data && chown notesd:notesd /data
USER notesd
VOLUME ["/data"]
EXPOSE 3333
ENTRYPOINT ["/sbin/tini", "--", "notesd"]
