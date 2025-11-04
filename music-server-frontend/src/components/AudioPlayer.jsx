// Suggested path: music-server-frontend/src/components/AudioPlayer.jsx
import React, { useState, useEffect, useRef, useCallback } from 'react';
import { API_BASE, apiFetch } from '../api';

function CustomAudioPlayer({ song, onEnded, credentials, onPlayNext, onPlayPrevious, hasQueue, onToggleQueueView, queueCount = 0 }) {
    const [audioSrc, setAudioSrc] = useState(null);
    const [isLoading, setIsLoading] = useState(false);
    const [error, setError] = useState(false);
    const [currentTime, setCurrentTime] = useState(0);
    const [duration, setDuration] = useState(0);
    const audioRef = useRef(null);

    // Effect for fetching audio data and scrobbling
    useEffect(() => {
        if (!song) {
            setAudioSrc(null);
            setError(false);
            setIsLoading(false);
            return;
        }

        setIsLoading(true);
        setError(false);
        let objectUrl;

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

                // Get JWT token from localStorage for direct URL streaming
                const token = localStorage.getItem('token');
                if (!token) {
                    throw new Error('No authentication token found');
                }

                // Build stream URL with JWT in query string (for <audio> element direct streaming)
                // This allows the browser to natively stream without blob buffering
                const streamUrl = `${API_BASE}/rest/stream.view?id=${encodeURIComponent(song.id)}&v=1.16.1&c=AudioMuse-AI&jwt=${encodeURIComponent(token)}`;
                
                console.log('🎵 Setting direct stream URL for immediate playback:', song.title);
                
                // Set URL directly - browser will handle progressive streaming natively!
                setAudioSrc(streamUrl);
                setError(false);
                setIsLoading(false);
                
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
            if (objectUrl) {
                URL.revokeObjectURL(objectUrl);
            }
        };
    }, [song, credentials]);

    const setupMediaSession = useCallback(() => {
        if (song && 'mediaSession' in navigator) {
            // Only include artwork if song has coverArt and credentials are available
            const artwork = [];
            if (song.coverArt && credentials?.username && credentials?.password) {
                const params = new URLSearchParams({ 
                    id: song.coverArt, 
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
    
    useEffect(() => {
        if (audioSrc && audioRef.current) {
            // Check if this song was added from Map - if so, don't auto-play
            if (window._mapAddedSong) {
                console.log('Map song detected - skipping auto-play');
                window._mapAddedSong = false;
                return;
            }
            audioRef.current.play().catch(e => console.error("Autoplay was prevented:", e));
        }
    }, [audioSrc]);

    return (
        <div className="fixed bottom-0 left-0 right-0 glass border-t border-dark-600 z-50 shadow-2xl">
            {/* Mobile-only progress bar at the very top */}
            {song && duration > 0 && (
                <div className="sm:hidden">
                    <input
                        type="range"
                        min="0"
                        max={duration}
                        value={currentTime}
                        onChange={(e) => {
                            if (audioRef.current) {
                                audioRef.current.currentTime = parseFloat(e.target.value);
                            }
                        }}
                        className="w-full h-2 bg-dark-700 appearance-none cursor-pointer"
                        style={{
                            background: `linear-gradient(to right, #14b8a6 0%, #14b8a6 ${(currentTime / duration * 100) || 0}%, #374151 ${(currentTime / duration * 100) || 0}%, #374151 100%)`
                        }}
                    />
                </div>
            )}
            <div className="container mx-auto px-2 sm:px-6 py-2 sm:py-3">
                <div className="flex items-center gap-2 sm:gap-6">
                    {/* Album Art & Song Info - Fixed width on desktop to prevent layout shift */}
                    <div className="flex items-center gap-2 flex-shrink-0 w-full sm:w-64 max-w-[180px] sm:max-w-none overflow-hidden">
                        {song && song.coverArt && credentials?.username && credentials?.password ? (
                            <div className="relative group">
                                <img 
                                    src={`${API_BASE}/rest/getCoverArt.view?id=${encodeURIComponent(song.coverArt)}&size=60&v=1.16.1&c=AudioMuse-AI&u=${encodeURIComponent(credentials.username)}&p=${encodeURIComponent(credentials.password)}`}
                                    alt={song.title}
                                    className="w-10 h-10 sm:w-14 sm:h-14 rounded-lg shadow-lg object-cover"
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
                        ) : (
                            <div className="w-10 h-10 sm:w-14 sm:h-14 bg-dark-700 rounded-lg shadow-lg flex items-center justify-center flex-shrink-0">
                                {/* Generic music icon placeholder */}
                                <svg className="w-5 h-5 sm:w-6 sm:h-6 text-gray-600" fill="currentColor" viewBox="0 0 20 20">
                                    <path d="M18 3a1 1 0 00-1.196-.98l-10 2A1 1 0 006 5v9.114A4.369 4.369 0 005 14c-1.657 0-3 .895-3 2s1.343 2 3 2 3-.895 3-2V7.82l8-1.6v5.894A4.37 4.37 0 0015 12c-1.657 0-3 .895-3 2s1.343 2 3 2 3-.895 3-2V3z" />
                                </svg>
                            </div>
                        )}
                        
                        <div className="overflow-hidden flex-1 min-w-0">
                            {isLoading && !error && (
                                <div className="space-y-2">
                                    <div className="h-4 bg-dark-700 rounded skeleton w-3/4"></div>
                                    <div className="h-3 bg-dark-700 rounded skeleton w-1/2"></div>
                                </div>
                            )}
                            {error && <p className="font-semibold truncate text-sm text-red-400">Error Loading Track</p>}
                            {!isLoading && !error && song && (
                                <>
                                    <p className="font-semibold truncate text-xs sm:text-base text-white">{song.title}</p>
                                    <p className="text-[10px] sm:text-sm text-gray-400 truncate">{song.artist}</p>
                                </>
                            )}
                            {!song && <div className="text-gray-500 text-xs sm:text-sm">Select a song to play</div>}
                        </div>
                    </div>

                    {/* Playback Controls - Centered (Desktop & Mobile) - << Play >> layout */}
                    <div className="flex items-center justify-center gap-1 sm:gap-2 flex-1">
                        {hasQueue && (
                            <button 
                                onClick={onPlayPrevious} 
                                className="text-gray-300 hover:text-white p-2 rounded-full hover:bg-dark-700 transition-all group" 
                                title="Previous"
                            >
                                <svg className="w-4 h-4 sm:w-5 sm:h-5 group-hover:scale-110 transition-transform" fill="currentColor" viewBox="0 0 20 20">
                                    <path d="M8.445 14.832A1 1 0 0010 14v-2.798l5.445 3.63A1 1 0 0017 14V6a1 1 0 00-1.555-.832L10 8.798V6a1 1 0 00-1.555-.832l-6 4a1 1 0 000 1.664l6 4z"></path>
                                </svg>
                            </button>
                        )}
                        <audio
                            ref={audioRef}
                            src={audioSrc || ''}
                            controls
                            preload="metadata"
                            onPlay={setupMediaSession}
                            onEnded={onEnded}
                            onTimeUpdate={(e) => setCurrentTime(e.target.currentTime)}
                            onDurationChange={(e) => setDuration(e.target.duration)}
                            onLoadedData={() => console.log('🎵 Audio loadeddata event - ready to play')}
                            onCanPlay={() => console.log('🎵 Audio canplay event - playback can start')}
                            onSeeking={() => console.log('🎵 Seeking...')}
                            onSeeked={() => console.log('🎵 Seeked - ready to play')}
                            className="w-full sm:max-w-md flex-1 h-8 sm:h-10"
                            style={{ display: song ? 'block' : 'none' }}
                        />
                        {hasQueue && (
                            <button 
                                onClick={onPlayNext} 
                                className="text-gray-300 hover:text-white p-2 rounded-full hover:bg-dark-700 transition-all group" 
                                title="Next"
                            >
                                <svg className="w-4 h-4 sm:w-5 sm:h-5 group-hover:scale-110 transition-transform" fill="currentColor" viewBox="0 0 20 20">
                                    <path d="M10 5.99v8.02a1 1 0 001.555.832l6-4a1 1 0 000-1.664l-6-4A1 1 0 0010 5.99z"></path>
                                    <path d="M3 5.99v8.02a1 1 0 001.555.832l6-4a1 1 0 000-1.664l-6-4A1 1 0 003 5.99z"></path>
                                </svg>
                            </button>
                        )}
                    </div>

                    {/* Queue Button - Desktop & Mobile */}
                    <div className="flex-shrink-0 relative">
                        <button 
                            onClick={onToggleQueueView} 
                            className={`p-1.5 sm:p-3 rounded-lg transition-all ${queueCount > 0 ? 'text-accent-400 hover:bg-accent-500/10' : 'text-gray-400 hover:bg-dark-700'}`}
                            title={`Queue (${queueCount} songs)`}
                        >
                            <svg className="w-4 h-4 sm:w-6 sm:h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M4 6h16M4 12h16M4 18h16"></path>
                            </svg>
                        </button>
                        {queueCount > 0 && (
                            <span className="absolute -top-1 -right-1 bg-accent-500 text-white text-[10px] sm:text-xs font-bold rounded-full min-w-[16px] sm:min-w-[20px] h-4 sm:h-5 flex items-center justify-center px-1 shadow-glow animate-scale-in">
                                {queueCount > 99 ? '99+' : queueCount}
                            </span>
                        )}
                    </div>
                </div>
            </div>
        </div>
    );
}

export default CustomAudioPlayer;
