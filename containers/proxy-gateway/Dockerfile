FROM alpine:3.20

RUN apk add --no-cache \
    curl \
    bash

RUN curl -sL https://github.com/ginuerzh/gost/releases/download/v2.11.5/gost-linux-amd64-2.11.5.gz \
    | gunzip > /usr/local/bin/gost && chmod +x /usr/local/bin/gost

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 51820/udp 1080/tcp
ENTRYPOINT ["/entrypoint.sh"]
