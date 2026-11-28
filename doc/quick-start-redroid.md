# Quick Start with Redroid and Docker Compose

This guide helps you set up **scrmux** alongside **Redroid** (Remote Android) using `docker-compose`. This allows you to run a virtual Android instance and control it via your browser.

## Docker Compose Configuration

Create a file named `docker-compose.yml` with the following content:

```yaml
services:
  scrmux:
    image: dukihiroi/scrmux:latest
    container_name: scrmux
    restart: unless-stopped
    network_mode: host
    volumes:
      - ./data:/scrmux/data
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

## Start the Service

```bash
docker compose up -d
```

## Accessing the Interface

1.  Open your browser and navigate to:
    [http://localhost:8081](http://localhost:8081)
    
2.  You might want to edit ./data/config.yml. Remember to change jwt secret!

3.  Inside the console, you should see the `redroid` device listed. Click to connect and control. If you can't see it, connect the device (ip: `127.0.0.1` if host mode, else `redroid`, port: `5555`) manually.

## TroubleShooting

- **Redroid keeps restarting or fails to start**:
  Check if kernel modules are loaded correctly: `lsmod | grep -E 'binder|ashmem'`.
  
- **Webwcreen cannot connect**:
  Ensure both containers are on the same network (docker-compose creates a default network automatically).
  You can manually connect by entering the scrmux container:
  ```bash
  docker exec -it scrmux adb connect redroid:5555
  ```
