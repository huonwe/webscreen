FROM alpine:latest

RUN apk add --no-cache android-tools

WORKDIR /app

ARG TARGETARCH
ARG BINARY_PATH=dist/webscreen-linux-${TARGETARCH}

COPY ${BINARY_PATH} ./webscreen

ENV PORT=8079
EXPOSE $PORT

ENTRYPOINT ./webscreen -port $PORT