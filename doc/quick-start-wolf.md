```yml
services:
  wolf:
    image: ghcr.io/games-on-whales/wolf:stable
    volumes:
      - /etc/wolf/:/etc/wolf
      - /var/run/docker.sock:/var/run/docker.sock:rw
      - /dev/:/dev/:rw
      - /run/udev:/run/udev:rw
    device_cgroup_rules:
      - 'c 13:* rmw'
    devices:
      - /dev/dri
      - /dev/uinput
      - /dev/uhid
    network_mode: host
    restart: unless-stopped

  webscreen:
    image: dukihiroi/webscreen:latest
    container_name: webscreen
    restart: unless-stopped
    network_mode: host
    environment:
      - GIN_MODE=release
      - PIN=123456
      #- PORT=8079
    depends_on:
      - wolf
```