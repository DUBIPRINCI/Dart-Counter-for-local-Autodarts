// Sound Manager using Web Audio API
class SoundManager {
    constructor() {
        this.ctx = null;
        this.buffers = {};
        this.volume = 0.8;
        this.activePack = 'default';
        this.initialized = false;
    }

    async init() {
        if (this.initialized) return;
        try {
            this.ctx = new (window.AudioContext || window.webkitAudioContext)();
            this.initialized = true;
            await this.loadPack(this.activePack);
        } catch (e) {
            console.warn('Web Audio not available:', e);
        }
    }

    async loadPack(packName) {
        try {
            const resp = await fetch(`/api/sounds/packs`);
            const packs = await resp.json();
            const pack = packs.find(p => p.dir === packName);
            if (!pack) return;

            this.activePack = packName;
            this.buffers = {};

            const loadPromises = [];
            for (const [event, filename] of Object.entries(pack.sounds)) {
                loadPromises.push(this.loadSound(event, `/sounds/${packName}/${filename}`));
            }
            await Promise.allSettled(loadPromises);
        } catch (e) {
            console.warn('Failed to load sound pack:', e);
        }
    }

    async loadSound(name, url) {
        try {
            const resp = await fetch(url);
            if (!resp.ok) return;
            const data = await resp.arrayBuffer();
            this.buffers[name] = await this.ctx.decodeAudioData(data);
        } catch (e) {
            // Sound file not found, skip silently
        }
    }

    play(eventName) {
        if (!this.ctx || !this.buffers[eventName]) return;

        // Resume context if suspended (autoplay policy)
        if (this.ctx.state === 'suspended') {
            this.ctx.resume();
        }

        const source = this.ctx.createBufferSource();
        source.buffer = this.buffers[eventName];

        const gainNode = this.ctx.createGain();
        gainNode.gain.value = this.volume;

        source.connect(gainNode);
        gainNode.connect(this.ctx.destination);
        source.start(0);
    }

    playEvents(events) {
        if (!events || events.length === 0) return;

        // Priority: match events > visit events > dart events
        const priority = ['matchshot', 'gameshot', 'bust', '180', 'hatTrick', 'highTon', 'lowTon'];

        // Always play throw sound
        if (events.includes('throw')) {
            this.play('throw');
        }

        // Play highest priority event sound
        for (const p of priority) {
            if (events.includes(p)) {
                this.play(p);
                return;
            }
        }

        // Play dart-level sound
        const dartSounds = ['triple', 'dbull', 'bull', 'double', 'single', 'miss'];
        for (const d of dartSounds) {
            if (events.includes(d)) {
                this.play(d);
                return;
            }
        }
    }

    setVolume(v) {
        this.volume = Math.max(0, Math.min(1, v));
    }
}
