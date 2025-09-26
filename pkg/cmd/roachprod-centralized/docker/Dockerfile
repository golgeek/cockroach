FROM golang:1.24-bullseye as builder

# Copy go.mod and go.sum files
COPY go.mod go.sum /build/

# Set the working directory
WORKDIR /build

# Get the dependencies
# This will cache the dependencies layer, so subsequent builds are faster
RUN go mod download

# Copy the source code
COPY pkg /build/pkg

# Build the roachprod-centralized binary
RUN go build -v -o /build/roachprod-centralized ./pkg/cmd/roachprod-centralized


# Build the final image
FROM golang:1.24-bullseye

# Copy entrypoint and build scripts
COPY ./pkg/cmd/roachprod-centralized/docker/entrypoint.sh ./pkg/cmd/roachprod-centralized/docker/build.sh /build/

# Run the build script to install final image dependencies
# then remove the build script
RUN ["/build/build.sh"]
RUN rm /build/build.sh

# Copy the roachprod-centralized binary from the builder stage
COPY --from=builder /build/roachprod-centralized /usr/local/bin/roachprod-centralized

# Set the entrypoint
ENTRYPOINT ["/build/entrypoint.sh"]
