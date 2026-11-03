# Match the version in go.mod
ARG GO_VERSION=1.27.1

# Build stage for compiling the Go binary.
FROM golang:${GO_VERSION} AS build
WORKDIR /src

# Dependencies first, so this layer is cached until go.mod/go.sum change
COPY go.mod go.sum* ./
RUN go mod download

# Copy the source code and build the Go binary.
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# Use a minimal base image for the final container.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/server /server
EXPOSE 8080
ENTRYPOINT ["/server"]