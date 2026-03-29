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
        if (!this.ctx) return;
        try {
            const resp = await fetch(`/api/sounds/packs/manifest?pack=${encodeURIComponent(packName)}`);
            if (!resp.ok) return;
            const pack = await resp.json();
            if (!pack?.sounds) return;

            this.activePack = packName;
            this.buffers = {};

            const loadPromises = Object.entries(pack.sounds).map(([event, filename]) =>
                this.loadSound(event, `/sounds/${encodeURIComponent(packName)}/${encodeURIComponent(filename)}`)
            );
            await Promise.allSettled(loadPromises);
            console.log(`[Sound] Loaded pack "${packName}": ${Object.keys(this.buffers).length} sounds`);
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
            // Sound file missing or decode error — skip silently
        }
    }

    play(eventName) {
        if (!this.ctx || !this.buffers[eventName]) return;
        if (this.ctx.state === 'suspended') this.ctx.resume();

        const source   = this.ctx.createBufferSource();
        source.buffer  = this.buffers[eventName];

        const gain      = this.ctx.createGain();
        gain.gain.value = this.volume;

        source.connect(gain);
        gain.connect(this.ctx.destination);
        source.start(0);
    }

    playEvents(events) {
        if (!events || events.length === 0) return;

        // Always play generic throw sound first
        if (events.includes('throw')) this.play('throw');

        // High-priority match/visit events — play one and stop
        const priority = ['matchshot', 'gameshot', 'bust', '180', 'hatTrick', 'highTon', 'lowTon'];
        for (const p of priority) {
            if (events.includes(p)) {
                this.play(p);
                return; // don't play dart sound or caller on these
            }
        }

        // Dart-level sounds — play the best one
        const dartSounds = ['triple', 'dbull', 'bull', 'double', 'single', 'miss'];
        for (const d of dartSounds) {
            if (events.includes(d)) {
                this.play(d);
                break;
            }
        }

        // Caller sound — specific score first, then generic fallback
        const callerEvent = events.find(e => e.startsWith('caller_'));
        if (callerEvent) {
            setTimeout(() => {
                if (this.buffers[callerEvent]) {
                    this.play(callerEvent);
                } else if (this.buffers['caller']) {
                    this.play('caller');
                }
            }, 700);
        }
    }

    setVolume(v) {
        this.volume = Math.max(0, Math.min(1, v));
    }
}
