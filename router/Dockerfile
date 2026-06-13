# Build from /home/noname/Desktop/Projects so the sibling gollm checkout is in context:
# docker build -f AetherCode/router/Dockerfile -t aethercode-router:local .
FROM golang:1.25-bookworm AS build

WORKDIR /src
COPY gollm ./gollm
COPY AetherCode/router/go.mod AetherCode/router/go.sum ./AetherCode/router/
COPY AetherCode/router/cmd ./AetherCode/router/cmd
COPY AetherCode/router/internal ./AetherCode/router/internal

WORKDIR /src/AetherCode/router
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/router ./cmd/router
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/mock-provider ./cmd/mock-provider

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/router /router
COPY --from=build /out/mock-provider /mock-provider
EXPOSE 8080
ENTRYPOINT ["/router"]
