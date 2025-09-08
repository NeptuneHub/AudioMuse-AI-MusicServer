// Suggested path: music-server-frontend/src/components/AudioPlayer.jsx
import React, { useState, useEffect, useRef, useCallback } from 'react';

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
    const audioRef = useRef(null);

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
            audioRef.current.play().catch(e => console.error("Autoplay was prevented:", e));
        }
    }, [audioSrc]);

    return (
        <div className="fixed bottom-0 left-0 right-0 bg-gray-800 shadow-lg border-t border-gray-700 z-50 p-2 sm:p-3">
            <div className="container mx-auto flex items-center justify-between gap-2 sm:gap-4">
                <div className="flex-shrink-0 w-1/3 sm:w-1/4 overflow-hidden">
                    {isLoading && !error && <p className="font-semibold truncate text-sm text-gray-400">Loading...</p>}
                    {error && <p className="font-semibold truncate text-sm text-red-500">Error Loading Track</p>}
                    {!isLoading && !error && song && (
                        <>
                            <p className="font-semibold truncate text-sm text-white">{song.title}</p>
                            <p className="text-xs text-gray-400 truncate">{song.artist}</p>
                        </>
                    )}
                    {!song && <div className="text-gray-500 text-sm">Select a song</div>}
                </div>

                <div className="flex-grow flex items-center justify-center gap-2">
                    {hasQueue && (
                        <button onClick={onPlayPrevious} className="text-white p-2 rounded-full hover:bg-gray-700" title="Previous">
                            <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 20 20"><path d="M8.445 14.832A1 1 0 0010 14.032V5.968a1 1 0 00-1.555-.832L4.12 9.168a1 1 0 000 1.664l4.325 4.001zM11.555 4.168a1 1 0 011.555.832v8.064a1 1 0 01-1.555.832l-4.325-4.001a1 1 0 010-1.664l4.325-4.001z"></path></svg>
                        </button>
                    )}
                    <audio
                        ref={audioRef}
                        src={audioSrc || ''}
                        controls
                        onPlay={setupMediaSession}
                        onEnded={onEnded}
                        className="w-full max-w-md"
                        style={{ display: song ? 'block' : 'none' }}
                    />
                    {hasQueue && (
                        <button onClick={onPlayNext} className="text-white p-2 rounded-full hover:bg-gray-700" title="Next">
                            <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 20 20"><path d="M11.555 5.168A1 1 0 0010 5.968v8.064a1 1 0 001.555.832l4.325-4.001a1 1 0 000-1.664l-4.325-4.001zM8.445 15.832a1 1 0 01-1.555-.832V5.001a1 1 0 011.555-.832l4.325 4.001a1 1 0 010 1.664l-4.325 4.001z"></path></svg>
                        </button>
                    )}
                </div>

                <div className="flex-shrink-0">
                    <button onClick={onToggleQueueView} className="text-white p-2 rounded-full hover:bg-gray-700" title="Show queue">
                        <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M4 6h16M4 12h16M4 18h16"></path></svg>
                    </button>
                </div>
            </div>
        </div>
    );
}

export default CustomAudioPlayer;
