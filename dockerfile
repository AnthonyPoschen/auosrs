FROM node:latest as frontend
WORKDIR /code
COPY . .
RUN npm install
RUN npx parcel build --dist-dir dist --no-content-hash
RUN cp -R ./src/ico ./dist/ico

# ----
FROM golang:alpine as builder
WORKDIR /code
RUN go install github.com/a-h/templ/cmd/templ@latest
COPY --from=frontend /code /code
RUN templ generate
RUN go test ./... 
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o main ./main.go
# ----
FROM alpine:latest as final
RUN apk add --no-cache ca-certificates
RUN addgroup -g 1000 -S app && adduser -u 1000 -S app -G app
COPY --chown=app:app --from=builder /code/main /app
USER app
CMD ["/app"]
