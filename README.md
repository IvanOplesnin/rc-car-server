# RC Car Server

Backend-сервер для удаленного управления гусеничной машинкой через веб-интерфейс.

Система строится вокруг Raspberry Pi 5, который работает как центральный сервер управления, мониторинга, видеопрокси и VPN-точка доступа. Пользователь подключается к Raspberry Pi 5 только через VPN, открывает веб-интерфейс и управляет машинкой из браузера.

---

## Архитектура

```text
[Пользователь]
     |
     | VPN
     v
[Raspberry Pi 5]
     |
     | HTTP
     v
[Web UI в браузере]

[Браузер]
     |
     | WebSocket
     v
[Go backend]
     |
     | UDP
     v
[ESP32 motor controller]
     |
     | PWM / GPIO
     v
[Драйверы моторов]
     |
     v
[Левая и правая гусеница]

[ESP32-CAM]
     |
     | MJPEG over HTTP
     v
[Go backend / video proxy]
     |
     v
[Web UI]
```

---

## Основные компоненты

```text
Raspberry Pi 5:
    Go backend
    HTTP server
    WebSocket server
    UDP client for motor commands
    UDP listener for telemetry
    MJPEG camera proxy
    VPN server

ESP32:
    управление моторами
    прием UDP-команд
    отправка UDP-телеметрии
    watchdog аварийной остановки

ESP32-CAM:
    видеопоток MJPEG over HTTP

Frontend:
    HTML
    CSS
    JavaScript
    WebSocket для управления
    /video/stream для картинки камеры
```

---

## Сетевые каналы

```text
Browser -> Raspberry Pi 5:
    HTTP — получение веб-интерфейса
    WebSocket — команды управления и состояние

Raspberry Pi 5 -> ESP32 motors:
    UDP — команды движения и управления устройствами

ESP32 motors -> Raspberry Pi 5:
    UDP — телеметрия

ESP32-CAM -> Raspberry Pi 5:
    HTTP MJPEG — видеопоток

Browser -> Raspberry Pi 5:
    HTTP /video/stream — видеопоток через backend proxy
```

---

## Конфигурация

Пример `configs/config.yaml`:

```yaml
server:
  host: "0.0.0.0"
  port: 8080

web:
  static_dir: "web"

motor:
  address: "127.0.0.1:4210"

telemetry:
  listen_address: "0.0.0.0:4211"
  motor_timeout_ms: 1500

camera:
  stream_url: "http://192.168.1.60/stream"
  check_interval_ms: 3000
  check_timeout_ms: 1000

safety:
  command_timeout_ms: 500
```

---

## Общий формат сообщений

Для WebSocket, UDP-команд и UDP-телеметрии используется расширяемый JSON-формат:

```json
{
  "version": 1,
  "type": "message_type",
  "seq": 1,
  "timestamp": 1710000000000,
  "payload": {}
}
```

Поля:

```text
version   — версия протокола
type      — тип сообщения
seq       — порядковый номер сообщения
timestamp — Unix time в миллисекундах
payload   — полезная нагрузка сообщения
```

---

## Типы сообщений

```text
control    — команда управления
system     — системная команда
telemetry  — телеметрия от ESP32
state      — состояние от backend к браузеру
error      — сообщение об ошибке
```

---

## WebSocket protocol

Канал:

```text
Browser -> Raspberry Pi 5
/ws
```

Назначение:

```text
передача команд управления из браузера в backend;
получение текущего состояния системы.
```

### Команда движения

```json
{
  "version": 1,
  "type": "control",
  "seq": 1,
  "timestamp": 1710000000000,
  "payload": {
    "drive": {
      "left": 70,
      "right": 70
    }
  }
}
```

Поля `drive`:

```text
left  — скорость левой гусеницы от -100 до 100
right — скорость правой гусеницы от -100 до 100
```

Примеры движения:

```text
left = 80, right = 80       движение вперед
left = -80, right = -80     движение назад
left = 80, right = 30       плавный поворот вправо
left = 30, right = 80       плавный поворот влево
left = 80, right = -80      разворот на месте
left = 0, right = 0         остановка
```

### Команда stop

```json
{
  "version": 1,
  "type": "system",
  "seq": 2,
  "timestamp": 1710000000100,
  "payload": {
    "system": {
      "command": "stop",
      "reason": "button_up"
    }
  }
}
```

Возможные значения `reason`:

```text
user
button_up
key_up
window_blur
pointer_cancel
pointer_leave
ws_close
timeout
watchdog
emergency
```

### Команда emergency_stop

```json
{
  "version": 1,
  "type": "system",
  "seq": 3,
  "timestamp": 1710000000200,
  "payload": {
    "system": {
      "command": "emergency_stop",
      "reason": "stop_button"
    }
  }
}
```

