FROM golang:1.23-bookworm AS build
WORKDIR /src
COPY go.mod ./
COPY *.go ./
RUN CGO_ENABLED=0 go build -o /balancer .

FROM gcr.io/distroless/static-debian12
COPY --from=build /balancer /balancer
ENTRYPOINT ["/balancer"]
