// Suggested path: music-server-frontend/src/components/AudioPlayer.jsx
import React, { useState, useEffect, useRef, useCallback } from 'react';
import { API_BASE, apiFetch } from '../api';
import WaveSurfer from 'wavesurfer.js';
import Hover from 'wavesurfer.js/dist/plugins/hover.esm.js';
import Hls from 'hls.js';
import './AudioPlayer.css';

function CustomAudioPlayer({ song, onEnded, credentials, onPlayNext, onPlayPrevious, hasQueue, onToggleQueueView, queueCount = 0, playMode = 'sequential', onTogglePlayMode }) {
    const [audioSrc, setAudioSrc] = useState(null);
    const [isLoading, setIsLoading] = useState(false);
    const [error, setError] = useState(false);
    const [currentTime, setCurrentTime] = useState(0);
    const [duration, setDuration] = useState(0);
    const [isPlaying, setIsPlaying] = useState(false);
    const [volume, setVolume] = useState(1.0);
    const [isMuted, setIsMuted] = useState(false);
    const audioRef = useRef(null);
    const hlsRef = useRef(null); // HLS.js instance
    const playPromiseRef = useRef(null);
    const seekingRef = useRef(false);
    const isDraggingRef = useRef(false);
    const previousSongIdRef = useRef(null);
    const waveformRef = useRef(null);
    const waveformMobileRef = useRef(null);
    const wavesurferRef = useRef(null);
    const wavesurferMobileRef = useRef(null);
    const seekDebounceRef = useRef(null);
    const isSeekingViaWaveformRef = useRef(false);

    // Effect for fetching audio data and scrobbling
    useEffect(() => {
        if (!song) {
            setAudioSrc(null);
            setError(false);
            setIsLoading(false);
            setDuration(0);
            setCurrentTime(0);
            previousSongIdRef.current = null;
            
            // Clean up HLS instance
            if (hlsRef.current) {
                hlsRef.current.destroy();
                hlsRef.current = null;
            }
            
            return;
        }
        
        // Check if this is actually a NEW song (not just a re-render)
        const isNewSong = previousSongIdRef.current !== song.id;
        
        if (isNewSong) {
            console.log('🎵 NEW SONG:', song.title, '(id:', song.id, ')');
            // Reset current time ONLY for new songs
            setCurrentTime(0);
            previousSongIdRef.current = song.id;
            
            // Clean up old HLS instance for new song
            if (hlsRef.current) {
                hlsRef.current.destroy();
                hlsRef.current = null;
            }
        }
        
        // Use song.duration from metadata immediately (duration is in seconds)
        // This ensures the progress bar shows correct duration even before stream loads
        if (song.duration && !isNaN(song.duration) && song.duration > 0) {
            console.log('⏱️ Setting duration from song metadata:', song.duration, 'seconds');
            setDuration(song.duration);
        } else {
            // If no duration in metadata, reset to 0 and wait for stream
            console.log('⏱️ No duration in song metadata, waiting for stream');
            setDuration(0);
        }

        setIsLoading(true);
        setError(false);

        const fetchAndSetAudio = async () => {
            try {
                // Scrobble the song play (fire and forget)
                if (credentials) {
                    try {
                        apiFetch(`/rest/scrobble.view?id=${encodeURIComponent(song.id)}`).catch(() => {});
                    } catch (e) {
                        console.error("Failed to scrobble song:", e);
                    }
                }

                // Get JWT token from localStorage
                const token = localStorage.getItem('token');
                if (!token) {
                    throw new Error('No authentication token found');
                }

                // Check if transcoding is enabled
                let transcodingSettings = { enabled: false };
                try {
                    const settingsResponse = await fetch(`${API_BASE}/api/v1/user/settings/transcoding`, {
                        headers: {
                            'Authorization': `Bearer ${token}`
                        }
                    });
                    if (settingsResponse.ok) {
                        transcodingSettings = await settingsResponse.json();
                    }
                } catch (e) {
                    console.warn('Could not fetch transcoding settings:', e);
                }

                console.log('🎵 Transcoding settings:', transcodingSettings);

                // Use HLS if transcoding is enabled
                if (transcodingSettings.enabled) {
                    console.log('📺 Using HLS transcoding for playback');
                    
                    // Build HLS playlist URL with JWT authentication
                    const hlsPlaylistUrl = `${API_BASE}/rest/hlsPlaylist.view?id=${encodeURIComponent(song.id)}&v=1.16.1&c=AudioMuse-AI&jwt=${encodeURIComponent(token)}&format=${transcodingSettings.format || 'mp3'}&maxBitRate=${transcodingSettings.bitrate || 192}`;
                    
                    console.log('📺 HLS Playlist URL:', hlsPlaylistUrl);
                    console.log('📺 Song ID being sent:', song.id);
                    
                    // Setup HLS.js
                    const audio = audioRef.current;
                    if (!audio) {
                        throw new Error('Audio element not available');
                    }

                    // Check for HLS support
                    if (Hls.isSupported()) {
                        console.log('📺 HLS.js is supported, initializing...');
                        
                        // Destroy old HLS instance if exists
                        if (hlsRef.current) {
                            console.log('📺 Destroying previous HLS instance');
                            hlsRef.current.destroy();
                            hlsRef.current = null;
                        }
                        
                        const hls = new Hls({
                            debug: false,
                            enableWorker: true,
                            lowLatencyMode: false,
                            backBufferLength: 90,
                            // Aggressive buffering to prevent gaps between segments
                            maxBufferLength: 30,        // Buffer up to 30 seconds ahead
                            maxMaxBufferLength: 60,     // Maximum buffer in seconds
                            maxBufferSize: 60 * 1000 * 1000, // 60MB buffer
                            maxBufferHole: 0.5,         // Jump over small gaps
                            // Start buffering next segment earlier
                            nudgeOffset: 0.1,           // Start fetching 0.1s before needed
                            nudgeMaxRetry: 3,           // Retry nudging
                            // Faster segment loading
                            manifestLoadingTimeOut: 10000,
                            manifestLoadingMaxRetry: 4,
                            manifestLoadingRetryDelay: 1000,
                            levelLoadingTimeOut: 10000,
                            levelLoadingMaxRetry: 4,
                            levelLoadingRetryDelay: 1000,
                            fragLoadingTimeOut: 20000,
                            fragLoadingMaxRetry: 6,
                            fragLoadingRetryDelay: 1000
                        });

                        hls.loadSource(hlsPlaylistUrl);
                        hls.attachMedia(audio);

                        hls.on(Hls.Events.MANIFEST_PARSED, () => {
                            console.log('📺 HLS manifest parsed, ready to play');
                            setError(false);
                            setIsLoading(false);
                            
                            // Always auto-play when song loads (matches original behavior)
                            if (audioRef.current) {
                                console.log('📺 Auto-playing HLS stream');
                                setTimeout(() => {
                                    audioRef.current?.play().catch(e => console.error('📺 Auto-play failed:', e));
                                }, 100);
                            }
                        });

                        // Monitor buffer levels to detect stalls
                        hls.on(Hls.Events.FRAG_BUFFERED, (event, data) => {
                            const buffered = audioRef.current?.buffered;
                            if (buffered && buffered.length > 0) {
                                const bufferEnd = buffered.end(buffered.length - 1);
                                const currentTime = audioRef.current?.currentTime || 0;
                                const bufferAhead = bufferEnd - currentTime;
                                if (bufferAhead < 5) {
                                    console.log(`📺 Buffer low: ${bufferAhead.toFixed(2)}s ahead (loading segment ${data.frag.sn})`);
                                }
                            }
                        });

                        // Log when starting to load fragments
                        hls.on(Hls.Events.FRAG_LOADING, (event, data) => {
                            const buffered = audioRef.current?.buffered;
                            if (buffered && buffered.length > 0) {
                                const bufferEnd = buffered.end(buffered.length - 1);
                                const currentTime = audioRef.current?.currentTime || 0;
                                const bufferAhead = bufferEnd - currentTime;
                                console.log(`📺 Loading segment ${data.frag.sn} (buffer: ${bufferAhead.toFixed(2)}s ahead)`);
                            }
                        });

                        hls.on(Hls.Events.ERROR, (event, data) => {
                            console.error('📺 HLS error:', data);
                            if (data.fatal) {
                                switch (data.type) {
                                    case Hls.ErrorTypes.NETWORK_ERROR:
                                        console.error('📺 Fatal network error, trying to recover');
                                        hls.startLoad();
                                        break;
                                    case Hls.ErrorTypes.MEDIA_ERROR:
                                        console.error('📺 Fatal media error, trying to recover');
                                        hls.recoverMediaError();
                                        break;
                                    default:
                                        console.error('📺 Fatal error, cannot recover');
                                        hls.destroy();
                                        setError(true);
                                        break;
                                }
                            }
                        });

                        hlsRef.current = hls;
                        setAudioSrc(null); // Not needed for HLS.js
                        
                    } else if (audio.canPlayType('application/vnd.apple.mpegurl')) {
                        // Native HLS support (Safari/iOS)
                        console.log('📺 Using native HLS support (Safari/iOS)');
                        audio.src = hlsPlaylistUrl;
                        setAudioSrc(hlsPlaylistUrl);
                        setError(false);
                        setIsLoading(false);
                    } else {
                        console.error('📺 HLS not supported on this browser');
                        throw new Error('HLS not supported');
                    }
                    
                } else {
                    // Direct streaming (no transcoding)
                    console.log('🎵 Using direct streaming (no transcoding)');
                    
                    // Clean up HLS if it was previously active
                    if (hlsRef.current) {
                        console.log('🧹 Cleaning up HLS instance for direct stream');
                        hlsRef.current.destroy();
                        hlsRef.current = null;
                    }
                    
                    // Build stream URL with JWT in query string (for <audio> element direct streaming)
                    const streamUrl = `${API_BASE}/rest/stream.view?id=${encodeURIComponent(song.id)}&v=1.16.1&c=AudioMuse-AI&jwt=${encodeURIComponent(token)}`;
                    
                    // Fetch X-Content-Duration header from stream (if no duration in metadata)
                    if (!song.duration || song.duration === 0) {
                        try {
                            console.log('⏱️ Fetching duration from stream headers...');
                            const response = await fetch(streamUrl, { 
                                method: 'GET',
                                headers: {
                                    'Authorization': `Bearer ${token}`,
                                    'Range': 'bytes=0-0' // Request only 1 byte to get headers quickly
                                }
                            });
                            
                            const xContentDuration = response.headers.get('X-Content-Duration');
                            console.log('⏱️ X-Content-Duration header value:', xContentDuration);
                            
                            if (xContentDuration) {
                                const durationSeconds = parseInt(xContentDuration, 10);
                                if (!isNaN(durationSeconds) && durationSeconds > 0) {
                                    console.log('⏱️ ✅ Got duration from X-Content-Duration header:', durationSeconds, 'seconds');
                                    setDuration(durationSeconds);
                                }
                            } else {
                                console.warn('⏱️ ❌ No X-Content-Duration header in response');
                            }
                            
                            // Abort the response body to avoid downloading the full file
                            response.body?.cancel();
                        } catch (e) {
                            console.warn('⏱️ Could not fetch X-Content-Duration header:', e);
                        }
                    }
                    
                    console.log('🎵 Setting direct stream URL for immediate playback:', song.title);
                    
                    // Set URL directly - browser will handle progressive streaming natively!
                    setAudioSrc(streamUrl);
                    setError(false);
                    setIsLoading(false);
                }
                
            } catch (err) {
                console.error("Error setting up audio stream:", err);
                setError(true);
                setAudioSrc(null);
                setIsLoading(false);
            }
        };

        const timer = setTimeout(fetchAndSetAudio, 50);

        return () => {
            clearTimeout(timer);
        };
    }, [song, credentials]);

    const setupMediaSession = useCallback(() => {
        if (song && 'mediaSession' in navigator) {
            // Only include artwork if song has coverArt and credentials are available
            const artwork = [];
            if ((song.coverArt || song.id) && credentials?.username && credentials?.password) {
                const params = new URLSearchParams({ 
                    id: song.coverArt || song.id, 
                    v: '1.16.1', 
                    c: 'AudioMuse-AI',
                    u: credentials.username,
                    p: credentials.password
                });
                const artworkUrl = `${API_BASE}/rest/getCoverArt.view?${params.toString()}`;
                artwork.push({ src: artworkUrl, sizes: '300x300', type: 'image/jpeg' });
            }
            
            navigator.mediaSession.metadata = new window.MediaMetadata({
                title: song.title,
                artist: song.artist,
                album: song.album,
                artwork: artwork
            });

            navigator.mediaSession.setActionHandler('play', () => audioRef.current?.play());
            navigator.mediaSession.setActionHandler('pause', () => audioRef.current?.pause());
            navigator.mediaSession.setActionHandler('nexttrack', hasQueue ? onPlayNext : null);
            navigator.mediaSession.setActionHandler('previoustrack', hasQueue ? onPlayPrevious : null);
        }
    }, [song, credentials, hasQueue, onPlayNext, onPlayPrevious]);

    useEffect(() => {
        setupMediaSession();
    }, [song, setupMediaSession]);
    
    // Initialize WaveSurfer instances and load audio when song changes
    useEffect(() => {
        if (!waveformRef.current || !waveformMobileRef.current) {
            return;
        }

        // Only load waveform if we have a song
        if (!song || !song.id) {
            return;
        }

        console.log('🌊 Loading waveform for song:', song.id);
        
        // Capture current duration at effect start to avoid re-renders from duration updates
        const initialDuration = song.duration || duration;

        let cleanupFn = null; // Store cleanup function

        // Fetch pre-computed waveform peaks for instant rendering
        const loadWaveform = async () => {
            try {
                const token = localStorage.getItem('token');
                const waveformUrl = `${API_BASE}/rest/waveform.view?id=${encodeURIComponent(song.id)}&v=1.16.1&c=AudioMuse-AI&jwt=${encodeURIComponent(token)}&_t=${Date.now()}`; // Add timestamp to prevent caching
                
                console.log('🌊 Fetching pre-computed waveform peaks for song:', song.id);
                const response = await fetch(waveformUrl, {
                    cache: 'no-store' // Disable browser caching
                });
                const data = await response.json();
                
                if (data.peaks) {
                    console.log(`✅ Received ${data.peaks.length} waveform peaks for duration ${data.duration}s`);
                    
                    // Destroy any existing instances before creating new ones
                    if (wavesurferRef.current) {
                        wavesurferRef.current.destroy();
                        wavesurferRef.current = null;
                    }
                    if (wavesurferMobileRef.current) {
                        wavesurferMobileRef.current.destroy();
                        wavesurferMobileRef.current = null;
                    }
                    
                    // Create desktop waveform with pre-computed peaks
                    const wavesurfer = WaveSurfer.create({
                        container: waveformRef.current,
                        waveColor: '#4B5563',
                        progressColor: '#14b8a6',
                        cursorColor: '#14b8a6',
                        height: 48,
                        normalize: true,
                        interact: true,
                        hideScrollbar: true,
                        fillParent: true,
                        peaks: [data.peaks], // Use pre-computed peaks
                        duration: data.duration || initialDuration, // Use captured duration
                        plugins: [
                            Hover.create({
                                lineColor: '#14b8a6',
                                lineWidth: 2,
                                labelBackground: '#14b8a6',
                                labelColor: '#fff',
                                labelSize: '11px'
                            })
                        ]
                    });

                    // Create mobile waveform with pre-computed peaks
                    const wavesurferMobile = WaveSurfer.create({
                        container: waveformMobileRef.current,
                        waveColor: '#4B5563',
                        progressColor: '#14b8a6',
                        cursorColor: '#14b8a6',
                        height: 32,
                        normalize: true,
                        interact: true,
                        hideScrollbar: true,
                        fillParent: true,
                        peaks: [data.peaks], // Use pre-computed peaks
                        duration: data.duration || initialDuration // Use captured duration
                    });

                    wavesurferRef.current = wavesurfer;
                    wavesurferMobileRef.current = wavesurferMobile;

                    cleanupFn = setupWaveformInteraction(wavesurfer, wavesurferMobile);
                } else {
                    // Fallback to loading from audio URL
                    console.warn('⚠️ No peaks data, falling back to audio URL');
                    cleanupFn = createWaveformFromAudio();
                }
            } catch (error) {
                console.error('❌ Failed to load waveform peaks:', error);
                // Fallback to loading from audio URL
                cleanupFn = createWaveformFromAudio();
            }
        };

        // Fallback: Create waveform from audio URL (slow)
        const createWaveformFromAudio = () => {
            console.log('🌊 Creating waveform from audio URL (slow method)');
            
            // Destroy any existing instances before creating new ones
            if (wavesurferRef.current) {
                wavesurferRef.current.destroy();
                wavesurferRef.current = null;
            }
            if (wavesurferMobileRef.current) {
                wavesurferMobileRef.current.destroy();
                wavesurferMobileRef.current = null;
            }
            
            const wavesurfer = WaveSurfer.create({
                container: waveformRef.current,
                waveColor: '#4B5563',
                progressColor: '#14b8a6',
                cursorColor: '#14b8a6',
                height: 48,
                normalize: true,
                interact: true,
                hideScrollbar: true,
                fillParent: true,
                url: audioSrc,
                plugins: [
                    Hover.create({
                        lineColor: '#14b8a6',
                        lineWidth: 2,
                        labelBackground: '#14b8a6',
                        labelColor: '#fff',
                        labelSize: '11px'
                    })
                ]
            });

            const wavesurferMobile = WaveSurfer.create({
                container: waveformMobileRef.current,
                waveColor: '#4B5563',
                progressColor: '#14b8a6',
                cursorColor: '#14b8a6',
                height: 32,
                normalize: true,
                interact: true,
                hideScrollbar: true,
                fillParent: true,
                url: audioSrc
            });

            wavesurferRef.current = wavesurfer;
            wavesurferMobileRef.current = wavesurferMobile;

            return setupWaveformInteraction(wavesurfer, wavesurferMobile);
        };

        // Setup interaction handlers (extracted to avoid duplication)
        const setupWaveformInteraction = (wavesurfer, wavesurferMobile) => {
            // Handle seeking via waveform click with debouncing
            wavesurfer.on('interaction', (timeInSeconds) => {
                console.log('🌊 Waveform click - time:', timeInSeconds);
                
                // Prevent visual update during debounce
                isSeekingViaWaveformRef.current = true;
                
                // Clear any pending seek
                if (seekDebounceRef.current) {
                    clearTimeout(seekDebounceRef.current);
                }
                
                // Debounce the seek - wait 500ms before actually seeking
                seekDebounceRef.current = setTimeout(() => {
                    if (audioRef.current) {
                        console.log('🌊 Seeking audio to:', timeInSeconds);
                        audioRef.current.currentTime = timeInSeconds;
                        isSeekingViaWaveformRef.current = false;
                    }
                }, 500);
            });

            wavesurferMobile.on('interaction', (timeInSeconds) => {
                console.log('🌊 Mobile waveform click - time:', timeInSeconds);
                
                // Prevent visual update during debounce
                isSeekingViaWaveformRef.current = true;
                
                // Clear any pending seek
                if (seekDebounceRef.current) {
                    clearTimeout(seekDebounceRef.current);
                }
                
                // Debounce the seek - wait 500ms before actually seeking
                seekDebounceRef.current = setTimeout(() => {
                    if (audioRef.current) {
                        console.log('🌊 Mobile seeking audio to:', timeInSeconds);
                        audioRef.current.currentTime = timeInSeconds;
                        isSeekingViaWaveformRef.current = false;
                    }
                }, 500);
            });

            // Sync waveform progress with our audio element
            const syncHandler = () => {
                // Don't sync if we're waiting for a debounced seek
                if (isSeekingViaWaveformRef.current) {
                    return;
                }
                
                if (audioRef.current && audioRef.current.duration > 0 && isFinite(audioRef.current.duration)) {
                    const progress = audioRef.current.currentTime / audioRef.current.duration;
                    if (isFinite(progress) && progress >= 0 && progress <= 1) {
                        wavesurfer?.seekTo(progress);
                        wavesurferMobile?.seekTo(progress);
                    }
                }
            };

            // Attach to audio element events
            const audio = audioRef.current;
            if (audio) {
                audio.addEventListener('timeupdate', syncHandler);
                audio.addEventListener('seeking', syncHandler);
            }

            return () => {
                if (audio) {
                    audio.removeEventListener('timeupdate', syncHandler);
                    audio.removeEventListener('seeking', syncHandler);
                }
                wavesurfer.destroy();
                wavesurferMobile.destroy();
            };
        };

        // Start loading waveform
        loadWaveform();

        // Return cleanup function
        return () => {
            console.log('🧹 Cleaning up WaveSurfer instances');
            if (cleanupFn) {
                cleanupFn();
            }
            if (wavesurferRef.current) {
                wavesurferRef.current.destroy();
                wavesurferRef.current = null;
            }
            if (wavesurferMobileRef.current) {
                wavesurferMobileRef.current.destroy();
                wavesurferMobileRef.current = null;
            }
        };

    // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [song?.id]); // Only reload waveform when song changes, not when duration updates


    
    useEffect(() => {
        if (audioSrc && audioRef.current) {
            console.log('🎵 audioSrc changed, loading new audio');
            
            // Cancel any pending play promise before loading new audio
            if (playPromiseRef.current) {
                playPromiseRef.current.catch(() => {
                    // Suppress the abort error - it's expected when switching songs
                });
                playPromiseRef.current = null;
            }

            // Check if this song was added from Map - if so, don't auto-play
            if (window._mapAddedSong) {
                console.log('Map song detected - skipping auto-play');
                window._mapAddedSong = false;
                return;
            }
            
            // Store the play promise and handle it properly
            playPromiseRef.current = audioRef.current.play();
            playPromiseRef.current
                .then(() => {
                    console.log('🎵 Playback started successfully');
                    playPromiseRef.current = null;
                })
                .catch(e => {
                    // Only log if it's not an abort error (which happens when switching songs)
                    if (e.name !== 'AbortError') {
                        console.error("Autoplay was prevented:", e);
                    }
                    playPromiseRef.current = null;
                });
        }
    }, [audioSrc]);
    
    // Separate effect to handle volume/mute changes without restarting playback
    useEffect(() => {
        if (audioRef.current) {
            audioRef.current.volume = volume;
            audioRef.current.muted = isMuted;
        }
    }, [volume, isMuted]);

    return (
        <div className="fixed bottom-0 left-0 right-0 glass border-t border-dark-600 z-50 shadow-2xl">
            {/* Mobile Timeline - At the very top on mobile only */}
            <div className={`sm:hidden px-2 py-1.5 bg-dark-800/30 ${!song ? 'hidden' : ''}`}>
                <div className="flex items-center gap-1">
                    <span className="text-[9px] text-gray-400 whitespace-nowrap flex-shrink-0 w-8 text-right">
                        {Math.floor(currentTime / 60)}:{String(Math.floor(currentTime % 60)).padStart(2, '0')}
                    </span>
                    <div ref={waveformMobileRef} className="flex-1 h-8" />
                    <span className="text-[9px] text-gray-400 whitespace-nowrap flex-shrink-0 w-8">
                        {Math.floor(duration / 60)}:{String(Math.floor(duration % 60)).padStart(2, '0')}
                    </span>
                </div>
            </div>
            <div className="container mx-auto px-1 sm:px-6 py-2 sm:py-3">
                <div className="flex items-center gap-1 sm:gap-6">
                    {/* Song Info - More compact on mobile */}
                    <div className="flex items-center gap-1.5 sm:gap-2 flex-shrink-0 w-32 sm:w-64 overflow-hidden">
                        {/* Album art only on desktop */}
                        {song && (song.coverArt || song.id) && credentials?.username && credentials?.password && (
                            <div className="hidden sm:block relative group">
                                <img 
                                    src={`${API_BASE}/rest/getCoverArt.view?id=${encodeURIComponent(song.coverArt || song.id)}&size=60&v=1.16.1&c=AudioMuse-AI&u=${encodeURIComponent(credentials.username)}&p=${encodeURIComponent(credentials.password)}`}
                                    alt={song.title}
                                    className="w-14 h-14 rounded-lg shadow-lg object-cover"
                                    onError={(e) => {
                                        // Hide image if it fails to load
                                        e.target.style.display = 'none';
                                    }}
                                />
                                {isLoading && (
                                    <div className="absolute inset-0 bg-black bg-opacity-50 rounded-lg flex items-center justify-center">
                                        <svg className="animate-spin h-5 w-5 text-accent-400" fill="none" viewBox="0 0 24 24">
                                            <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                                            <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                                        </svg>
                                    </div>
                                )}
                            </div>
                        )}
                        
                        <div className="overflow-hidden flex-1 min-w-0">
                            {isLoading && !error && (
                                <div className="space-y-1">
                                    <div className="h-3 bg-dark-700 rounded skeleton w-3/4"></div>
                                    <div className="h-2 bg-dark-700 rounded skeleton w-1/2"></div>
                                </div>
                            )}
                            {error && <p className="font-semibold truncate text-xs sm:text-sm text-red-400">Error Loading Track</p>}
                            {!isLoading && !error && song && (
                                <>
                                    <p className="font-semibold truncate text-[11px] sm:text-base text-white leading-tight">{song.title}</p>
                                    <p className="text-[9px] sm:text-sm text-gray-400 truncate leading-tight">{song.artist}</p>
                                </>
                            )}
                            {!song && <div className="text-gray-500 text-[10px] sm:text-sm">Select a song</div>}
                        </div>
                    </div>

                    {/* Playback Controls - Centered (Desktop & Mobile) - << Play >> layout */}
                    <div className="flex items-center justify-center gap-0.5 sm:gap-2 flex-1">
                        <audio
                            ref={audioRef}
                            src={audioSrc || ''}
                            preload="metadata"
                            onPlay={() => {
                                setupMediaSession();
                                setIsPlaying(true);
                            }}
                            onPause={() => setIsPlaying(false)}
                            onEnded={() => {
                                console.log('🎵 Song ended, calling onEnded');
                                setIsPlaying(false);
                                onEnded();
                            }}
                            onSeeking={() => {
                                seekingRef.current = true;
                            }}
                            onSeeked={() => {
                                seekingRef.current = false;
                                // Force update currentTime after seek completes to ensure accuracy
                                if (audioRef.current && !isDraggingRef.current) {
                                    setCurrentTime(audioRef.current.currentTime);
                                }
                            }}
                            onTimeUpdate={(e) => {
                                // Don't update time while user is dragging or seeking to avoid jitter
                                if (!isDraggingRef.current && !seekingRef.current) {
                                    const time = e.target.currentTime;
                                    setCurrentTime(time);
                                }
                                
                                const dur = e.target.duration;
                                // Only update duration if stream provides valid duration AND we don't have metadata duration
                                // This prevents overwriting our database duration with Infinity
                                if (dur && !isNaN(dur) && dur !== Infinity && dur > 0) {
                                    // Use stream duration if it's more accurate (has decimals)
                                    setDuration(dur);
                                }
                                // If duration is still 0 or invalid, keep using metadata duration from state
                            }}
                            onDurationChange={(e) => {
                                console.log('⏱️ onDurationChange - duration:', e.target.duration);
                                // Only update if stream provides valid duration
                                if (e.target.duration && !isNaN(e.target.duration) && e.target.duration !== Infinity && e.target.duration > 0) {
                                    console.log('⏱️ Setting duration from stream:', e.target.duration);
                                    setDuration(e.target.duration);
                                } else {
                                    console.log('⏱️ Stream duration invalid, keeping metadata duration:', duration);
                                }
                            }}
                            onLoadedMetadata={(e) => {
                                console.log('⏱️ onLoadedMetadata - duration:', e.target.duration, 'current duration state:', duration);
                                // Only update if stream metadata has valid duration
                                if (e.target.duration && !isNaN(e.target.duration) && e.target.duration !== Infinity && e.target.duration > 0) {
                                    console.log('⏱️ Setting duration from metadata:', e.target.duration);
                                    setDuration(e.target.duration);
                                } else {
                                    console.log('⏱️ Metadata duration invalid, keeping existing:', duration);
                                }
                                
                                // Auto-play when metadata loads (matches original behavior)
                                if (audioRef.current && audioSrc) {
                                    console.log('🎵 Auto-playing direct stream');
                                    audioRef.current.play().catch(e => console.error('🎵 Auto-play failed:', e));
                                }
                            }}
                            style={{ display: 'none' }}
                        />
                        {song && (
                            <>
                                {/* Playback Control Buttons Group - Previous, Play/Pause, Next */}
                                <div className="flex items-center gap-0.5 sm:gap-1">
                                    {/* Previous Button */}
                                    <button 
                                        onClick={onPlayPrevious}
                                        disabled={!hasQueue}
                                        className={`p-1.5 sm:p-2 rounded-full transition-all group ${hasQueue ? 'text-gray-300 hover:text-white hover:bg-dark-700' : 'text-gray-600 cursor-not-allowed opacity-50'}`}
                                        title="Previous"
                                    >
                                        <svg className="w-4 h-4 sm:w-5 sm:h-5 group-hover:scale-110 transition-transform" fill="currentColor" viewBox="0 0 20 20">
                                            <path d="M8.445 14.832A1 1 0 0010 14v-2.798l5.445 3.63A1 1 0 0017 14V6a1 1 0 00-1.555-.832L10 8.798V6a1 1 0 00-1.555-.832l-6 4a1 1 0 000 1.664l6 4z"></path>
                                        </svg>
                                    </button>
                                    
                                    {/* Play/Pause Button */}
                                    <button 
                                        onClick={() => {
                                            if (audioRef.current) {
                                                if (audioRef.current.paused) {
                                                    audioRef.current.play().catch(e => console.error("Playback error:", e));
                                                } else {
                                                    audioRef.current.pause();
                                                }
                                            }
                                        }} 
                                        className="text-white hover:text-accent-400 p-1.5 sm:p-2 rounded-full hover:bg-dark-700 transition-all group" 
                                        title={isPlaying ? "Pause" : "Play"}
                                    >
                                        {isPlaying ? (
                                            <svg className="w-5 h-5 sm:w-8 sm:h-8" fill="currentColor" viewBox="0 0 20 20">
                                                <path d="M5.75 3a.75.75 0 00-.75.75v12.5c0 .414.336.75.75.75h1.5a.75.75 0 00.75-.75V3.75A.75.75 0 007.25 3h-1.5zM12.75 3a.75.75 0 00-.75.75v12.5c0 .414.336.75.75.75h1.5a.75.75 0 00.75-.75V3.75a.75.75 0 00-.75-.75h-1.5z"></path>
                                            </svg>
                                        ) : (
                                            <svg className="w-5 h-5 sm:w-8 sm:h-8" fill="currentColor" viewBox="0 0 20 20">
                                                <path d="M6.3 2.841A1.5 1.5 0 004 4.11V15.89a1.5 1.5 0 002.3 1.269l9.344-5.89a1.5 1.5 0 000-2.538L6.3 2.84z"></path>
                                            </svg>
                                        )}
                                    </button>
                                    
                                    {/* Next Button */}
                                    <button 
                                        onClick={onPlayNext}
                                        disabled={!hasQueue}
                                        className={`p-1.5 sm:p-2 rounded-full transition-all group ${hasQueue ? 'text-gray-300 hover:text-white hover:bg-dark-700' : 'text-gray-600 cursor-not-allowed opacity-50'}`}
                                        title="Next"
                                    >
                                        <svg className="w-4 h-4 sm:w-5 sm:h-5 group-hover:scale-110 transition-transform" fill="currentColor" viewBox="0 0 20 20">
                                            <path d="M10 5.99v8.02a1 1 0 001.555.832l6-4a1 1 0 000-1.664l-6-4A1 1 0 0010 5.99z"></path>
                                            <path d="M3 5.99v8.02a1 1 0 001.555.832l6-4a1 1 0 000-1.664l-6-4A1 1 0 003 5.99z"></path>
                                        </svg>
                                    </button>
                                </div>
                                {/* Desktop Timeline - Waveform between playback controls and volume */}
                                <div className={`hidden sm:flex items-center gap-2 flex-1 max-w-md ${!song ? 'invisible' : ''}`}>
                                    <span className="text-xs text-gray-400 whitespace-nowrap">
                                        {Math.floor(currentTime / 60)}:{String(Math.floor(currentTime % 60)).padStart(2, '0')}
                                    </span>
                                    <div ref={waveformRef} className="flex-1 h-12" />
                                    <span className="text-xs text-gray-400 whitespace-nowrap">
                                        {Math.floor(duration / 60)}:{String(Math.floor(duration % 60)).padStart(2, '0')}
                                    </span>
                                </div>
                                {/* Volume Control - Desktop Only */}
                                <div className="hidden sm:flex items-center gap-2 ml-2">
                                    <button
                                        onClick={() => {
                                            if (audioRef.current) {
                                                const newMuted = !isMuted;
                                                audioRef.current.muted = newMuted;
                                                setIsMuted(newMuted);
                                            }
                                        }}
                                        className="text-gray-300 hover:text-white p-1 rounded transition-all"
                                        title={isMuted ? "Unmute" : "Mute"}
                                    >
                                        {isMuted || volume === 0 ? (
                                            <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
                                                <path fillRule="evenodd" d="M9.383 3.076A1 1 0 0110 4v12a1 1 0 01-1.707.707L4.586 13H2a1 1 0 01-1-1V8a1 1 0 011-1h2.586l3.707-3.707a1 1 0 011.09-.217zM12.293 7.293a1 1 0 011.414 0L15 8.586l1.293-1.293a1 1 0 111.414 1.414L16.414 10l1.293 1.293a1 1 0 01-1.414 1.414L15 11.414l-1.293 1.293a1 1 0 01-1.414-1.414L13.586 10l-1.293-1.293a1 1 0 010-1.414z" clipRule="evenodd"></path>
                                            </svg>
                                        ) : volume < 0.5 ? (
                                            <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
                                                <path fillRule="evenodd" d="M9.383 3.076A1 1 0 0110 4v12a1 1 0 01-1.707.707L4.586 13H2a1 1 0 01-1-1V8a1 1 0 011-1h2.586l3.707-3.707a1 1 0 011.09-.217zM14 8a1 1 0 011 1v2a1 1 0 11-2 0V9a1 1 0 011-1z" clipRule="evenodd"></path>
                                            </svg>
                                        ) : (
                                            <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
                                                <path fillRule="evenodd" d="M9.383 3.076A1 1 0 0110 4v12a1 1 0 01-1.707.707L4.586 13H2a1 1 0 01-1-1V8a1 1 0 011-1h2.586l3.707-3.707a1 1 0 011.09-.217zM14.657 2.929a1 1 0 011.414 0A9.972 9.972 0 0119 10a9.972 9.972 0 01-2.929 7.071 1 1 0 01-1.414-1.414A7.971 7.971 0 0017 10c0-2.21-.894-4.208-2.343-5.657a1 1 0 010-1.414zm-2.829 2.828a1 1 0 011.415 0A5.983 5.983 0 0115 10a5.984 5.984 0 01-1.757 4.243 1 1 0 01-1.415-1.415A3.984 3.984 0 0013 10a3.983 3.983 0 00-1.172-2.828 1 1 0 010-1.415z" clipRule="evenodd"></path>
                                            </svg>
                                        )}
                                    </button>
                                    <input
                                        type="range"
                                        min="0"
                                        max="1"
                                        step="0.01"
                                        value={isMuted ? 0 : volume}
                                        onChange={(e) => {
                                            const newVolume = parseFloat(e.target.value);
                                            setVolume(newVolume);
                                            if (audioRef.current) {
                                                audioRef.current.volume = newVolume;
                                                if (newVolume > 0 && isMuted) {
                                                    audioRef.current.muted = false;
                                                    setIsMuted(false);
                                                }
                                            }
                                        }}
                                        className="w-20 h-2 bg-dark-700 rounded-lg appearance-none cursor-pointer"
                                        style={{
                                            background: `linear-gradient(to right, #14b8a6 0%, #14b8a6 ${(isMuted ? 0 : volume) * 100}%, #374151 ${(isMuted ? 0 : volume) * 100}%, #374151 100%)`
                                        }}
                                    />
                                </div>
                            </>
                        )}
                    </div>

                    {/* Right Controls Group - Play Mode & Queue Buttons - Compact on mobile */}
                    <div className="flex-shrink-0 flex items-center gap-0.5 sm:gap-2">
                        {/* Play Mode Button */}
                        <button 
                            onClick={onTogglePlayMode}
                            className={`p-1 sm:p-2.5 rounded-lg transition-all ${
                                playMode === 'sequential' ? 'text-gray-400 hover:text-white hover:bg-dark-700' :
                                'text-accent-400 hover:text-accent-300 hover:bg-accent-500/10'
                            }`}
                            title={
                                playMode === 'sequential' ? 'Sequential Play (Click for Shuffle)' :
                                'Shuffle Mode (Click for Sequential)'
                            }
                        >
                            {playMode === 'sequential' ? (
                                // Sequential icon - right arrow (-->)
                                <svg className="w-4 h-4 sm:w-5 sm:h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M14 5l7 7m0 0l-7 7m7-7H3"></path>
                                </svg>
                            ) : (
                                // Shuffle icon - crossing arrows
                                <svg className="w-4 h-4 sm:w-5 sm:h-5" fill="currentColor" viewBox="0 0 20 20">
                                    <path fillRule="evenodd" d="M4 2a1 1 0 011 1v2.101a7.002 7.002 0 0111.601 2.566 1 1 0 11-1.885.666A5.002 5.002 0 005.999 7H9a1 1 0 010 2H4a1 1 0 01-1-1V3a1 1 0 011-1zm.008 9.057a1 1 0 011.276.61A5.002 5.002 0 0014.001 13H11a1 1 0 110-2h5a1 1 0 011 1v5a1 1 0 11-2 0v-2.101a7.002 7.002 0 01-11.601-2.566 1 1 0 01.61-1.276z" clipRule="evenodd"></path>
                                </svg>
                            )}
                        </button>

                        {/* Queue Button with badge */}
                        <button 
                            onClick={onToggleQueueView} 
                            className={`p-1 sm:p-2.5 rounded-lg transition-all relative ${queueCount > 0 ? 'text-accent-400 hover:bg-accent-500/10' : 'text-gray-400 hover:bg-dark-700'}`}
                            title={`Queue (${queueCount} songs)`}
                        >
                            <svg className="w-4 h-4 sm:w-5 sm:h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M4 6h16M4 12h16M4 18h16"></path>
                            </svg>
                            {/* Queue count badge - only show on mobile when > 0 */}
                            {queueCount > 0 && (
                                <span className="absolute -top-0.5 -right-0.5 sm:-top-1 sm:-right-1 bg-accent-500 text-white text-[8px] sm:text-[10px] font-bold rounded-full min-w-[14px] h-[14px] sm:min-w-[16px] sm:h-[16px] flex items-center justify-center px-0.5">
                                    {queueCount > 99 ? '99+' : queueCount}
                                </span>
                            )}
                        </button>
                    </div>
                </div>
            </div>
        </div>
    );
}

export default CustomAudioPlayer;
