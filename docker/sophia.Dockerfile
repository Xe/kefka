ARG GO_VERSION=1.26
ARG ALPINE_VERSION=3.23

FROM --platform=${BUILDPLATFORM} golang:${GO_VERSION}-alpine AS build

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src/xeiaso.net/kefka

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN --mount=type=cache,target=/root/.cache GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -gcflags "all=-N -l" -o /app/bin/sophia ./cmd/sophia

FROM alpine:${ALPINE_VERSION} AS run
WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY --from=build /app/bin/sophia /app/bin/sophia

EXPOSE 2222

CMD ["/app/bin/sophia"]

LABEL org.opencontainers.image.source="https://tangled.org/xeiaso.net/sophia"
LABEL org.opencontainers.image.title="Sophia"
LABEL org.opencontainers.image.description="An SSH server backed by Tigris"
