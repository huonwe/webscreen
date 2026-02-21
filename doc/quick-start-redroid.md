# Quick Start with Redroid and Docker Compose

This guide helps you set up **WebScreen** alongside **Redroid** (Remote Android) using `docker-compose`. This allows you to run a virtual Android instance and control it via your browser without needing a physical device.

## Docker Compose Configuration

Create a file named `docker-compose.yml` with the following content:

```yaml
services:
  webscreen:
    image: dukihiroi/webscreen:latest
    container_name: webscreen
    restart: unless-stopped
    ports:
      - "8079:8079"
      # udp range for WebRTC if needed, currently using host network is easier for webrtc
      # - "51200-51299:51200-51299/udp"
    environment:
      - GIN_MODE=release
      - PIN=123456
    depends_on:
      - redroid

  redroid:
    image: redroid/redroid:16.0.0-latest
    container_name: redroid
    privileged: true
    restart: unless-stopped
    ports:
      - "5555:5555"
```

## Running the Stack

1.  Start the services:
    ```bash
    docker-compose up -d
    ```

2.  Check the logs to ensure connection:
    ```bash
    docker-compose logs -f webscreen
    ```
    You should see `Connected to redroid!` eventually.

## Accessing the Interface

1.  Open your browser and navigate to:
    [http://localhost:8079](http://localhost:8079)
    
2.  Enter the PIN configured in the docker-compose file (default: `123456`).

3.  Inside the console, you should see the `redroid` device listed. Click to connect and control. If you can't see it, connect the device (ip: `redroid`, port: `5555`) manually.

## TroubleShooting

- **Redroid keeps restarting or fails to start**:
  Check if kernel modules are loaded correctly: `lsmod | grep -E 'binder|ashmem'`.
  
- **WebScreen cannot connect**:
  Ensure both containers are on the same network (docker-compose creates a default network automatically).
  You can manually connect by entering the webscreen container:
  ```bash
  docker exec -it webscreen adb connect redroid:5555
  ```
