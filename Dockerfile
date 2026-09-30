# Imagen de cualquier servicio de JAPpi:
#   docker build --build-arg SERVICE=deployer -t jappi-deployer .
# Binario estático sobre distroless: sin shell ni gestor de paquetes, y sin root.

FROM golang:1.27-alpine AS build
ARG SERVICE
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY pkg ./pkg
COPY services ./services
RUN test -n "$SERVICE" && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/app ./services/${SERVICE}/cmd/${SERVICE}

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/app /app
USER nonroot:nonroot
ENTRYPOINT ["/app"]
