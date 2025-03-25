# syntax=docker/dockerfile:1

FROM golang:1.23 AS go-builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . ./

# Enable CGO because fitz relies on c code.
RUN CGO_ENABLED=1 GOOS=linux go build -o /smart-docs

FROM python:3.11-slim

WORKDIR /app

# Install system dependencies required for pdfplumber
RUN apt-get update && apt-get install -y \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

# Install Python dependencies
RUN pip install pdfplumber

# Copy the Go binary from the builder stage
COPY --from=go-builder /smart-docs /smart-docs

# Create directory for file storage and set permissions
RUN mkdir -p /data && chmod 777 /data

EXPOSE 8080

CMD ["/smart-docs"]
