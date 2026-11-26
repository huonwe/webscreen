FROM alpine:latest

ARG TARGETARCH
ARG BINARY_PATH=dist/webscreen-linux-${TARGETARCH}

RUN apk add --no-cache android-tools v4l-utils

WORKDIR /app

COPY ${BINARY_PATH} ./webscreen
RUN chmod +x ./webscreen

ENTRYPOINT ["./webscreen"]