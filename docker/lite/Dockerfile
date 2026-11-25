ARG TARGETARCH
ARG BINARY_PATH=dist/webscreen-linux-${TARGETARCH}

FROM alpine:latest

RUN apk add --no-cache android-tools v4l-utils

WORKDIR /app
ARG BINARY_PATH
COPY ${BINARY_PATH} /app/webscreen

RUN chmod +x /app/webscreen

ENV PIN="123456"
ENV PORT=8079
EXPOSE $PORT

ENTRYPOINT ["sh", "-c", "./webscreen -port ${PORT} -pin ${PIN}"]