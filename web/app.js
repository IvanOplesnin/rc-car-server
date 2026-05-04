let socket = null;
let repeatTimer = null;
let activeCommand = null;
let wsSeq = 0;

const COMMAND_REPEAT_MS = 100;
const PROTOCOL_VERSION = 1;

const wsDot = document.getElementById("ws-dot");
const wsStatus = document.getElementById("ws-status");
const leftValue = document.getElementById("left-value");
const rightValue = document.getElementById("right-value");
const motorStatus = document.getElementById("motor-status");
const cameraStatus = document.getElementById("camera-status");
const failsafeStatus = document.getElementById("failsafe-status");
const batteryVoltage = document.getElementById("battery-voltage");
const batteryPercent = document.getElementById("battery-percent");
const rssiValue = document.getElementById("rssi-value");
const uptimeMs = document.getElementById("uptime-ms");
const freeHeap = document.getElementById("free-heap");
const commandStatus = document.getElementById("command-status");

function nextSeq() {
    wsSeq += 1;
    return wsSeq;
}

function nowUnixMilli() {
    return Date.now();
}

function makeMessage(type, payload) {
    return {
        version: PROTOCOL_VERSION,
        type,
        seq: nextSeq(),
        timestamp: nowUnixMilli(),
        payload,
    };
}

function connectWebSocket() {
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const url = `${protocol}//${window.location.host}/ws`;

    socket = new WebSocket(url);

    socket.onopen = () => {
        wsDot.classList.remove("offline");
        wsDot.classList.add("online");
        wsStatus.textContent = "WebSocket: подключен";
        commandStatus.textContent = "подключение установлено";
    };

    socket.onclose = () => {
        wsDot.classList.remove("online");
        wsDot.classList.add("offline");
        wsStatus.textContent = "WebSocket: отключен";
        commandStatus.textContent = "соединение потеряно";

        clearRepeatTimer();

        setTimeout(connectWebSocket, 1000);
    };

    socket.onerror = () => {
        commandStatus.textContent = "ошибка WebSocket";
    };

    socket.onmessage = (event) => {
        const msg = JSON.parse(event.data);

        if (msg.version !== PROTOCOL_VERSION) {
            commandStatus.textContent = "неподдерживаемая версия протокола";
            return;
        }

        if (msg.type !== "state") {
            return;
        }

        renderStateMessage(msg);
    };
}

function renderStateMessage(msg) {
    const payload = msg.payload || {};

    renderState({
        motor_connected: payload.connection?.motor_connected || false,
        camera_connected: payload.connection?.camera_connected || false,
        left: payload.drive?.left || 0,
        right: payload.drive?.right || 0,
        battery_voltage: payload.power?.battery_voltage || 0,
        battery_percent: payload.power?.battery_percent || 0,
        rssi: payload.network?.rssi || 0,
        uptime_ms: payload.system?.uptime_ms || 0,
        free_heap: payload.system?.free_heap || 0,
        failsafe: payload.safety?.failsafe || false,
        last_command_valid: payload.safety?.last_command_valid ?? true,
        last_error: payload.safety?.last_error || "",
    });
}

function renderState(state) {
    leftValue.textContent = state.left;
    rightValue.textContent = state.right;

    motorStatus.textContent = state.motor_connected ? "подключен" : "нет связи";
    cameraStatus.textContent = state.camera_connected ? "подключена" : "нет связи";
    failsafeStatus.textContent = state.failsafe ? "true" : "false";

    batteryVoltage.textContent = `${Number(state.battery_voltage || 0).toFixed(2)} В`;
    batteryPercent.textContent = `${state.battery_percent || 0}%`;
    rssiValue.textContent = `${state.rssi || 0} dBm`;
    uptimeMs.textContent = `${state.uptime_ms || 0} мс`;
    freeHeap.textContent = `${state.free_heap || 0} байт`;

    if (state.last_command_valid) {
        commandStatus.textContent = "ok";
    } else {
        commandStatus.textContent = state.last_error || "ошибка команды";
    }
}

async function refreshState() {
    try {
        const response = await fetch("/api/state");

        if (!response.ok) {
            commandStatus.textContent = "ошибка получения состояния";
            return;
        }

        const state = await response.json();
        renderState(state);
    } catch (error) {
        commandStatus.textContent = "backend недоступен";
    }
}

function sendMessage(message) {
    if (!socket || socket.readyState !== WebSocket.OPEN) {
        commandStatus.textContent = "WebSocket не подключен";
        return;
    }

    socket.send(JSON.stringify(message));
}

function sendDrive(left, right) {
    const message = makeMessage("control", {
        drive: {
            left,
            right,
        },
    });

    sendMessage(message);
}

function sendStop(reason = "user") {
    const message = makeMessage("system", {
        system: {
            command: "stop",
            reason,
        },
    });

    sendMessage(message);
}

function sendEmergencyStop(reason = "user") {
    const message = makeMessage("system", {
        system: {
            command: "emergency_stop",
            reason,
        },
    });

    sendMessage(message);
}

function startDrive(left, right) {
    activeCommand = { left, right };

    sendDrive(left, right);

    clearRepeatTimer();

    repeatTimer = setInterval(() => {
        if (!activeCommand) {
            return;
        }

        sendDrive(activeCommand.left, activeCommand.right);
    }, COMMAND_REPEAT_MS);
}

function stopDrive(reason = "button_up") {
    activeCommand = null;
    clearRepeatTimer();
    sendStop(reason);
}

function clearRepeatTimer() {
    if (repeatTimer !== null) {
        clearInterval(repeatTimer);
        repeatTimer = null;
    }
}

function bindHoldButton(id, left, right) {
    const button = document.getElementById(id);

    button.addEventListener("pointerdown", (event) => {
        event.preventDefault();
        startDrive(left, right);
    });

    button.addEventListener("pointerup", (event) => {
        event.preventDefault();
        stopDrive("button_up");
    });

    button.addEventListener("pointercancel", (event) => {
        event.preventDefault();
        stopDrive("pointer_cancel");
    });

    button.addEventListener("pointerleave", (event) => {
        if (event.buttons !== 0) {
            stopDrive("pointer_leave");
        }
    });
}

bindHoldButton("forward", 70, 70);
bindHoldButton("backward", -70, -70);
bindHoldButton("left", -50, 50);
bindHoldButton("right", 50, -50);

document.getElementById("stop").addEventListener("click", () => {
    sendEmergencyStop("stop_button");
});

document.addEventListener("keydown", (event) => {
    if (event.repeat) {
        return;
    }

    switch (event.key.toLowerCase()) {
        case "w":
        case "ц":
            startDrive(70, 70);
            break;

        case "s":
        case "ы":
            startDrive(-70, -70);
            break;

        case "a":
        case "ф":
            startDrive(-50, 50);
            break;

        case "d":
        case "в":
            startDrive(50, -50);
            break;

        case " ":
            sendEmergencyStop("space_key");
            break;
    }
});

document.addEventListener("keyup", (event) => {
    switch (event.key.toLowerCase()) {
        case "w":
        case "ц":
        case "s":
        case "ы":
        case "a":
        case "ф":
        case "d":
        case "в":
            stopDrive("key_up");
            break;
    }
});

window.addEventListener("blur", () => {
    stopDrive("window_blur");
});

connectWebSocket();
refreshState();

setInterval(refreshState, 1000);