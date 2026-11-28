FROM alpine:latest

ARG TARGETARCH
ARG BINARY_PATH=dist/scrmux-linux-${TARGETARCH}

RUN apk add --no-cache android-tools v4l-utils

WORKDIR /scrmux

COPY ${BINARY_PATH} ./scrmux
RUN chmod +x ./scrmux

ENTRYPOINT ["./scrmux"]