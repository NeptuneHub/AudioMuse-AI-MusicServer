// Suggested path: music-server-frontend/src/components/AudioPlayer.jsx
import React, { useState, useEffect, useRef, useCallback } from 'react';
// This is the correct import. The error indicates a build environment issue.
// Please run `npm install` in your terminal to fix the "Could not resolve" error.
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
    const [error, setError] = useState(false);
    const playerRef = useRef(null);

    // Effect for fetching audio data
    useEffect(() => {
        if (!song) {
            setAudioSrc(null);
            setError(false);
            return;
        }

        setAudioSrc(null);
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
            } catch (err) {
                console.error("Error streaming song:", err);
                setError(true);
                setAudioSrc(null);
            }
        };

        fetchAndSetAudio();

        return () => {
            if (objectUrl) {
                URL.revokeObjectURL(objectUrl);
            }
        };
    }, [song, credentials]);

    // Function to setup Media Session API, wrapped in useCallback to fix dependency warning.
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

            // Set action handlers
            navigator.mediaSession.setActionHandler('play', () => {
                if (playerRef.current?.audio.current) playerRef.current.audio.current.play();
            });
            navigator.mediaSession.setActionHandler('pause', () => {
                 if (playerRef.current?.audio.current) playerRef.current.audio.current.pause();
            });
            navigator.mediaSession.setActionHandler('nexttrack', hasQueue ? onPlayNext : null);
            navigator.mediaSession.setActionHandler('previoustrack', hasQueue ? onPlayPrevious : null);
        }
    }, [song, credentials, hasQueue, onPlayNext, onPlayPrevious]);

    // Effect to update metadata when song changes
    useEffect(() => {
        // We update the metadata here, but the action handlers are set on play
        setupMediaSession();
    }, [song, setupMediaSession]);

    if (!song) {
        return null;
    }
    
    return (
        <div className="fixed bottom-0 left-0 right-0 bg-gray-800 p-2 sm:p-4 shadow-lg border-t border-gray-700 z-50 player-container">
            <style>{`
                /* Custom styles to make the player theme match the app */
                .player-container .rhap_container {
                    background-color: transparent;
                    box-shadow: none;
                    padding: 0;
                }
                .player-container .rhap_main-controls {
                    flex: 1 1 auto;
                    justify-content: center;
                }
                .player-container .rhap_additional-controls {
                     flex: 0 0 auto;
                }
                .player-container .rhap_progress-indicator, .player-container .rhap_volume-indicator {
                    background: #14b8a6; /* teal-500 */
                }
                 .player-container .rhap_progress-filled {
                    background-color: #0d9488; /* teal-600 */
                }
                .player-container .rhap_time, .rhap_current-time, .rhap_total-time {
                    color: #9ca3af; /* gray-400 */
                }
                 .player-container svg {
                    color: #fff;
                }
            `}</style>
             <AudioPlayer
                // key={song.id} // REMOVED: This was causing the player to remount on every song change.
                ref={playerRef}
                autoPlay
                src={audioSrc}
                onPlay={setupMediaSession} // Set handlers on play to satisfy mobile browser rules
                onEnded={onEnded}
                showSkipControls={hasQueue}
                onClickNext={onPlayNext}
                onClickPrevious={onPlayPrevious}
                header={
                    <div className="text-white text-center pb-2 px-12">
                        <p className="font-bold truncate">{song.title}</p>
                        <p className="text-sm text-gray-400 truncate">{song.artist}</p>
                    </div>
                }
                showJumpControls={true} // FIX: Re-enabled jump controls
                layout="stacked" // FIX: Changed layout for better consistency
                customAdditionalControls={ // FIX: Moved queue button here to avoid replacing main controls
                    [
                        <button key="queue-button" onClick={onToggleQueueView} className="text-white p-2 rounded-full hover:bg-gray-700" title="Show queue">
                            <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M4 6h16M4 12h16M4 18h16"></path></svg>
                        </button>
                    ]
                }
             />
             {error && <div className="absolute inset-0 bg-gray-800 flex items-center justify-center text-red-500">Error Loading Track</div>}
             {!audioSrc && !error && <div className="absolute inset-0 bg-gray-800 flex items-center justify-center text-gray-400">Loading...</div>}
        </div>
    );
}

export default CustomAudioPlayer;

