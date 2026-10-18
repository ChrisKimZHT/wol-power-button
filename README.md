# wol-power-button

A lightweight Wake-on-LAN power button with a Go backend and an HTML frontend.

```sh
docker run -d \
  --name wol-power-button \
  --restart unless-stopped \
  --network host \
  -e WEB_PORT=8080 \
  chriskimzht/wol-power-button:0
```
