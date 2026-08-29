ARG STELE_GO_IMAGE=golang:1.25-bookworm
ARG STELE_RUNTIME_IMAGE=debian:bookworm-slim
ARG STELE_GOPROXY=https://proxy.golang.org,direct
FROM ${STELE_GO_IMAGE} AS build
ARG STELE_GOPROXY=https://proxy.golang.org,direct

WORKDIR /src

COPY go.mod go.sum ./
ENV GOPROXY=${STELE_GOPROXY}
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/stele ./cmd/stele

FROM ${STELE_RUNTIME_IMAGE}

RUN apt-get update \
	&& apt-get install -y --no-install-recommends ca-certificates \
	&& rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=build /out/stele /usr/local/bin/stele

USER 65532:65532

ENTRYPOINT ["/usr/local/bin/stele"]
