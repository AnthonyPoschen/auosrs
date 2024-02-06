FROM golang:latest as builder
WORKDIR /code
COPY . .
# RUN go get -u github.com/

FROM alpine:latest
COPY --from=builder /code/main /app
