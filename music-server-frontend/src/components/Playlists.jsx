// Suggested path: music-server-frontend/src/components/Playlists.jsx
import React, { useState, useEffect, useCallback } from 'react';

const Modal = ({ children, onClose }) => (
    <div className="fixed inset-0 bg-black bg-opacity-60 flex items-center justify-center z-50">
        <div className="bg-gray-800 p-6 rounded-lg shadow-xl w-full max-w-md relative">
            <button onClick={onClose} className="absolute top-2 right-2 text-gray-400 hover:text-white">
                <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12"></path></svg>
            </button>
            {children}
        </div>
    </div>
);

const CreatePlaylistModal = ({ onClose, onSubmit }) => {
    const [name, setName] = useState('');
    const handleSubmit = (e) => {
        e.preventDefault();
        onSubmit(name);
    };

    return (
        <Modal onClose={onClose}>
            <h3 className="text-xl font-bold mb-4 text-teal-400">Create New Playlist</h3>
            <form onSubmit={handleSubmit}>
                <div className="mb-4">
                    <label className="block text-gray-400 mb-2">Playlist Name</label>
                    <input type="text" value={name} onChange={e => setName(e.target.value)} className="w-full p-2 bg-gray-700 rounded" required/>
                </div>
                <div className="flex justify-end space-x-4">
                    <button type="button" onClick={onClose} className="bg-gray-600 hover:bg-gray-700 text-white font-bold py-2 px-4 rounded">Cancel</button>
                    <button type="submit" className="bg-teal-500 hover:bg-teal-600 text-white font-bold py-2 px-4 rounded">Create</button>
                </div>
            </form>
        </Modal>
    );
};


function Playlists({ credentials, onNavigate }) {
    const [playlists, setPlaylists] = useState([]);
    const [error, setError] = useState('');
    const [isCreating, setIsCreating] = useState(false);
    const [successMessage, setSuccessMessage] = useState('');
    
    const subsonicFetch = useCallback(async (endpoint, params = {}) => {
        const allParams = new URLSearchParams({
            u: credentials.username, p: credentials.password, v: '1.16.1', c: 'AudioMuse-AI', f: 'json', ...params
        });
        const response = await fetch(`/rest/${endpoint}?${allParams.toString()}`);
        const data = await response.json();
        const subsonicResponse = data['subsonic-response'];
        if (subsonicResponse.status === 'failed') {
            throw new Error(subsonicResponse.error.message);
        }
        return subsonicResponse;
    }, [credentials]);

    const fetchPlaylists = useCallback(async () => {
        try {
            const data = await subsonicFetch('getPlaylists.view');
            const playlistData = data.playlists?.playlist || [];
            setPlaylists(Array.isArray(playlistData) ? playlistData : [playlistData]);
        } catch (err) {
            setError(err.message);
        }
    }, [subsonicFetch]);

    useEffect(() => {
        fetchPlaylists();
    }, [fetchPlaylists]);

    const handleCreatePlaylist = async (name) => {
        setError('');
        setSuccessMessage('');
        try {
            await subsonicFetch('createPlaylist.view', { name });
            setSuccessMessage(`Playlist "${name}" created successfully!`);
            setIsCreating(false);
            fetchPlaylists();
        } catch (err) {
            setError(err.message);
        }
    };
    
    const handleDeletePlaylist = async (playlistId, playlistName) => {
        if (window.confirm(`Are you sure you want to delete the playlist "${playlistName}"?`)) {
            setError('');
            setSuccessMessage('');
            try {
                await subsonicFetch('deletePlaylist.view', { id: playlistId });
                setSuccessMessage(`Playlist "${playlistName}" deleted.`);
                fetchPlaylists();
            } catch (err) {
                setError(err.message);
            }
        }
    };

    return (
        <div>
            <div className="flex justify-between items-center mb-6">
                <h2 className="text-3xl font-bold">Your Playlists</h2>
                <button onClick={() => setIsCreating(true)} className="bg-indigo-600 hover:bg-indigo-700 text-white font-bold py-2 px-4 rounded">
                    Create Playlist
                </button>
            </div>

            {error && <p className="text-red-500 mb-4 p-3 bg-red-900/50 rounded">{error}</p>}
            {successMessage && <p className="text-green-400 mb-4 p-3 bg-green-900/50 rounded">{successMessage}</p>}

            {isCreating && <CreatePlaylistModal onClose={() => setIsCreating(false)} onSubmit={handleCreatePlaylist} />}
            
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                {playlists.map(p => (
                    <div key={p.id} className="bg-gray-800 rounded-lg p-4 flex flex-col justify-between">
                        <div>
                            <h3 className="font-bold text-lg truncate hover:text-teal-400 cursor-pointer" onClick={() => onNavigate({ page: 'songs', title: p.name, filter: { playlistId: p.id } })}>
                                {p.name}
                            </h3>
                            <p className="text-sm text-gray-400">{p.songCount} songs</p>
                        </div>
                        <div className="mt-4 flex justify-end space-x-2">
                             <button onClick={() => handleDeletePlaylist(p.id, p.name)} className="text-xs font-medium text-red-500 hover:underline">
                                Delete
                            </button>
                        </div>
                    </div>
                ))}
            </div>
        </div>
    );
}

export default Playlists;
