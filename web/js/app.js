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
    soundPacks: [],
    currentSoundPack: 'default',

    async init() {
        this.ws.on('connected', () => this.updateAutodartsStatus(true));
        this.ws.on('disconnected', () => this.updateAutodartsStatus(false));
        this.ws.on('state', (data) => this.onGameState(data));
        this.ws.on('sound', (data) => this.sound.playEvents(data.events));
        this.ws.on('event', (data) => this.onGameEvent(data));
        this.ws.connect();

        document.addEventListener('click', () => this.sound.init(), { once: true });
        document.addEventListener('touchstart', () => this.sound.init(), { once: true });

        await this.loadPlayers();

        try {
            const resp = await fetch('/api/games/current');
            if (resp.ok) {
                this.state = await resp.json();
                this.showGame();
                this.renderGame();
            }
        } catch (e) {}

        this.bindSetup();
        this.bindKeyboard();
        this.bindPhysicalKeyboard();
        this.bindSettings();
        this.checkAutodartsStatus();
    },

    // ── Screen Management ──────────────────────────────────
    showScreen(name) {
        document.querySelectorAll('.screen').forEach(s => s.classList.remove('active'));
        const screen = document.getElementById(`screen-${name}`);
        if (screen) screen.classList.add('active');
        this.currentScreen = name;
    },

    showHome()     { this.showScreen('home'); },
    showSetup()    { this.showScreen('setup'); this.renderSetupPlayers(); },
    showGame()     { this.showScreen('game'); },
    showSettings() {
        this.showScreen('settings');
        this.checkAutodartsStatus();
        this.loadSoundSettings();
    },

    // ── Players ────────────────────────────────────────────
    async loadPlayers() {
        try {
            const resp = await fetch('/api/players');
            this.players = await resp.json();
        } catch (e) { this.players = []; }
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
            } else if (resp.status === 409) {
                const existing = this.players.find(p => p.name.toLowerCase() === name.toLowerCase());
                if (existing && !this.setupPlayers.find(p => p.id === existing.id)) {
                    this.setupPlayers.push(existing);
                    input.value = '';
                    this.renderSetupPlayers();
                } else {
                    this.showInputError(input, 'Ce nom existe déjà');
                }
            } else {
                const body = await resp.json().catch(() => ({}));
                this.showInputError(input, body.error || `Erreur ${resp.status}`);
            }
        } catch (e) { console.error('addPlayer:', e); }
    },

    showInputError(input, msg) {
        input.classList.add('input-error');
        input.placeholder = msg;
        input.value = '';
        setTimeout(() => {
            input.classList.remove('input-error');
            input.placeholder = 'Nom du joueur';
        }, 2500);
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

    // ── Setup Bindings ─────────────────────────────────────
    bindSetup() {
        document.getElementById('game-type-grid').addEventListener('click', (e) => {
            const btn = e.target.closest('.game-type-btn');
            if (!btn) return;
            document.querySelectorAll('.game-type-btn').forEach(b => b.classList.remove('selected'));
            btn.classList.add('selected');
            const type = btn.dataset.type;
            document.getElementById('x01-options').classList.toggle('hidden', type !== 'x01');
            document.getElementById('cricket-options').classList.toggle('hidden', type !== 'cricket');
        });

        document.querySelectorAll('.option-group').forEach(group => {
            group.addEventListener('click', (e) => {
                const btn = e.target.closest('.opt-btn');
                if (!btn) return;
                group.querySelectorAll('.opt-btn').forEach(b => b.classList.remove('selected'));
                btn.classList.add('selected');
            });
        });

        document.getElementById('new-player-name').addEventListener('keydown', (e) => {
            if (e.key === 'Enter') this.addPlayer();
        });

        document.getElementById('btn-new-game').addEventListener('click', () => this.showSetup());
        document.getElementById('btn-settings').addEventListener('click', () => this.showSettings());
    },

    getSelectedValue(groupId) {
        const group = document.getElementById(groupId);
        const selected = group?.querySelector('.opt-btn.selected');
        return selected?.dataset.value || '';
    },

    // ── Start Game ─────────────────────────────────────────
    async startGame() {
        if (this.setupPlayers.length < 1) { alert('Ajoutez au moins un joueur'); return; }

        const typeBtn = document.querySelector('.game-type-btn.selected');
        const gameType = typeBtn.dataset.type;
        const variant  = typeBtn.dataset.variant;

        const opts = {
            gameType, variant,
            playerIds:   this.setupPlayers.map(p => p.id),
            playerNames: this.setupPlayers.map(p => p.name),
        };

        if (gameType === 'x01') {
            opts.startScore = parseInt(variant);
            opts.inMode     = this.getSelectedValue('in-mode');
            opts.outMode    = this.getSelectedValue('out-mode');
            opts.sets       = parseInt(this.getSelectedValue('sets-count')) || 1;
            opts.legs       = parseInt(this.getSelectedValue('legs-count')) || 3;
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
        } catch (e) { console.error('startGame:', e); }
    },

    // ── Game Rendering ─────────────────────────────────────
    onGameState(data) {
        this.state = data;
        if (this.currentScreen === 'game') this.renderGame();
    },

    onGameEvent(data) {
        const display = document.getElementById('event-display');
        if (!display) return;
        display.className = 'event-display';

        switch (data.event) {
            case 'bust':
                display.textContent = 'BUST!';
                display.classList.add('bust', 'anim-event-pop');
                this.flashActiveZone('bust');
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

        setTimeout(() => {
            display.classList.add('anim-event-fade');
            setTimeout(() => { display.textContent = ''; display.className = 'event-display'; }, 500);
        }, 2000);
    },

    flashActiveZone(type) {
        const zone = document.getElementById('active-zone');
        if (!zone) return;
        zone.classList.add(type);
        setTimeout(() => zone.classList.remove(type), 500);
    },

    renderGame() {
        if (!this.state) return;

        const info = document.getElementById('game-info');
        if (info) info.textContent = `${this.state.variant || this.state.gameType} — Set ${this.state.currentSet} Leg ${this.state.currentLeg}`;

        const p = this.state.players[this.state.currentPlayer];
        this.renderActivePlayer(p);
        this.renderWaitingPlayers();
        this.renderActiveDarts(p);
        this.renderTakeoutOverlay();

        if (this.state.status === 'finished') {
            const winner = this.state.players.find(pl => pl.playerId === this.state.winnerId);
            const display = document.getElementById('event-display');
            if (display) {
                display.textContent = winner ? `${winner.playerName} WINS!` : 'GAME OVER';
                display.className = 'event-display gameshot anim-event-pop';
            }
        }
    },

    renderActivePlayer(p) {
        if (!p) return;
        const nameEl  = document.getElementById('ap-name');
        const scoreEl = document.getElementById('ap-score');
        const statsEl = document.getElementById('ap-stats');
        const hintEl  = document.getElementById('checkout-hint');

        if (nameEl)  nameEl.textContent  = p.playerName;
        if (scoreEl) scoreEl.textContent = p.score;
        if (hintEl)  hintEl.textContent  = this.state.checkoutHint || '';

        if (statsEl) {
            const avg = p.average ? p.average.toFixed(1) : '0.0';
            const legsNeeded = Math.floor(((this.state.options?.legs || 1) + 1) / 2);
            let legDots = '';
            if ((this.state.options?.legs || 1) > 1) {
                for (let l = 0; l < legsNeeded; l++) {
                    legDots += `<span class="leg-dot ${l < (p.legsWon || 0) ? 'won' : ''}"></span>`;
                }
            }
            statsEl.innerHTML = `
                <span><span>${avg}</span> AVG</span>
                <span><span>${p.dartsThrown}</span> Darts</span>
                ${(this.state.options?.sets || 1) > 1 ? `<span><span>${p.setsWon || 0}</span> Sets</span>` : ''}
                ${legDots ? `<div class="leg-dots">${legDots}</div>` : ''}
            `;
        }
    },

    renderWaitingPlayers() {
        const container = document.getElementById('waiting-players');
        if (!container) return;
        const waiting = this.state.players.filter((_, i) => i !== this.state.currentPlayer);
        if (waiting.length === 0) { container.innerHTML = ''; return; }
        container.innerHTML = waiting.map(p => {
            const avg = p.average ? p.average.toFixed(1) : '—';
            return `<div class="waiting-card">
                <div class="waiting-card-name">${this.esc(p.playerName)}</div>
                <div class="waiting-card-score">${p.score}</div>
                <div class="waiting-card-avg">AVG ${avg}</div>
            </div>`;
        }).join('');
    },

    renderActiveDarts(p) {
        if (!p) return;
        const container = document.getElementById('active-darts');
        if (!container) return;
        const darts   = p.currentVisit?.darts || [];
        const totalEl = document.getElementById('ap-total');
        if (totalEl) totalEl.textContent = p.currentVisit?.totalScore || 0;

        container.querySelectorAll('.adart:not(.total)').forEach((el, i) => {
            el.className = 'adart';
            const valEl = el.querySelector('.adart-val');
            if (i < darts.length) {
                const d = darts[i];
                valEl.textContent = d.segment ? d.segment.toUpperCase() : String(d.score);
                el.classList.add('filled');
                if (d.multiplier === 3)      el.classList.add('triple');
                else if (d.multiplier === 2) el.classList.add('double');
                else if (d.score === 0)      el.classList.add('miss');
                if (this.correctingDart === i) el.classList.add('correcting');
            } else {
                valEl.textContent = '—';
                el.classList.add('empty');
            }
        });
    },

    renderTakeoutOverlay() {
        let overlay = document.getElementById('takeout-overlay');
        const keyboard = document.getElementById('keyboard');

        if (this.state?.waitingTakeout) {
            if (!overlay) {
                overlay = document.createElement('div');
                overlay.id = 'takeout-overlay';
                overlay.className = 'takeout-overlay';
                overlay.innerHTML = `
                    <div class="takeout-message">
                        <div class="takeout-icon">🎯</div>
                        <div class="takeout-text">RETIREZ VOS FLÉCHETTES</div>
                        <button class="btn btn-primary takeout-btn" onclick="app.doFinishTakeout()">SUIVANT ▶</button>
                    </div>
                `;
                if (keyboard) keyboard.parentNode.insertBefore(overlay, keyboard);
            }
            overlay.style.display = 'flex';
            if (keyboard) keyboard.classList.add('keyboard-disabled');
        } else {
            if (overlay) overlay.style.display = 'none';
            if (keyboard) keyboard.classList.remove('keyboard-disabled');
        }
    },

    // ── Keyboard ───────────────────────────────────────────
    bindKeyboard() {
        const keyboard = document.getElementById('keyboard');
        if (!keyboard) return;

        keyboard.querySelectorAll('.key-btn.num').forEach(btn => {
            btn.addEventListener('click', () => this.sendThrow(parseInt(btn.dataset.num)));
        });

        keyboard.querySelectorAll('.key-btn.special').forEach(btn => {
            btn.addEventListener('click', () => {
                const action = btn.dataset.action;
                if (action === 'miss')  this.sendSegment('MISS');
                else if (action === 'bull')  this.sendSegment('BULL');
                else if (action === 'dbull') this.sendSegment('DBULL');
            });
        });

        keyboard.querySelectorAll('.key-btn.modifier').forEach(btn => {
            btn.addEventListener('click', () => {
                keyboard.querySelectorAll('.key-btn.modifier').forEach(b => b.classList.remove('selected'));
                btn.classList.add('selected');
                this.selectedModifier = btn.dataset.mod;
            });
        });

        keyboard.querySelectorAll('.key-btn.action').forEach(btn => {
            btn.addEventListener('click', () => {
                const action = btn.dataset.action;
                if (action === 'undo') this.doUndo();
                else if (action === 'next') this.doNextPlayer();
            });
        });

        // Dart slot click → correction mode
        const activeDarts = document.getElementById('active-darts');
        if (activeDarts) {
            activeDarts.addEventListener('click', (e) => {
                const dart = e.target.closest('.adart:not(.total)');
                if (!dart || dart.classList.contains('empty')) return;
                const index = parseInt(dart.dataset.index);
                this.correctingDart = (this.correctingDart === index) ? -1 : index;
                this.renderActiveDarts(this.state?.players[this.state?.currentPlayer]);
            });
        }
    },

    sendThrow(num) {
        this.sendSegment(`${this.selectedModifier}${num}`);
    },

    async sendSegment(segment) {
        if (this.state?.waitingTakeout) return; // ignore during takeout
        let url, body;
        if (this.correctingDart >= 0) {
            url  = '/api/games/current/correct';
            body = { dartIndex: this.correctingDart, segment };
            this.correctingDart = -1;
        } else {
            url  = '/api/games/current/throw';
            body = { segment };
        }

        try {
            const resp = await fetch(url, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(body)
            });
            if (resp.ok && !this.ws.connected) {
                this.state = await resp.json();
                this.renderGame();
            }
        } catch (e) { console.warn('Throw failed:', e); }

        // Reset modifier to single
        this.selectedModifier = 's';
        document.querySelectorAll('.key-btn.modifier').forEach(b => b.classList.remove('selected'));
        document.querySelector('.key-btn.modifier[data-mod="s"]')?.classList.add('selected');
    },

    async doUndo() {
        try {
            const resp = await fetch('/api/games/current/undo', { method: 'POST' });
            if (resp.ok && !this.ws.connected) { this.state = await resp.json(); this.renderGame(); }
        } catch (e) {}
    },

    async doNextPlayer() {
        if (this.state?.waitingTakeout) return this.doFinishTakeout();
        try {
            const resp = await fetch('/api/games/current/next', { method: 'POST' });
            if (resp.ok && !this.ws.connected) { this.state = await resp.json(); this.renderGame(); }
        } catch (e) {}
    },

    async doFinishTakeout() {
        try {
            const resp = await fetch('/api/games/current/finish-takeout', { method: 'POST' });
            if (resp.ok && !this.ws.connected) { this.state = await resp.json(); this.renderGame(); }
        } catch (e) {}
    },

    // ── Physical Keyboard ──────────────────────────────────
    bindPhysicalKeyboard() {
        let numBuffer = '';
        let numTimeout = null;

        document.addEventListener('keydown', (e) => {
            if (this.currentScreen !== 'game') return;
            if (e.target.tagName === 'INPUT') return;
            const key = e.key.toLowerCase();

            if (key === 's') { this.selectModifier('s'); return; }
            if (key === 'd') { this.selectModifier('d'); return; }
            if (key === 't') { this.selectModifier('t'); return; }
            if (key === 'b') { this.sendSegment('BULL'); return; }
            if (key === 'm') { this.sendSegment('MISS'); return; }
            if (key === 'backspace') { e.preventDefault(); this.doUndo(); return; }
            if (key === 'enter') { this.doNextPlayer(); return; }

            if (key >= '0' && key <= '9') {
                clearTimeout(numTimeout);
                numBuffer += key;
                if (numBuffer.length >= 2 || parseInt(numBuffer) > 2) {
                    const num = parseInt(numBuffer);
                    if (num >= 1 && num <= 20) this.sendThrow(num);
                    numBuffer = '';
                } else {
                    numTimeout = setTimeout(() => {
                        const num = parseInt(numBuffer);
                        if (num >= 1 && num <= 20) this.sendThrow(num);
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

    // ── Autodarts Status ───────────────────────────────────
    updateAutodartsStatus(wsConnected) {
        const dot = document.getElementById('autodarts-status');
        if (dot) dot.className = `status-dot ${wsConnected ? 'connected' : 'disconnected'}`;
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

    // ── Settings ───────────────────────────────────────────
    bindSettings() {
        const volumeSlider = document.getElementById('sound-volume');
        if (volumeSlider) {
            volumeSlider.addEventListener('input', (e) => {
                this.sound.setVolume(parseInt(e.target.value) / 100);
            });
        }
    },

    // ── Sound Management ───────────────────────────────────
    async loadSoundSettings() {
        try {
            const resp = await fetch('/api/sounds/packs');
            const packs = await resp.json();
            this.soundPacks = packs || [];
            const sel = document.getElementById('sound-pack-select');
            if (!sel) return;
            sel.innerHTML = this.soundPacks.length === 0
                ? '<option value="">Aucun pack</option>'
                : this.soundPacks.map(p => `<option value="${this.esc(p.dir)}">${this.esc(p.name || p.dir)}</option>`).join('');
            if (this.soundPacks.length > 0) {
                this.currentSoundPack = this.soundPacks[0].dir;
                sel.value = this.currentSoundPack;
                this.renderSoundEvents(this.soundPacks[0]);
            }
        } catch (e) { console.warn('loadSoundSettings:', e); }
    },

    onPackChange(packDir) {
        this.currentSoundPack = packDir;
        const pack = this.soundPacks.find(p => p.dir === packDir);
        if (pack) this.renderSoundEvents(pack);
        this.sound.loadPack(packDir);
    },

    renderSoundEvents(pack) {
        const container = document.getElementById('sound-events-list');
        if (!container) return;
        const sounds = pack.sounds || {};

        const groups = [
            { label: 'Fléchettes', events: [
                ['throw',   'Fléchette (générique)'],
                ['single',  'Simple'],
                ['double',  'Double'],
                ['triple',  'Triple'],
                ['bull',    'Bull (25)'],
                ['dbull',   'Bullseye (50)'],
                ['miss',    'Miss / Raté'],
            ]},
            { label: 'Événements de partie', events: [
                ['bust',       'Bust !'],
                ['gameon',     'Game On'],
                ['gameshot',   'Game Shot'],
                ['matchshot',  'Match Shot'],
                ['180',        '180 !'],
                ['hatTrick',   'Hat Trick'],
                ['highTon',    'High Ton (140+)'],
                ['lowTon',     'Low Ton (100–139)'],
            ]},
            { label: 'Caller (annonceur de score)', events: [
                ['caller',     'Caller générique (fallback)'],
                ['caller_180', 'Caller 180'],
                ['caller_140', 'Caller 140'],
                ['caller_100', 'Caller 100'],
                ['caller_60',  'Caller 60'],
                ['caller_45',  'Caller 45'],
                ['caller_26',  'Caller 26'],
                ['caller_3',   'Caller 3'],
                ['caller_1',   'Caller 1'],
            ]},
        ];

        let html = '';
        for (const group of groups) {
            html += `<div class="sound-section-header">${group.label}</div>`;
            for (const [event, label] of group.events) {
                const file    = sounds[event] || '';
                const hasFile = !!file;
                const safeEvt = event.replace(/['"]/g, '');
                html += `<div class="sound-event-row">
                    <div class="sound-event-label">${label}</div>
                    <div class="sound-event-file ${hasFile ? 'has-file' : ''}" id="sef-${safeEvt}">${hasFile ? file : 'Aucun fichier'}</div>
                    <div class="sound-event-actions">
                        ${hasFile ? `<button class="btn-icon-sm play" title="Écouter" onclick="app.previewSound('${safeEvt}')">▶</button>` : ''}
                        <label class="btn-icon-sm" title="Uploader" style="cursor:pointer">
                            📁<input type="file" accept=".mp3,.wav,.ogg,.m4a,.webm" style="display:none"
                                onchange="app.uploadSound('${pack.dir}','${safeEvt}',this)">
                        </label>
                        ${hasFile ? `<button class="btn-icon-sm delete" title="Supprimer" onclick="app.deleteSound('${pack.dir}','${safeEvt}')">×</button>` : ''}
                    </div>
                </div>`;
            }
        }
        html += `<p style="color:var(--text-muted);font-size:0.75rem;padding:8px 4px 0">
            💡 Vous pouvez uploader <em>caller_N</em> pour n'importe quel score N (1–180) en ajoutant un événement personnalisé.
        </p>`;
        container.innerHTML = html;
    },

    previewSound(event) {
        this.sound.init().then(() => this.sound.play(event));
    },

    async uploadSound(pack, event, input) {
        const file = input.files[0];
        if (!file) return;
        const formData = new FormData();
        formData.append('pack', pack);
        formData.append('event', event);
        formData.append('file', file);

        try {
            const resp = await fetch('/api/sounds/upload', { method: 'POST', body: formData });
            if (resp.ok) {
                const data = await resp.json();
                const fileEl = document.getElementById(`sef-${event}`);
                if (fileEl) { fileEl.textContent = data.filename; fileEl.className = 'sound-event-file has-file'; }
                const packObj = this.soundPacks.find(p => p.dir === pack);
                if (packObj) {
                    if (!packObj.sounds) packObj.sounds = {};
                    packObj.sounds[event] = data.filename;
                    this.renderSoundEvents(packObj);
                }
                await this.sound.loadPack(pack);
            } else {
                const body = await resp.json().catch(() => ({}));
                alert('Erreur upload: ' + (body.error || resp.status));
            }
        } catch (e) { alert('Erreur: ' + e.message); }
        input.value = '';
    },

    async deleteSound(pack, event) {
        if (!confirm(`Supprimer le son "${event}" ?`)) return;
        try {
            const resp = await fetch(`/api/sounds/sound?pack=${encodeURIComponent(pack)}&event=${encodeURIComponent(event)}`, { method: 'DELETE' });
            if (resp.ok) {
                const packObj = this.soundPacks.find(p => p.dir === pack);
                if (packObj?.sounds) delete packObj.sounds[event];
                if (packObj) this.renderSoundEvents(packObj);
                await this.sound.loadPack(pack);
            }
        } catch (e) {}
    },

    async promptCreatePack() {
        const name = prompt('Nom du nouveau pack de sons :');
        if (!name?.trim()) return;
        try {
            const resp = await fetch('/api/sounds/packs', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ name: name.trim() })
            });
            if (resp.ok) await this.loadSoundSettings();
            else alert('Erreur création pack');
        } catch (e) {}
    },

    // ── Utilities ──────────────────────────────────────────
    esc(str) {
        const div = document.createElement('div');
        div.textContent = str;
        return div.innerHTML;
    }
};

document.addEventListener('DOMContentLoaded', () => app.init());