При `emergency_stop` backend должен немедленно остановить движение. В дальнейшем эта команда может также отключать опасные механизмы.

---

## State message

Канал:

```text
Raspberry Pi 5 -> Browser
WebSocket /ws
```

Пример сообщения состояния:

```json
{
  "version": 1,
  "type": "state",
  "seq": 10,
  "timestamp": 1710000000500,
  "payload": {
    "connection": {
      "motor_connected": true,
      "camera_connected": false
    },
    "drive": {
      "left": 70,
      "right": 70
    },
    "power": {
      "battery_voltage": 7.4,
      "battery_percent": 80
    },
    "network": {
      "rssi": -58
    },
    "system": {
      "uptime_ms": 120000,
      "free_heap": 104000
    },
    "safety": {
      "failsafe": false,
      "last_command_valid": true,
      "last_error": ""
    }
  }
}
```

---

## Motor UDP protocol

Канал:

```text
Raspberry Pi 5 -> ESP32 motor controller
UDP
```

Назначение:

```text
быстрая передача команд движения;
передача команд будущим устройствам машинки;
аварийная остановка.
```

### Команда движения

```json
{
  "version": 1,
  "type": "control",
  "seq": 1,
  "timestamp": 1710000000000,
  "payload": {
    "drive": {
      "left": 70,
      "right": 70
    }
  }
}
```

ESP32 должен читать:

```text
payload.drive.left
payload.drive.right
```

Диапазон значений:

```text
left/right: -100 ... 100
```

### Системная команда emergency_stop

```json
{
  "version": 1,
  "type": "system",
  "seq": 2,
  "timestamp": 1710000000100,
  "payload": {
    "system": {
      "command": "emergency_stop",
      "reason": "watchdog"
    }
  }
}
```

При получении `emergency_stop` ESP32 должен:

```text
остановить моторы;
отключить потенциально опасные механизмы;
перейти в безопасное состояние;
отправить телеметрию с failsafe=true.
```

---

## Telemetry UDP protocol

Канал:

```text
ESP32 motor controller -> Raspberry Pi 5
UDP
```

Назначение:

```text
передача состояния моторного контроллера;
мониторинг батареи;
мониторинг Wi-Fi;
проверка доступности ESP32.
```

Пример телеметрии:

```json
{
  "version": 1,
  "type": "telemetry",
  "seq": 100,
  "timestamp": 1710000000000,
  "payload": {
    "motor": {
      "left": 70,
      "right": 70,
      "failsafe": false
    },
    "power": {
      "battery_voltage": 7.4,
      "battery_percent": 80
    },
    "network": {
      "rssi": -58
    },
    "system": {
      "uptime_ms": 120000,
      "free_heap": 104000
    }
  }
}
```

Поля:

```text
motor.left             — текущая скорость левой гусеницы
motor.right            — текущая скорость правой гусеницы
motor.failsafe         — активен ли аварийный режим

power.battery_voltage  — напряжение аккумулятора
power.battery_percent  — примерный заряд аккумулятора

network.rssi           — уровень Wi-Fi сигнала ESP32

system.uptime_ms       — время работы ESP32
system.free_heap       — свободная память ESP32
```

Backend считает ESP32 подключенным, если телеметрия приходит регулярно. Если телеметрия не приходит дольше `telemetry.motor_timeout_ms`, backend устанавливает:

```text
motor_connected = false
```

---

## Camera protocol

Канал:

```text
ESP32-CAM -> Raspberry Pi 5
HTTP MJPEG
```

Backend проксирует поток камеры через endpoint:

```text
GET /video/stream
```

Браузер использует:

```html
<img src="/video/stream" alt="Camera stream">
```

Прямой доступ браузера к ESP32-CAM не требуется. Браузер работает только с Raspberry Pi 5.

---

## HTTP API

### Healthcheck

```text
GET /health
```

Ответ:

```json
{
  "status": "ok",
  "time": "2026-05-04T12:00:00+02:00"
}
```

### Состояние системы

```text
GET /api/state
```

Ответ:

```json
{
  "motor_connected": true,
  "camera_connected": false,
  "left": 70,
  "right": 70,
  "failsafe": false,
  "last_command_valid": true,
  "last_error": "",
  "last_command_at": "2026-05-04T12:00:00+02:00",
  "battery_voltage": 7.4,
  "battery_percent": 80,
  "rssi": -58,
  "uptime_ms": 120000,
  "free_heap": 104000,
  "last_telemetry_at": "2026-05-04T12:00:00+02:00"
}
```

---

## Расширение протокола

Протокол специально построен вокруг `payload`, чтобы можно было добавлять новые блоки без изменения общей структуры.

