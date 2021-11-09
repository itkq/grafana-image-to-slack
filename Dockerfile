FROM golang:1.17 as build

WORKDIR /app
COPY go.* ./
RUN go mod download
COPY . .
RUN go build -o server

FROM gcr.io/distroless/base-debian10
WORKDIR /
COPY --from=build /app/server /server
USER nonroot:nonroot
CMD ["/server"]
