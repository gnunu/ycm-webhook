FROM golang:1.17 AS build

ENV GOOS=linux
ENV GOARCH=amd64
ENV CGO_ENABLED=0

WORKDIR /work
COPY . /work

# Build ycm-webhook
RUN --mount=type=cache,target=/root/.cache/go-build,sharing=private \
  go build -o bin/pod-validator .

# ---
FROM scratch AS run

COPY --from=build /work/bin/pod-validator /usr/local/bin/

CMD ["pod-validator"]
