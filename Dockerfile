FROM golang:1.21-alpine

# نصب وابستگی‌ها
RUN apk add --no-cache \
    openjdk17-jre \
    bash \
    && apk add --no-cache -X http://dl-cdn.alpinelinux.org/alpine/edge/community openjdk17-jdk

# تنظیم محیط
WORKDIR /app
COPY . .

# دیباگ: چک کردن فایل‌ها
RUN echo "Listing files in /app:" && ls -la

# ساخت و اجرای برنامه با تنظیمات دلخواه و دیباگ
RUN echo "Initializing Go module..." && go mod init apk-signer-bot && go mod tidy
RUN echo "Building binary..." && go build -tags netgo -ldflags '-s -w' -o bot main.go || { echo "Build failed, check logs"; exit 1; }
RUN echo "Verifying binary..." && ls -la bot

ENV PORT=5000
ENV KEYSTORE_PATH=/app/my.keystore
ENV KEYSTORE_PASSWORD=123456
ENV KEY_ALIAS=mykey
ENV KEY_PASSWORD=123456

CMD ["./bot"]
