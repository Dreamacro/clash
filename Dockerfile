FROM golang:latest as builder

RUN wget http://geolite.maxmind.com/download/geoip/database/GeoLite2-Country.tar.gz -O /tmp/GeoLite2-Country.tar.gz && \
    tar zxvf /tmp/GeoLite2-Country.tar.gz -C /tmp && \
    mv /tmp/GeoLite2-Country_*/GeoLite2-Country.mmdb /Country.mmdb

WORKDIR /clash-src

COPY . /clash-src

RUN go mod download && \
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags '-w -s' -o /clash

FROM alpine:latest

RUN apk add --no-cache ca-certificates

COPY --from=builder /Country.mmdb /root/.config/clash/
COPY --from=builder /clash /

EXPOSE 7890 7891

ENTRYPOINT ["/clash"]
