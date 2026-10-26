
FROM golang:1.26.8-alpine AS build

WORKDIR /src

COPY go.mod ./
COPY *.go ./

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/spiffe-checker \
    .

FROM scratch

COPY --from=build /out/spiffe-checker /spiffe-checker

USER 65532:65532

ENTRYPOINT ["/spiffe-checker"]
