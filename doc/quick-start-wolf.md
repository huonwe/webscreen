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
## How to enable gamepad?
1. Configure host udev rule
```bash
sudo curl -fsSL https://raw.githubusercontent.com/games-on-whales/wolf/stable/85-wolf.rules -o /etc/udev/rules.d/85-wolf.rules
sudo udevadm control --reload-rules && sudo udevadm trigger

```

2. Mount device to Wolf Container
```yml
volumes:
  - /etc/wolf:/etc/wolf
  - /var/run/docker.sock:/var/run/docker.sock
  - /dev/:/dev/
devices:
  - /dev/uinput # ADD THIS
  - /dev/uhid # ADD THIS
device_cgroup_rules:
  - 'c 13:* rmw' # ADD THIS

```