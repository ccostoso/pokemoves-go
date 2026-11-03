# Match the version in go.mod
ARG GO_VERSION=1.27.1

# Build stage for compiling the Go binary.
# Use the official Golang image as the build environment.
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src

# Set the working directory inside the container.
COPY go.mod go.sum* ./
RUN go mod download

# Copy the source code into the container and build the binary.
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# Use a minimal base image for the final container.
# Copy the compiled Go binary from the build stage.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/server /server
EXPOSE 8080
ENTRYPOINT ["/server"]