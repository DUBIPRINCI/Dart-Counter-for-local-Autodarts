// WebSocket client with auto-reconnect
class DartWebSocket {
    constructor() {
        this.ws = null;
        this.handlers = {};
        this.reconnectDelay = 1000;
        this.maxReconnectDelay = 10000;
        this.connected = false;
    }

    connect() {
        const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
        const url = `${proto}//${location.host}/ws`;

        try {
            this.ws = new WebSocket(url);
        } catch (e) {
            this.scheduleReconnect();
            return;
        }

        this.ws.onopen = () => {
            this.connected = true;
            this.reconnectDelay = 1000;
            this.emit('connected');
        };

        this.ws.onclose = () => {
            this.connected = false;
            this.emit('disconnected');
            this.scheduleReconnect();
        };

        this.ws.onerror = () => {
            this.ws.close();
        };

        this.ws.onmessage = (evt) => {
            // Handle multiple messages (batched by server)
            const lines = evt.data.split('\n');
            for (const line of lines) {
                if (!line.trim()) continue;
                try {
                    const msg = JSON.parse(line);
                    this.emit(msg.type, msg.data);
                } catch (e) {
                    console.warn('Invalid WS message:', line);
                }
            }
        };
    }

    scheduleReconnect() {
        setTimeout(() => {
            this.reconnectDelay = Math.min(this.reconnectDelay * 1.5, this.maxReconnectDelay);
            this.connect();
        }, this.reconnectDelay);
    }

    send(type, data) {
        if (!this.ws || this.ws.readyState !== WebSocket.OPEN) return;
        this.ws.send(JSON.stringify({ type, data }));
    }

    on(event, handler) {
        if (!this.handlers[event]) this.handlers[event] = [];
        this.handlers[event].push(handler);
    }

    emit(event, data) {
        const handlers = this.handlers[event];
        if (handlers) {
            for (const h of handlers) h(data);
        }
    }

    // Convenience methods
    throwDart(segment) { this.send('manualThrow', { segment }); }
    correct(dartIndex, segment) { this.send('correct', { dartIndex, segment }); }
    undo() { this.send('undo'); }
    nextPlayer() { this.send('nextPlayer'); }
}
