FROM golang:1.16-buster AS builder
ENV CGO_ENABLED=0
ARG COMPILE_FLAGS
WORKDIR /root/mailroom
COPY . /root/mailroom
RUN apt-get -yq update \
        && DEBIAN_FRONTEND=noninteractive apt-get install -y \
                curl \
                coreutils \
                grep
RUN go build -ldflags "${COMPILE_FLAGS}" -o mailroom ./cmd/mailroom \
            && go build -ldflags "${COMPILE_FLAGS}" -o test-smtp ./cmd/test-smtp
RUN export GOFLOW_VERSION=$(grep goflow go.mod | cut -d" " -f2 | cut -c2-) \
            && curl https://codeload.github.com/nyaruka/goflow/tar.gz/v${GOFLOW_VERSION} \
                | tar --wildcards --strip=1 -zx "goflow-${GOFLOW_VERSION}/docs/*" \
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
WORKDIR /srv/mailroom/mailroom
COPY --from=builder /root/mailroom/mailroom /usr/bin/
COPY --from=builder /root/mailroom/test-smtp /usr/bin/
COPY --chown=mailroom --from=builder /root/mailroom/docs/ /srv/mailroom/mailroom/docs/
COPY entrypoint /usr/bin/
EXPOSE 8090
USER mailroom
ENTRYPOINT ["entrypoint"]
