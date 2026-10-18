FROM golang:1 AS build

WORKDIR /src
COPY go.mod ./
COPY *.go ./
COPY web/ ./web/

ARG VERSION=dev
RUN go test ./... && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/wol-power-button .

FROM scratch

COPY --from=build /out/wol-power-button /wol-power-button
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/wol-power-button"]
