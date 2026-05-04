let socket = null;
let repeatTimer = null;
let activeCommand = null;

const COMMAND_REPEAT_MS = 100;

const wsDot = document.getElementById("ws-dot");
const wsStatus = document.getElementById("ws-status");
const leftValue = document.getElementById("left-value");
const rightValue = document.getElementById("right-value");
const motorStatus = document.getElementById("motor-status");
const cameraStatus = document.getElementById("camera-status");
const failsafeStatus = document.getElementById("failsafe-status");
const commandStatus = document.getElementById("command-status");

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
    
        if (msg.type !== "state") {
            return;
        }
    
        renderState(msg);
    };
}

function sendCommand(command) {
    if (!socket || socket.readyState !== WebSocket.OPEN) {
        commandStatus.textContent = "WebSocket не подключен";
        return;
    }

    socket.send(JSON.stringify(command));
}

function sendDrive(left, right) {
    sendCommand({
        type: "drive",
        left,
        right,
    });
}

function sendStop() {
    sendCommand({
        type: "stop",
    });
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

function stopDrive() {
    activeCommand = null;
    clearRepeatTimer();
    sendStop();
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
        stopDrive();
    });

    button.addEventListener("pointercancel", (event) => {
        event.preventDefault();
        stopDrive();
    });

    button.addEventListener("pointerleave", (event) => {
        if (event.buttons !== 0) {
            stopDrive();
        }
    });
}

function renderState(state) {
    leftValue.textContent = state.left;
    rightValue.textContent = state.right;

    motorStatus.textContent = state.motor_connected ? "подключен" : "нет связи";
    cameraStatus.textContent = state.camera_connected ? "подключена" : "нет связи";
    failsafeStatus.textContent = state.failsafe ? "true" : "false";

    if (state.last_command_valid) {
        commandStatus.textContent = "ok";
    } else {
        commandStatus.textContent = state.error || state.last_error || "ошибка команды";
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


bindHoldButton("forward", 70, 70);
bindHoldButton("backward", -70, -70);
bindHoldButton("left", -50, 50);
bindHoldButton("right", 50, -50);

document.getElementById("stop").addEventListener("click", () => {
    stopDrive();
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
            stopDrive();
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
            stopDrive();
            break;
    }
});

window.addEventListener("blur", () => {
    stopDrive();
});

connectWebSocket();
refreshState();

setInterval(refreshState, 1000);