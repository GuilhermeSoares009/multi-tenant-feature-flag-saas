FROM golang:1.22 AS build
WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /out/feature-flags ./cmd/featureflags

FROM gcr.io/distroless/base-debian12
WORKDIR /
COPY --from=build /out/feature-flags /feature-flags
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/feature-flags"]
