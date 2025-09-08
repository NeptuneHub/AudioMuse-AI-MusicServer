import React, { useState, useEffect } from 'react';

const subsonicFetch = async (endpoint, creds, params = {}) => {
    const allParams = new URLSearchParams({
        u: creds.username, p: creds.password, v: '1.16.1', c: 'AudioMuse-AI', ...params
    });
    const response = await fetch(`/rest/${endpoint}?${allParams.toString()}`);
    return response;
};

function AudioPlayer({ song, onEnded, credentials }) {
    const [audioSrc, setAudioSrc] = useState(null);
    const [error, setError] = useState(false);

    useEffect(() => {
        if (!song) {
            setAudioSrc(null);
            setError(false);
            return;
        }

        // When a new song is selected, reset state to show loading indicator
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
                setError(true); // Set a persistent error state if fetch fails
                setAudioSrc(null);
            }
        };

        fetchAndSetAudio();

        return () => { // Cleanup function for the previous song
            if (objectUrl) {
                URL.revokeObjectURL(objectUrl);
            }
        };
    }, [song, credentials]);

    if (!song) {
        return null; // Don't render the player if no song is selected
    }

    return (
        <div className="fixed bottom-0 left-0 right-0 bg-gray-800 p-2 sm:p-4 shadow-lg border-t border-gray-700 flex items-center space-x-2 sm:space-x-4 z-50">
            <div className="flex-shrink-0 w-24 sm:w-40 md:w-64">
                {error ? (
                     <p className="font-bold text-red-500 truncate text-sm sm:text-base">Error Loading</p>
                ) : (
                    <p className="font-bold text-white truncate text-sm sm:text-base">{audioSrc ? song.title : 'Loading...'}</p>
                )}
                <p className="text-xs sm:text-sm text-gray-400 truncate">{song.artist}</p>
            </div>
            
            {/* Only render the audio element when we have a valid source. This prevents the browser's native player from showing an error state. */}
            {audioSrc && !error && (
                 <audio 
                    key={song.id} 
                    controls 
                    autoPlay 
                    src={audioSrc} 
                    onEnded={onEnded} 
                    className="w-full" 
                />
            )}
        </div>
    );
}

export default AudioPlayer;

