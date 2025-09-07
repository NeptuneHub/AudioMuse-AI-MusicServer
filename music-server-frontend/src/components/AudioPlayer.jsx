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

    useEffect(() => {
        if (!song) {
            setAudioSrc(null);
            return;
        }

        let objectUrl;
        const fetchAndSetAudio = async () => {
            try {
                const response = await subsonicFetch('stream.view', credentials, { id: song.id });
                if (!response.ok) throw new Error('Failed to fetch song');
                const blob = await response.blob();
                objectUrl = URL.createObjectURL(blob);
                setAudioSrc(objectUrl);
            } catch (error) {
                console.error("Error streaming song:", error);
                setAudioSrc(null);
            }
        };

        fetchAndSetAudio();
        return () => { // Cleanup function
            if (objectUrl) URL.revokeObjectURL(objectUrl);
        };
    }, [song, credentials]);

    if (!song) {
        return null; // Don't render the player if no song is selected
    }

    return (
        <div className="fixed bottom-0 left-0 right-0 bg-gray-800 p-4 shadow-lg border-t border-gray-700 flex items-center space-x-4 z-50">
            <div className="flex-shrink-0 w-64">
                <p className="font-bold text-white truncate">{song.title}</p>
                <p className="text-sm text-gray-400 truncate">{song.artist}</p>
            </div>
            <audio key={song.id} controls autoPlay src={audioSrc || ''} onEnded={onEnded} className="w-full" />
        </div>
    );
}

export default AudioPlayer;