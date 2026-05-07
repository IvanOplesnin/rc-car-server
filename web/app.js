let socket = null;
let repeatTimer = null;
let activeCommand = null;
let wsSeq = 0;

const COMMAND_REPEAT_MS = 100;
const PROTOCOL_VERSION = 1;

const SPEED_FORWARD = 70;
const SPEED_TURN = 50;

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

function makeMessage(type, payload) {
    return {
        version: PROTOCOL_VERSION,
        type,
        seq: nextSeq(),
        timestamp: Date.now(),
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

        stopDrive("ws_close", false);

        setTimeout(connectWebSocket, 1000);
    };

    socket.onerror = () => {
        commandStatus.textContent = "ошибка WebSocket";
    };

    socket.onmessage = (event) => {
        let msg;

        try {
            msg = JSON.parse(event.data);
        } catch {
            commandStatus.textContent = "ошибка разбора сообщения";
            return;
        }

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
    } catch {
        commandStatus.textContent = "backend недоступен";
    }
}

function sendMessage(message) {
    if (!socket || socket.readyState !== WebSocket.OPEN) {
        commandStatus.textContent = "WebSocket не подключен";
        return false;
    }

    socket.send(JSON.stringify(message));
    return true;
}

function sendDrive(left, right) {
    return sendMessage(
        makeMessage("control", {
            drive: {
                left,
                right,
            },
        })
    );
}

function sendStop(reason = "button_up") {
    return sendMessage(
        makeMessage("system", {
            system: {
                command: "stop",
                reason,
            },
        })
    );
}

function sendEmergencyStop(reason = "user") {
    return sendMessage(
        makeMessage("system", {
            system: {
                command: "emergency_stop",
                reason,
            },
        })
    );
}

function clearRepeatTimer() {
    if (repeatTimer !== null) {
        clearInterval(repeatTimer);
        repeatTimer = null;
    }
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

function stopDrive(reason = "button_up", sendStopCommand = true) {
    activeCommand = null;
    clearRepeatTimer();

    document
        .querySelectorAll(".control-button.active")
        .forEach((button) => button.classList.remove("active"));

    if (sendStopCommand) {
        sendStop(reason);
    }
}

function bindHoldButton(id, left, right) {
    const button = document.getElementById(id);

    if (!button) {
        return;
    }

    let isPressed = false;

    const start = (event) => {
        event.preventDefault();

        if (isPressed) {
            return;
        }

        isPressed = true;
        button.classList.add("active");

        button.setPointerCapture?.(event.pointerId);

        startDrive(left, right);
    };

    const stop = (event, reason) => {
        event?.preventDefault();

        if (!isPressed) {
            return;
        }

        isPressed = false;
        button.classList.remove("active");

        stopDrive(reason);
    };

    button.addEventListener("pointerdown", start);
    button.addEventListener("pointerup", (event) => stop(event, "button_up"));
    button.addEventListener("pointercancel", (event) => stop(event, "pointer_cancel"));
    button.addEventListener("pointerleave", (event) => stop(event, "pointer_leave"));

    button.addEventListener("contextmenu", (event) => {
        event.preventDefault();
    });
}

bindHoldButton("forward", SPEED_FORWARD, SPEED_FORWARD);
bindHoldButton("backward", -SPEED_FORWARD, -SPEED_FORWARD);
bindHoldButton("left", -SPEED_TURN, SPEED_TURN);
bindHoldButton("right", SPEED_TURN, -SPEED_TURN);

const stopButton = document.getElementById("stop");

if (stopButton) {
    stopButton.addEventListener("pointerdown", (event) => {
        event.preventDefault();

        stopDrive("stop_button");
        sendEmergencyStop("stop_button");
    });

    stopButton.addEventListener("contextmenu", (event) => {
        event.preventDefault();
    });
}

document.addEventListener("keydown", (event) => {
    if (event.repeat) {
        return;
    }

    switch (event.key.toLowerCase()) {
        case "w":
        case "ц":
            startDrive(SPEED_FORWARD, SPEED_FORWARD);
            break;

        case "s":
        case "ы":
            startDrive(-SPEED_FORWARD, -SPEED_FORWARD);
            break;

        case "a":
        case "ф":
            startDrive(-SPEED_TURN, SPEED_TURN);
            break;

        case "d":
        case "в":
            startDrive(SPEED_TURN, -SPEED_TURN);
            break;

        case " ":
            event.preventDefault();
            stopDrive("space_key");
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

document.addEventListener("visibilitychange", () => {
    if (document.hidden) {
        stopDrive("window_blur");
    }
});

connectWebSocket();
refreshState();
setInterval(refreshState, 1000);