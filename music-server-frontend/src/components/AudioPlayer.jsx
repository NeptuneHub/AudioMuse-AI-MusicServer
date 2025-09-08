// Suggested path: music-server-frontend/src/components/AudioPlayer.jsx
import React, { useState, useEffect, useRef, useCallback } from 'react';
// To bypass the persistent "Could not resolve" error in your environment,
// this component is being loaded from a reliable CDN. This is a workaround
// for local dependency issues.
import AudioPlayer from 'https://esm.sh/react-h5-audio-player@3.9.1?deps=react@18.2.0';

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

    // Effect to dynamically load the CSS for the audio player
    useEffect(() => {
        const linkId = 'react-h5-audio-player-styles';
        if (!document.getElementById(linkId)) {
            const link = document.createElement('link');
            link.id = linkId;
            link.rel = 'stylesheet';
            link.href = 'https://cdn.jsdelivr.net/npm/react-h5-audio-player@3.9.1/lib/styles.css';
            document.head.appendChild(link);
        }
    }, []);

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

        fetchAndSetAudio();

        return () => {
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
        <div className="fixed bottom-0 left-0 right-0 bg-gray-800 shadow-lg border-t border-gray-700 z-50 player-container h-28 flex items-center">
            <style>{`
                .player-container .rhap_container {
                    background-color: transparent;
                    box-shadow: none;
                    width: 100%;
                    padding: 0 1rem;
                }
                .player-container .rhap_main {
                    flex-direction: column;
                    justify-content: center;
                }
                @media (min-width: 640px) {
                    .player-container .rhap_main {
                       flex-direction: row;
                       align-items: center;
                    }
                    .player-container .rhap_controls-section {
                        flex: 1 1 auto;
                    }
                }
                .player-container .rhap_progress-indicator, .player-container .rhap_volume-indicator {
                    background: #14b8a6; /* teal-500 */
                }
                .player-container .rhap_progress-filled, .player-container .rhap_volume-bar {
                    background-color: #0d9488; /* teal-600 */
                }
                .player-container .rhap_time, .rhap_current-time, .rhap_total-time {
                    color: #9ca3af; /* gray-400 */
                }
                .player-container svg { color: #fff; }

                /* Mobile specific tweaks for a more compact player */
                @media (max-width: 639px) {
                    .player-container .rhap_volume-controls, .player-container .rhap_loop-controls, .rhap_shuffle-controls {
                        display: none;
                    }
                    .player-container .rhap_main-controls {
                         padding: 0;
                    }
                     .player-container .rhap_additional-controls {
                        flex-grow: 0;
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
                        <div className="text-white text-center w-full sm:w-auto sm:min-w-[150px] md:min-w-[250px] sm:mr-4">
                           {isLoading ? (
                                <p className="font-bold truncate text-sm text-gray-400">Loading...</p>
                            ) : error ? (
                                <p className="font-bold truncate text-sm text-red-500">Error Loading Track</p>
                            ) : (
                                <>
                                    <p className="font-bold truncate text-sm">{song.title}</p>
                                    <p className="text-sm text-gray-400 truncate">{song.artist}</p>
                                </>
                            )}
                        </div>
                    }
                    showJumpControls={true}
                    layout="horizontal-reverse"
                     customAdditionalControls={
                        [
                            <button key="queue-button" onClick={onToggleQueueView} className="text-white p-2 rounded-full hover:bg-gray-700" title="Show queue">
                                <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M4 6h16M4 12h16M4 18h16"></path></svg>
                            </button>
                        ]
                    }
                 />
            )}
        </div>
    );
}

export default CustomAudioPlayer;

