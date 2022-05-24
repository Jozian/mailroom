FROM golang:1.18-buster AS builder
ENV CGO_ENABLED=0
ARG COMPILE_FLAGS
WORKDIR /root/mailroom
COPY go.mod /root/mailroom/go.mod
COPY go.sum /root/mailroom/go.sum
RUN go mod download
COPY . /root/mailroom
RUN apt-get -yq update \
        && DEBIAN_FRONTEND=noninteractive apt-get install -y \
                curl \
                coreutils \
                grep
RUN go build -ldflags "${COMPILE_FLAGS}" -o mailroom ./cmd/mailroom \
            && go build -ldflags "${COMPILE_FLAGS}" -o test-smtp ./cmd/test-smtp
RUN export GOFLOW_VERSION=$(grep goflow go.mod | cut -d" " -f2 | cut -c2-) \
            && curl -L https://github.com/nyaruka/goflow/releases/download/v${GOFLOW_VERSION}/docs.tar.gz \
                | tar zxv \
            && cp ./docs/en-us/*.* docs/

FROM debian:buster AS mailroom
RUN adduser --uid 1000 --disabled-password --gecos '' --home /srv/mailroom mailroom
RUN apt-get -yq update \
        && DEBIAN_FRONTEND=noninteractive apt-get install -y \
                unattended-upgrades \
                # ssl certs to external services
                ca-certificates \
        && rm -rf /var/lib/apt/lists/* \
        && apt-get clean
# Make directory so it has correct ownership
RUN install -o mailroom -d /srv/mailroom/mailroom
WORKDIR /srv/mailroom/mailroom
COPY --from=builder /root/mailroom/mailroom /usr/bin/
COPY --from=builder /root/mailroom/test-smtp /usr/bin/
COPY --chown=mailroom --from=builder /root/mailroom/docs/ /srv/mailroom/mailroom/docs/
COPY entrypoint /usr/bin/
EXPOSE 8090
USER mailroom
ENTRYPOINT ["entrypoint"]
