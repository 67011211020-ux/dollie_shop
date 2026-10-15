FROM golang:1.26 AS build
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /dollie .

FROM alpine:3.21
WORKDIR /app

COPY --from=build /dollie /app/dollie
COPY --from=build /app/templates /app/templates
COPY --from=build /app/static /app/static
COPY --from=build /app/ca.pem /app/ca.pem

EXPOSE 8080
CMD ["/app/dollie"]
