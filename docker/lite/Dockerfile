FROM alpine:latest

ARG TARGETARCH
ARG BINARY_PATH=dist/scrmux-linux-${TARGETARCH}

RUN apk add --no-cache android-tools v4l-utils ca-certificates tzdata
WORKDIR /scrmux

COPY --chmod=755 ${BINARY_PATH} ./scrmux

ENV DATA_DIR="./data"


ENTRYPOINT ["./scrmux"]