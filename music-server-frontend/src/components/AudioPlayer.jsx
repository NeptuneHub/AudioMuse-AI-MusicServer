// Suggested path: music-server-frontend/src/components/AudioPlayer.jsx
import React, { useState, useEffect, useRef, useCallback } from 'react';
import AudioPlayer from 'react-h5-audio-player';
import 'react-h5-audio-player/lib/styles.css';

const subsonicFetch = async (endpoint, creds, params = {}) => {
    const allParams = new URLSearchParams({
        u: creds.username, p: creds.password, v: '1.16.1', c: 'AudioMuse-AI', ...params
    });
    const response = await fetch(`/rest/${endpoint}?${allParams.toString()}`);
    return response;
};

function CustomAudioPlayer({ song, onEnded, credentials, onPlayNext, onPlayPrevious, hasQueue, onToggleQueueView }) {
    const [audioSrc, setAudioSrc] = useState(null);
    const [isLoading, setIsLoading] = useState(false);
    const [error, setError] = useState(false);
    const playerRef = useRef(null);

    // Effect for fetching audio data
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
                const response = await subsonicFetch('stream.view', credentials, { id: song.id });
                if (!response.ok) {
                    throw new Error(`Failed to fetch song: ${response.statusText}`);
                }
                const blob = await response.blob();
                objectUrl = URL.createObjectURL(blob);
                setAudioSrc(objectUrl);
                setError(false);
            } catch (err) {
                console.error("Error streaming song:", err);
                setError(true);
                setAudioSrc(null);
            } finally {
                setIsLoading(false);
            }
        };

        // Adding a small delay to allow the UI to update before fetching
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
            navigator.mediaSession.metadata = new window.MediaMetadata({
                title: song.title,
                artist: song.artist,
                album: song.album,
                artwork: [
                    { 
                        src: `/rest/getCoverArt.view?id=${song.coverArt}&u=${credentials.username}&p=${credentials.password}&v=1.16.1&c=AudioMuse-AI`, 
                        sizes: '300x300', 
                        type: 'image/png' 
                    },
                ]
            });

            navigator.mediaSession.setActionHandler('play', () => playerRef.current?.audio.current.play());
            navigator.mediaSession.setActionHandler('pause', () => playerRef.current?.audio.current.pause());
            navigator.mediaSession.setActionHandler('nexttrack', hasQueue ? onPlayNext : null);
            navigator.mediaSession.setActionHandler('previoustrack', hasQueue ? onPlayPrevious : null);
        }
    }, [song, credentials, hasQueue, onPlayNext, onPlayPrevious]);

    useEffect(() => {
        setupMediaSession();
    }, [song, setupMediaSession]);
    
    return (
        <div className="fixed bottom-0 left-0 right-0 bg-gray-800 shadow-lg border-t border-gray-700 z-50 player-container h-20 sm:h-24 flex items-center">
            <style>{`
                .player-container .rhap_container {
                    background-color: transparent;
                    box-shadow: none;
                    width: 100%;
                    padding: 0 0.5rem;
                }
                .player-container .rhap_progress-indicator, .player-container .rhap_volume-indicator {
                    background: #14b8a6;
                }
                .player-container .rhap_progress-filled, .player-container .rhap_volume-bar {
                    background-color: #0d9488;
                }
                .player-container .rhap_time { color: #9ca3af; }
                .player-container svg { color: #fff; }

                /* --- Mobile Layout --- */
                @media (max-width: 639px) {
                    .player-container .rhap_main {
                        display: grid;
                        grid-template-columns: 1fr auto 1fr;
                        align-items: center;
                    }
                    .player-container .rhap_main-controls { grid-column: 2 / 3; }
                    .player-container .rhap_additional-controls { grid-column: 3 / 4; justify-self: end; }
                    .player-container .rhap_header {
                        grid-column: 1 / 4;
                        grid-row: 1 / 2;
                        text-align: center;
                        padding-bottom: 0.25rem;
                    }
                    .player-container .rhap_progress-section { display: none; }
                    .player-container .rhap_volume-controls { display: none; }
                }

                /* --- Desktop Layout --- */
                @media (min-width: 640px) {
                    .player-container .rhap_container { padding: 0 1.5rem; }
                    .player-container .rhap_main {
                       display: flex;
                       align-items: center;
                    }
                    .player-container .rhap_header {
                        min-width: 200px;
                        max-width: 300px;
                        margin-right: 1.5rem;
                    }
                    .player-container .rhap_controls-section {
                        flex: 1 1 auto;
                    }
                }
            `}</style>
            
            {!song && (
                 <div className="w-full text-center text-gray-500 px-4">Select a song to play</div>
            )}
            
            {song && (
                 <AudioPlayer
                    ref={playerRef}
                    autoPlay
                    src={audioSrc}
                    onPlay={setupMediaSession}
                    onEnded={onEnded}
                    showSkipControls={hasQueue}
                    onClickNext={onPlayNext}
                    onClickPrevious={onPlayPrevious}
                    header={
                        <div className="text-white text-center sm:text-left overflow-hidden">
                           {isLoading && !error && (
                                <p className="font-semibold truncate text-sm text-gray-400">Loading...</p>
                            )}
                            {error && (
                                <p className="font-semibold truncate text-sm text-red-500">Error Loading Track</p>
                            )}
                            {!isLoading && !error && (
                                <>
                                    <p className="font-semibold truncate text-sm">{song.title}</p>
                                    <p className="text-xs text-gray-400 truncate">{song.artist}</p>
                                </>
                            )}
                        </div>
                    }
                    layout="horizontal-reverse"
                    showJumpControls={false}
                     customAdditionalControls={
                        [
                            <button key="queue-button" onClick={onToggleQueueView} className="text-white p-2 rounded-full hover:bg-gray-700" title="Show queue">
                                <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M4 6h16M4 12h16M4 18h16"></path></svg>
                            </button>
                        ]
                    }
                 />
            )}
        </div>
    );
}

export default CustomAudioPlayer;

