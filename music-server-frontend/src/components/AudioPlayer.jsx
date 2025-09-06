// Suggested path: music-server-frontend/src/components/AudioPlayer.jsx
import React, { useState, useEffect } from 'react';

function AudioPlayer({ song, onEnded }) {
    const [audioSrc, setAudioSrc] = useState(null);
    const token = localStorage.getItem('token');

    useEffect(() => {
        if (!song) {
            setAudioSrc(null);
            return;
        }

        let objectUrl;
        const fetchAndSetAudio = async () => {
            try {
                // Use the standard Subsonic stream endpoint
                const response = await fetch(`/rest/stream.view?id=${song.id}`, {
                    headers: { 'Authorization': `Bearer ${token}` }
                });
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
    }, [song, token]);

    if (!song) {
        return (
            <div className="fixed bottom-0 left-0 right-0 bg-gray-800 p-4 text-center text-gray-500">
                No song selected.
            </div>
        );
    }

    return (
        <div className="fixed bottom-0 left-0 right-0 bg-gray-800 p-4 shadow-lg border-t border-gray-700 flex items-center space-x-4">
            <div className="flex-shrink-0 w-64">
                <p className="font-bold text-white truncate">{song.title}</p>
                <p className="text-sm text-gray-400 truncate">{song.artist}</p>
            </div>
            <audio key={song.id} controls autoPlay src={audioSrc || ''} onEnded={onEnded} className="w-full">
                Your browser does not support the audio element.
            </audio>
        </div>
    );
}

export default AudioPlayer;

