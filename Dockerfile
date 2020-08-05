FROM golang:1.14-buster AS builder
ENV CGO_ENABLED=0
ARG COMPILE_FLAGS
WORKDIR /root/mailroom
COPY . /root/mailroom
RUN go build -ldflags "${COMPILE_FLAGS}" -o mailroom ./cmd/mailroom \
            && go build -ldflags "${COMPILE_FLAGS}" -o test-smtp ./cmd/test-smtp

FROM debian:buster AS mailroom
RUN adduser --uid 1000 --disabled-password --gecos '' --home /srv/mailroom mailroom
RUN apt-get -yq update \
        && DEBIAN_FRONTEND=noninteractive apt-get install -y \
                unattended-upgrades \
        && rm -rf /var/lib/apt/lists/* \
        && apt-get clean
COPY --from=builder /root/mailroom/mailroom /usr/bin/
COPY --from=builder /root/mailroom/test-smtp /usr/bin/
COPY entrypoint /usr/bin/
EXPOSE 8090
USER mailroom
ENTRYPOINT ["/usr/bin/entrypoint"]
