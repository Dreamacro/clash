FROM golang:alpine as builder

RUN apk add --no-cache make git && \
    wget http://geolite.maxmind.com/download/geoip/database/GeoLite2-Country.tar.gz -O /tmp/GeoLite2-Country.tar.gz && \
    tar zxvf /tmp/GeoLite2-Country.tar.gz -C /tmp && \
    mv /tmp/GeoLite2-Country_*/GeoLite2-Country.mmdb /Country.mmdb
WORKDIR /clash-src
COPY . /clash-src
RUN go mod download && \
    make linux-amd64 && \
    mv ./bin/clash-linux-amd64 /clash

FROM alpine:latest

RUN apk add --no-cache ca-certificates
COPY --from=builder /Country.mmdb /root/.config/clash/
COPY --from=builder /clash /
ENTRYPOINT ["/clash"]
