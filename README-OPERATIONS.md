# RC Car Server: эксплуатация на mini-PC

Основной сервис работает в Dockge на mini-PC. Raspberry Pi принимает старый
поток телеметрии ESP32 на UDP/4211 и пересылает его на mini-PC. Этот relay можно
удалить после изменения адреса телеметрии в прошивке ESP32 на
`192.168.1.28:4211`.

## Адреса

- Web: `https://car.minipc.home`
- mini-PC backend: `192.168.1.28:18081`
- мотор: `192.168.1.15:4210/udp`
- телеметрия mini-PC: `192.168.1.28:4211/udp`
- камера: `http://192.168.1.16/stream`

## Управление основным сервисом

```bash
ssh minipc
cd /opt/stacks/rc-car-server
docker compose up -d --build
docker compose ps
docker compose logs --tail=100 rc-car-server
docker compose down
```

Проверка:

```bash
curl -fsS http://192.168.1.28:18081/health
curl -fsS http://192.168.1.28:18081/api/state
```

## UDP relay на Raspberry Pi

```bash
ssh raps
cd /home/ivan/rc-car-server
docker compose -f compose.telemetry-relay.yaml up -d --build
docker compose -f compose.telemetry-relay.yaml ps
docker compose -f compose.telemetry-relay.yaml logs --tail=100
```

## Обновление

Сначала обновить и проверить репозиторий локально, затем запушить один commit в
GitHub и локальный GitLab. На mini-PC:

```bash
cd /opt/stacks/rc-car-server
git pull --ff-only github main
docker compose build --pull
docker compose up -d
docker compose ps
```

## Откат

```bash
ssh minipc 'cd /opt/stacks/rc-car-server && docker compose down'
ssh raps 'sudo systemctl enable --now rc-car-server.service'
```

После отката Caddy можно временно направить на `192.168.1.25:8081`. Сервис
`rc-car-routing.service` останавливать нельзя: он обслуживает общую VPN/AP
маршрутизацию и используется `vpn-panel`.
