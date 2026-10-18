# wol-power-button

基于 Go 后端和 HTML 前端的轻量级 Wake on LAN 电源按钮。

```sh
docker run -d \
  --name wol-power-button \
  --restart unless-stopped \
  --network host \
  chriskimzht/wol-power-button:0
```
