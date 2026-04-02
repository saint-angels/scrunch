FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 go build -o /scrunch-server .

FROM alpine:3.21
COPY --from=build /scrunch-server /scrunch-server
EXPOSE 9000
CMD ["/scrunch-server"]
