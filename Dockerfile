# Start from the latest alpine based golang base image
FROM golang:alpine as builder

# Install git
RUN apk update && apk add --no-cache git

# Add maintainer info
LABEL maintainer="Matthias Ladkau <matthias@ladkau.de>"

# Set the current working directory inside the container
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source from the current directory to the working directory inside the container
COPY . .

# Build rufs and link statically (no CGO)
# Use ldflags -w -s to omit the symbol table, debug information and the DWARF table
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -ldflags="-w -s" ./server/dudeldu.go

# Start again from scratch
FROM scratch

# Copy the rufs binary
COPY --from=builder /app/dudeldu /dudeldu

# Set the working directory to data so all created files (e.g. rufs.config.json)
# can be mapped to physical files on disk
WORKDIR /data

# Run eliasdb binary
ENTRYPOINT ["../dudeldu"]

# To run the dudeldu as the current user, expose port 9091 and map
# all files in the current directory run:
#
# docker run --rm --user $(id -u):$(id -g) -v $PWD:/data -p 9091:9091 krotik/dudeldu -host 0.0.0.0 <playlist>
