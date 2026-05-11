ARG GO_IMAGE=golang:1.25-alpine
ARG ALPINE_IMAGE=alpine:3.22

FROM --platform=$BUILDPLATFORM ${GO_IMAGE} AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal

ARG TARGETOS=linux
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-$(go env GOARCH)} \
  go build -trimpath -ldflags="-s -w" -o /out/backend-api ./cmd/api
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-$(go env GOARCH)} \
  go build -trimpath -ldflags="-s -w" -o /out/backend-worker ./cmd/worker

FROM ${ALPINE_IMAGE} AS runtime

RUN apk add --no-cache ca-certificates curl tzdata \
  && addgroup -S app \
  && adduser -S -D -H -G app app

USER app:app
WORKDIR /app

FROM runtime AS api
COPY --from=build /out/backend-api /usr/local/bin/backend-api
EXPOSE 8080
ENTRYPOINT ["backend-api"]

FROM runtime AS worker
COPY --from=build /out/backend-worker /usr/local/bin/backend-worker
ENTRYPOINT ["backend-worker"]
