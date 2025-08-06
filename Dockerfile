FROM golang:1.21-alpine

# نصب وابستگی‌ها
RUN apk add --no-cache \
    openjdk17-jre \
    bash \
    && apk add --no-cache -X http://dl-cdn.alpinelinux.org/alpine/edge/community openjdk17-jdk

# تنظیم محیط
WORKDIR /app
COPY . .

# ساخت و اجرای برنامه با تنظیمات دلخواه
RUN go mod init apk-signer-bot && go mod tidy
RUN go build -tags netgo -ldflags '-s -w' -o bot main.go

ENV PORT=5000
ENV KEYSTORE_PATH=/app/my.keystore
ENV KEYSTORE_PASSWORD=123456
ENV KEY_ALIAS=mykey
ENV KEY_PASSWORD=123456

CMD ["./bot"]
