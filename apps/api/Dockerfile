FROM golang:1.23-alpine AS build
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/tanban-api ./cmd/server

FROM alpine:3.21
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories \
    && apk add --no-cache ca-certificates tzdata \
    && addgroup -S tanban \
    && adduser -S -G tanban tanban
WORKDIR /app
COPY --from=build /out/tanban-api /app/tanban-api
COPY migrations /app/migrations
USER tanban
EXPOSE 18090
ENTRYPOINT ["/app/tanban-api"]
