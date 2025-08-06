FROM golang:1.21

# نصب وابستگی‌ها
RUN apt-get update && apt-get install -y \
    openjdk-17-jre \
    openjdk-17-jdk \
    && apt-get clean

# تنظیم محیط
WORKDIR /app
COPY . .

# دیباگ: چک کردن فایل‌ها
RUN echo "Listing files in /app:" && ls -la

# ساخت و اجرای برنامه با تنظیمات دلخواه و دیباگ
RUN echo "Initializing Go module..." && go mod init apk-signer-bot && go mod tidy
RUN echo "Building binary..." && go build -tags netgo -ldflags '-s -w' -o bot main.go || { echo "Build failed, check logs:"; cat /app/*.log 2>/dev/null || echo "No log file"; exit 1; }
RUN echo "Verifying binary..." && ls -la bot || echo "Binary not found!"

ENV PORT=5000
ENV KEYSTORE_PATH=/app/my.keystore
ENV KEYSTORE_PASSWORD=123456
ENV KEY_ALIAS=mykey
ENV KEY_PASSWORD=123456

CMD ["./bot"]
