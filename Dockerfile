# syntax=docker/dockerfile:1

FROM golang:1.23

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . ./

# Create directory for file storage and set permissions
RUN mkdir -p /data && chmod 777 /data

# Enable CGO because fitz relies on c code.
RUN CGO_ENABLED=1 GOOS=linux go build -o /smart-docs

EXPOSE 8080

CMD ["/smart-docs"]
