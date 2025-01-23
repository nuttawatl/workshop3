# Stage 1: Build
FROM golang:1.21.5-alpine3.19 AS builder
WORKDIR /service
# Ensure that the go.mod file is present in the ./service directory
# If not using Go modules, comment out the next line
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -a -o api .

# Stage 2: Run
FROM alpine:3.19
RUN apk --no-cache add ca-certificates sqlite
WORKDIR /root/
COPY --from=builder /service/api .
EXPOSE 8080
CMD [ "./api" ]
