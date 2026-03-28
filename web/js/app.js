// DartCounter App
const app = {
    ws: new DartWebSocket(),
    sound: new SoundManager(),
    state: null,
    players: [],
    setupPlayers: [],
    selectedModifier: 's',
    correctingDart: -1,
    currentScreen: 'home',

    async init() {
        // Init WebSocket
        this.ws.on('connected', () => this.updateAutodartsStatus(true));
        this.ws.on('disconnected', () => this.updateAutodartsStatus(false));
        this.ws.on('state', (data) => this.onGameState(data));
        this.ws.on('sound', (data) => this.sound.playEvents(data.events));
        this.ws.on('event', (data) => this.onGameEvent(data));
        this.ws.connect();

        // Init sound on first interaction
        document.addEventListener('click', () => this.sound.init(), { once: true });
        document.addEventListener('touchstart', () => this.sound.init(), { once: true });

        // Load players
        await this.loadPlayers();

        // Check for active game
        try {
            const resp = await fetch('/api/games/current');
            if (resp.ok) {
                this.state = await resp.json();
                this.showGame();
                this.renderGame();
            }
        } catch (e) {}

        // Setup UI bindings
        this.bindSetup();
        this.bindKeyboard();
        this.bindPhysicalKeyboard();

        // Check autodarts status
        this.checkAutodartsStatus();

        // Settings
        this.bindSettings();
    },

    // ---- Screen Management ----
    showScreen(name) {
        document.querySelectorAll('.screen').forEach(s => s.classList.remove('active'));
        const screen = document.getElementById(`screen-${name}`);
        if (screen) screen.classList.add('active');
        this.currentScreen = name;
    },

    showHome() { this.showScreen('home'); },

    showSetup() {
        this.showScreen('setup');
        this.renderSetupPlayers();
    },

    showGame() { this.showScreen('game'); },

    showSettings() {
        this.showScreen('settings');
        this.checkAutodartsStatus();
    },

    // ---- Players ----
    async loadPlayers() {
        try {
            const resp = await fetch('/api/players');
            this.players = await resp.json();
        } catch (e) {
            this.players = [];
        }
    },

    async addPlayer() {
        const input = document.getElementById('new-player-name');
        const name = input.value.trim();
        if (!name) return;

        try {
            const resp = await fetch('/api/players', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ name, avatar: '' })
            });
            if (resp.ok) {
                const player = await resp.json();
                this.players.push(player);
                this.setupPlayers.push(player);
                input.value = '';
                this.renderSetupPlayers();
            }
        } catch (e) {}
    },

    removeSetupPlayer(index) {
        this.setupPlayers.splice(index, 1);
        this.renderSetupPlayers();
    },

    renderSetupPlayers() {
        const list = document.getElementById('player-list');
        list.innerHTML = this.setupPlayers.map((p, i) => `
            <div class="player-item anim-fade-in">
                <span class="player-name">${this.esc(p.name)}</span>
                <button class="player-remove" onclick="app.removeSetupPlayer(${i})">&times;</button>
            </div>
        `).join('');
    },

    // ---- Setup Bindings ----
    bindSetup() {
        // Game type selection
        document.getElementById('game-type-grid').addEventListener('click', (e) => {
            const btn = e.target.closest('.game-type-btn');
            if (!btn) return;
            document.querySelectorAll('.game-type-btn').forEach(b => b.classList.remove('selected'));
            btn.classList.add('selected');

            const type = btn.dataset.type;
            document.getElementById('x01-options').classList.toggle('hidden', type !== 'x01');
            document.getElementById('cricket-options').classList.toggle('hidden', type !== 'cricket');
        });

        // Option groups (generic toggle)
        document.querySelectorAll('.option-group').forEach(group => {
            group.addEventListener('click', (e) => {
                const btn = e.target.closest('.opt-btn');
                if (!btn) return;
                group.querySelectorAll('.opt-btn').forEach(b => b.classList.remove('selected'));
                btn.classList.add('selected');
            });
        });

        // Enter key on player name input
        document.getElementById('new-player-name').addEventListener('keydown', (e) => {
            if (e.key === 'Enter') this.addPlayer();
        });

        // Header buttons
        document.getElementById('btn-new-game').addEventListener('click', () => this.showSetup());
        document.getElementById('btn-settings').addEventListener('click', () => this.showSettings());
    },

    getSelectedValue(groupId) {
        const group = document.getElementById(groupId);
        const selected = group?.querySelector('.opt-btn.selected');
        return selected?.dataset.value || '';
    },

    // ---- Start Game ----
    async startGame() {
        if (this.setupPlayers.length < 1) {
            alert('Ajoutez au moins un joueur');
            return;
        }

        const typeBtn = document.querySelector('.game-type-btn.selected');
        const gameType = typeBtn.dataset.type;
        const variant = typeBtn.dataset.variant;

        const opts = {
            gameType,
            variant,
            playerIds: this.setupPlayers.map(p => p.id),
            playerNames: this.setupPlayers.map(p => p.name),
        };

        if (gameType === 'x01') {
            opts.startScore = parseInt(variant);
            opts.inMode = this.getSelectedValue('in-mode');
            opts.outMode = this.getSelectedValue('out-mode');
            opts.sets = parseInt(this.getSelectedValue('sets-count')) || 1;
            opts.legs = parseInt(this.getSelectedValue('legs-count')) || 3;
        } else if (gameType === 'cricket') {
            opts.cricketMode = this.getSelectedValue('cricket-mode');
        }

        try {
            const resp = await fetch('/api/games', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(opts)
            });
            if (resp.ok) {
                this.state = await resp.json();
                this.showGame();
                this.renderGame();
            }
        } catch (e) {
            console.error('Failed to create game:', e);
        }
    },

    // ---- Game Rendering ----
    onGameState(data) {
        this.state = data;
        this.renderGame();
    },

    onGameEvent(data) {
        const display = document.getElementById('event-display');
        display.className = 'event-display';

        switch (data.event) {
            case 'bust':
                display.textContent = 'BUST!';
                display.classList.add('bust', 'anim-event-pop');
                this.flashPlayerPanel('bust');
                break;
            case 'gameshot':
                display.textContent = 'GAME SHOT!';
                display.classList.add('gameshot', 'anim-event-pop');
                break;
            case 'matchshot':
                display.textContent = 'MATCH SHOT!';
                display.classList.add('gameshot', 'anim-event-pop');
                break;
            case '180':
                display.textContent = '180!';
                display.classList.add('one-eighty', 'anim-event-pop');
                break;
            case 'gameAbandoned':
                this.state = null;
                this.showHome();
                return;
            default:
                display.textContent = '';
                return;
        }

        // Auto-clear event after 2s
        setTimeout(() => {
            display.classList.add('anim-event-fade');
            setTimeout(() => {
                display.textContent = '';
                display.className = 'event-display';
            }, 500);
        }, 2000);
    },

    flashPlayerPanel(type) {
        if (!this.state) return;
        const panels = document.querySelectorAll('.player-panel');
        const activePanel = panels[this.state.currentPlayer];
        if (activePanel) {
            activePanel.classList.add(type);
            setTimeout(() => activePanel.classList.remove(type), 500);
        }
    },

    renderGame() {
        if (!this.state) return;

        // Update game info header
        const info = document.getElementById('game-info');
        info.textContent = `${this.state.variant || this.state.gameType} - Set ${this.state.currentSet} Leg ${this.state.currentLeg}`;

        // Render player panels
        this.renderPlayers();

        // Render visit display
        this.renderVisit();

        // Render checkout hint
        const hint = document.getElementById('checkout-hint');
        hint.textContent = this.state.checkoutHint || '';

        // Handle finished game
        if (this.state.status === 'finished') {
            const winner = this.state.players.find(p => p.playerId === this.state.winnerId);
            const display = document.getElementById('event-display');
            display.textContent = winner ? `${winner.playerName} WINS!` : 'GAME OVER';
            display.className = 'event-display gameshot anim-event-pop';
        }
    },

    renderPlayers() {
        const container = document.getElementById('players-container');
        container.innerHTML = this.state.players.map((p, i) => {
            const active = p.isActive ? 'active' : '';
            const avg = p.average ? p.average.toFixed(1) : '0.0';
            const legsNeeded = Math.ceil((this.state.options?.legs || 1) / 2) + (((this.state.options?.legs || 1) % 2 === 0) ? 1 : 0);

            let legDots = '';
            if (this.state.options?.legs > 1) {
                for (let l = 0; l < Math.ceil((this.state.options.legs || 1) / 2 + 0.5); l++) {
                    legDots += `<span class="leg-dot ${l < p.legsWon ? 'won' : ''}"></span>`;
                }
            }

            return `
                <div class="player-panel ${active} anim-fade-in">
                    <div class="player-name">${this.esc(p.playerName)}</div>
                    <div class="player-score score-flash">${p.score}</div>
                    <div class="player-stats">
                        <div class="player-stat">
                            <span class="player-stat-value">${avg}</span>
                            <span>AVG</span>
                        </div>
                        <div class="player-stat">
                            <span class="player-stat-value">${p.dartsThrown}</span>
                            <span>Darts</span>
                        </div>
                        ${this.state.options?.sets > 1 ? `
                        <div class="player-stat">
                            <span class="player-stat-value">${p.setsWon}</span>
                            <span>Sets</span>
                        </div>` : ''}
                    </div>
                    ${legDots ? `<div class="player-legs">${legDots}</div>` : ''}
                </div>
            `;
        }).join('');
    },

    renderVisit() {
        const activePlayer = this.state.players[this.state.currentPlayer];
        if (!activePlayer) return;

        const darts = activePlayer.currentVisit?.darts || [];
        const dartEls = document.querySelectorAll('.visit-dart');

        dartEls.forEach((el, i) => {
            el.className = 'visit-dart';
            if (i < darts.length) {
                const d = darts[i];
                el.textContent = d.segment.toUpperCase();
                el.classList.add('filled');
                if (d.multiplier === 3) el.classList.add('triple');
                else if (d.multiplier === 2) el.classList.add('double');
                if (this.correctingDart === i) el.classList.add('correcting');
            } else {
                el.textContent = '-';
                el.classList.add('empty');
            }
        });

        const total = document.getElementById('visit-total');
        total.textContent = activePlayer.currentVisit?.totalScore || 0;
    },

    // ---- Keyboard ----
    bindKeyboard() {
        const keyboard = document.getElementById('keyboard');
        if (!keyboard) return;

        // Number keys
        keyboard.querySelectorAll('.key-btn.num').forEach(btn => {
            btn.addEventListener('click', () => {
                const num = parseInt(btn.dataset.num);
                this.sendThrow(num);
            });
        });

        // Special keys
        keyboard.querySelectorAll('.key-btn.special').forEach(btn => {
            btn.addEventListener('click', () => {
                const action = btn.dataset.action;
                if (action === 'miss') this.sendSegment('MISS');
                else if (action === 'bull') this.sendSegment('BULL');
                else if (action === 'dbull') this.sendSegment('DBULL');
            });
        });

        // Modifier keys
        keyboard.querySelectorAll('.key-btn.modifier').forEach(btn => {
            btn.addEventListener('click', () => {
                keyboard.querySelectorAll('.key-btn.modifier').forEach(b => b.classList.remove('selected'));
                btn.classList.add('selected');
                this.selectedModifier = btn.dataset.mod;
            });
        });

        // Action keys
        keyboard.querySelectorAll('.key-btn.action').forEach(btn => {
            btn.addEventListener('click', () => {
                const action = btn.dataset.action;
                if (action === 'undo') this.doUndo();
                else if (action === 'next') this.doNextPlayer();
            });
        });

        // Visit dart correction (click on visit dart to correct)
        document.getElementById('visit-darts').addEventListener('click', (e) => {
            const dart = e.target.closest('.visit-dart');
            if (!dart || dart.classList.contains('empty')) return;
            const index = parseInt(dart.dataset.index);
            if (this.correctingDart === index) {
                this.correctingDart = -1;
            } else {
                this.correctingDart = index;
            }
            this.renderVisit();
        });
    },

    sendThrow(num) {
        const segment = `${this.selectedModifier}${num}`;
        this.sendSegment(segment);
    },

    async sendSegment(segment) {
        let url, body;
        if (this.correctingDart >= 0) {
            url = '/api/games/current/correct';
            body = { dartIndex: this.correctingDart, segment };
            this.correctingDart = -1;
        } else {
            url = '/api/games/current/throw';
            body = { segment };
        }

        try {
            const resp = await fetch(url, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(body)
            });
            // Fallback: use HTTP response if WebSocket is not connected
            if (resp.ok && !this.ws.connected) {
                this.state = await resp.json();
                this.renderGame();
            }
        } catch (e) {
            console.warn('Throw failed:', e);
        }

        // Reset modifier to single after throw
        this.selectedModifier = 's';
        document.querySelectorAll('.key-btn.modifier').forEach(b => b.classList.remove('selected'));
        document.querySelector('.key-btn.modifier[data-mod="s"]')?.classList.add('selected');
    },

    async doUndo() {
        try {
            const resp = await fetch('/api/games/current/undo', { method: 'POST' });
            if (resp.ok && !this.ws.connected) {
                this.state = await resp.json();
                this.renderGame();
            }
        } catch (e) {}
    },

    async doNextPlayer() {
        try {
            const resp = await fetch('/api/games/current/next', { method: 'POST' });
            if (resp.ok && !this.ws.connected) {
                this.state = await resp.json();
                this.renderGame();
            }
        } catch (e) {}
    },

    // ---- Physical Keyboard ----
    bindPhysicalKeyboard() {
        let numBuffer = '';
        let numTimeout = null;

        document.addEventListener('keydown', (e) => {
            if (this.currentScreen !== 'game') return;
            if (e.target.tagName === 'INPUT') return;

            const key = e.key.toLowerCase();

            // Modifiers
            if (key === 's') { this.selectModifier('s'); return; }
            if (key === 'd') { this.selectModifier('d'); return; }
            if (key === 't') { this.selectModifier('t'); return; }

            // Special
            if (key === 'b') { this.sendSegment('BULL'); return; }
            if (key === 'm') { this.sendSegment('MISS'); return; }
            if (key === 'backspace') { e.preventDefault(); this.doUndo(); return; }
            if (key === 'enter') { this.doNextPlayer(); return; }

            // Numbers
            if (key >= '0' && key <= '9') {
                clearTimeout(numTimeout);
                numBuffer += key;

                if (numBuffer.length >= 2 || parseInt(numBuffer) > 2) {
                    const num = parseInt(numBuffer);
                    if (num >= 1 && num <= 20) {
                        this.sendThrow(num);
                    }
                    numBuffer = '';
                } else {
                    numTimeout = setTimeout(() => {
                        const num = parseInt(numBuffer);
                        if (num >= 1 && num <= 20) {
                            this.sendThrow(num);
                        }
                        numBuffer = '';
                    }, 800);
                }
            }
        });
    },

    selectModifier(mod) {
        this.selectedModifier = mod;
        document.querySelectorAll('.key-btn.modifier').forEach(b => b.classList.remove('selected'));
        document.querySelector(`.key-btn.modifier[data-mod="${mod}"]`)?.classList.add('selected');
    },

    // ---- Autodarts Status ----
    updateAutodartsStatus(wsConnected) {
        const dot = document.getElementById('autodarts-status');
        dot.className = `status-dot ${wsConnected ? 'connected' : 'disconnected'}`;
    },

    async checkAutodartsStatus() {
        try {
            const resp = await fetch('/api/autodarts/status');
            const data = await resp.json();
            const statusEl = document.getElementById('settings-bm-status');
            if (statusEl) {
                statusEl.textContent = data.connected ? 'Connecté' : 'Déconnecté';
                statusEl.style.color = data.connected ? 'var(--accent-green)' : 'var(--accent-red)';
            }
        } catch (e) {}
    },

    // ---- Settings ----
    bindSettings() {
        const volumeSlider = document.getElementById('sound-volume');
        if (volumeSlider) {
            volumeSlider.addEventListener('input', (e) => {
                this.sound.setVolume(parseInt(e.target.value) / 100);
            });
        }
    },

    // ---- Utilities ----
    esc(str) {
        const div = document.createElement('div');
        div.textContent = str;
        return div.innerHTML;
    }
};

// Initialize
document.addEventListener('DOMContentLoaded', () => app.init());
