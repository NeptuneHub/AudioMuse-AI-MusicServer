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

const ConfirmationModal = ({ onClose, onConfirm, title, message }) => (
    <Modal onClose={onClose}>
        <h3 className="text-xl font-bold mb-4 text-yellow-400">{title}</h3>
        <p className="text-gray-300 mb-6">{message}</p>
        <div className="flex justify-end space-x-4">
            <button type="button" onClick={onClose} className="bg-gray-600 hover:bg-gray-700 text-white font-bold py-2 px-4 rounded">Cancel</button>
            <button type="button" onClick={onConfirm} className="bg-red-600 hover:bg-red-700 text-white font-bold py-2 px-4 rounded">Confirm Delete</button>
        </div>
    </Modal>
);


function Playlists({ credentials, onNavigate }) {
    const [playlists, setPlaylists] = useState([]);
    const [error, setError] = useState('');
    const [isCreating, setIsCreating] = useState(false);
    const [successMessage, setSuccessMessage] = useState('');
    const [isGenerating, setIsGenerating] = useState(false);
    const [playlistToDelete, setPlaylistToDelete] = useState(null);
    
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
    
    const confirmDeletePlaylist = async () => {
        if (!playlistToDelete) return;
        
        setError('');
        setSuccessMessage('');
        try {
            await subsonicFetch('deletePlaylist.view', { id: playlistToDelete.id });
            setSuccessMessage(`Playlist "${playlistToDelete.name}" deleted.`);
            fetchPlaylists();
        } catch (err) {
            setError(err.message);
        } finally {
            setPlaylistToDelete(null);
        }
    };

    const handleCreateSonicFingerprintPlaylist = async () => {
        setError('');
        setSuccessMessage('Generating Sonic Fingerprint...');
        setIsGenerating(true);
        try {
            // 1. Get the fingerprint songs
            const fingerprintData = await subsonicFetch('getSonicFingerprint.view');
            const songs = fingerprintData.directory?.song ? (Array.isArray(fingerprintData.directory.song) ? fingerprintData.directory.song : [fingerprintData.directory.song]) : [];

            if (songs.length === 0) {
                setSuccessMessage('Sonic Fingerprint generated an empty playlist.');
                setIsGenerating(false);
                return;
            }

            // 2. Create a new playlist with a unique name
            const playlistName = `Sonic Fingerprint ${new Date().toLocaleDateString()}`;
            await subsonicFetch('createPlaylist.view', { name: playlistName });
            
            // 3. Find the newly created playlist to get its ID
            const playlistsData = await subsonicFetch('getPlaylists.view');
            const allPlaylists = playlistsData.playlists?.playlist || [];
            const newPlaylist = (Array.isArray(allPlaylists) ? allPlaylists : [allPlaylists]).find(p => p.name === playlistName);

            if (!newPlaylist) {
                throw new Error("Could not find the newly created playlist to add songs to it.");
            }

            // 4. Add songs to the new playlist
            setSuccessMessage(`Created playlist "${playlistName}". Now adding ${songs.length} songs...`);
            for (const song of songs) {
                await subsonicFetch('updatePlaylist.view', { playlistId: newPlaylist.id, songIdToAdd: song.id });
            }

            setSuccessMessage(`Successfully created "${playlistName}" with ${songs.length} songs!`);
            fetchPlaylists(); // Refresh the list

        } catch (err) {
            setError(err.message || 'Failed to create Sonic Fingerprint playlist.');
            setSuccessMessage('');
        } finally {
            setIsGenerating(false);
        }
    };


    return (
        <div>
            <div className="flex justify-between items-center mb-6">
                <h2 className="text-3xl font-bold text-white">Your Playlists</h2>
                <div className="flex items-center space-x-2">
                    <button onClick={handleCreateSonicFingerprintPlaylist} disabled={isGenerating} className="bg-teal-600 hover:bg-teal-700 text-white font-bold py-2 px-4 rounded disabled:bg-gray-500">
                        {isGenerating ? 'Generating...' : 'Create Sonic Fingerprint Playlist'}
                    </button>
                    <button onClick={() => setIsCreating(true)} className="bg-indigo-600 hover:bg-indigo-700 text-white font-bold py-2 px-4 rounded">
                        Create Playlist
                    </button>
                </div>
            </div>

            {error && <p className="text-red-500 mb-4 p-3 bg-red-900/50 rounded">{error}</p>}
            {successMessage && <p className="text-green-400 mb-4 p-3 bg-green-900/50 rounded">{successMessage}</p>}

            {isCreating && <CreatePlaylistModal onClose={() => setIsCreating(false)} onSubmit={handleCreatePlaylist} />}
            
            {playlistToDelete && (
                <ConfirmationModal
                    onClose={() => setPlaylistToDelete(null)}
                    onConfirm={confirmDeletePlaylist}
                    title="Confirm Deletion"
                    message={`Are you sure you want to delete the playlist "${playlistToDelete.name}"?`}
                />
            )}
            
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
                             <button onClick={() => setPlaylistToDelete({ id: p.id, name: p.name })} className="text-xs font-medium text-red-500 hover:underline">
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