Будущие блоки:

```text
camera      — управление поворотом камеры
lights      — управление освещением
toy         — игровые механизмы для кошки
sensors     — дополнительные датчики
mode        — режимы движения
```

### Управление поворотом камеры

Пример будущей команды:

```json
{
  "version": 1,
  "type": "control",
  "seq": 200,
  "timestamp": 1710000000000,
  "payload": {
    "camera": {
      "pan": 30,
      "tilt": -10
    }
  }
}
```

Поля:

```text
pan  — поворот камеры влево/вправо
tilt — наклон камеры вверх/вниз
```

Предлагаемые диапазоны:

```text
pan:  -90 ... 90
tilt: -45 ... 45
```

### Управление освещением

Пример будущей команды:

```json
{
  "version": 1,
  "type": "control",
  "seq": 201,
  "timestamp": 1710000000100,
  "payload": {
    "lights": {
      "front": {
        "enabled": true,
        "brightness": 80
      },
      "rear": {
        "enabled": false,
        "brightness": 0
      },
      "camera": {
        "enabled": true,
        "brightness": 60
      },
      "ambient": {
        "enabled": false,
        "brightness": 0
      }
    }
  }
}
```

Поля:

```text
front    — передние фары
rear     — задние фары
camera   — подсветка камеры
ambient  — общее освещение
```

Диапазон яркости:

```text
brightness: 0 ... 100
```

### Игровые механизмы для кошки

Пример команды для сервопривода:

```json
{
  "version": 1,
  "type": "control",
  "seq": 202,
  "timestamp": 1710000000200,
  "payload": {
    "toy": {
      "device": "servo_arm",
      "action": "move",
      "value": 45,
      "duration_ms": 1000
    }
  }
}
```

Пример команды для моторчика с игрушкой:

```json
{
  "version": 1,
  "type": "control",
  "seq": 203,
  "timestamp": 1710000000300,
  "payload": {
    "toy": {
      "device": "teaser_motor",
      "action": "spin",
      "speed": 40,
      "duration_ms": 2000
    }
  }
}
```

Правило безопасности:

```text
любые toy-команды с движущимися механизмами должны иметь duration_ms;
если duration_ms не задан, устройство должно использовать безопасное значение по умолчанию;
после истечения duration_ms механизм должен быть остановлен автоматически.
```

---

## Правила совместимости

1. Если устройство получило неизвестную `version`, оно должно отклонить сообщение.
2. Если устройство получило известную `version`, но неизвестный блок внутри `payload`, оно может проигнорировать этот блок.
3. ESP32 должен обрабатывать только те блоки, которые ему нужны.
4. Backend может расширять `payload`, не ломая старые устройства.
5. Все критичные команды должны иметь безопасное поведение по умолчанию.

---

## Безопасность движения

Система использует несколько уровней аварийной остановки.

### Frontend

```text
при отпускании кнопки отправляется stop;
при потере фокуса окна отправляется stop;
команды drive повторяются каждые 100 мс, пока кнопка нажата.
```

### Backend

```text
если WebSocket отключился, backend отправляет stop;
если команды не приходят дольше safety.command_timeout_ms, backend отправляет stop;
при завершении backend отправляет stop.
```

### ESP32

```text
если UDP-команды не приходят заданное время, ESP32 самостоятельно останавливает моторы;
при emergency_stop ESP32 немедленно останавливает моторы;
при ошибке протокола ESP32 должен переходить в безопасное состояние.
```

---

## Тестовые инструменты

### UDP listener для проверки motor protocol

```bash
go run ./tools/udp-listener
```

Backend должен отправлять UDP-команды на адрес из:

```yaml
motor:
  address: "127.0.0.1:4210"
```

### Telemetry sender для проверки telemetry protocol

```bash
go run ./tools/telemetry-sender
```

Backend слушает телеметрию на адресе:

```yaml
telemetry:
  listen_address: "0.0.0.0:4211"
```

---

## Запуск backend

```bash
go run ./cmd/server
```

Проверка:

```bash
curl http://localhost:8080/health
curl http://localhost:8080/api/state
```

Веб-интерфейс:

```text
http://localhost:8080
```

---

## Текущий статус проекта

Реализовано:

```text
HTTP server
static frontend
WebSocket управление
UDP motor client
backend safety watchdog
MJPEG camera proxy
camera monitor
UDP telemetry listener
/api/state
расширяемые протоколы обмена
```

Будущие этапы:

```text
прошивка ESP32 motor controller
прошивка ESP32-CAM
подключение реального драйвера моторов
подключение камеры
управление светом
управление сервоприводом камеры
игровые механизмы для кошки
авторизация внутри VPN
логирование событий
```