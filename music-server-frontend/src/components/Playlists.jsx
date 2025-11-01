// Suggested path: music-server-frontend/src/components/Playlists.jsx
import React, { useState, useEffect, useCallback, useRef } from 'react';
import { subsonicFetch } from '../api';

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


function Playlists({ credentials, isAdmin, onNavigate }) {
    const [allPlaylists, setAllPlaylists] = useState([]);
    const [playlists, setPlaylists] = useState([]);
    const [error, setError] = useState('');
    const [isLoading, setIsLoading] = useState(false);
    const [hasMore, setHasMore] = useState(true);
    const [isCreating, setIsCreating] = useState(false);
    const [successMessage, setSuccessMessage] = useState('');
    const [isGenerating, setIsGenerating] = useState(false);
    const [playlistToDelete, setPlaylistToDelete] = useState(null);
    
    // use the shared subsonicFetch helper imported from ../api

    const fetchPlaylists = useCallback(async () => {
        setIsLoading(true);
        try {
            const data = await subsonicFetch('getPlaylists.view');
            const playlistData = data.playlists?.playlist || [];
            const fullList = Array.isArray(playlistData) ? playlistData : [playlistData];
            setAllPlaylists(fullList);
            setPlaylists(fullList.slice(0, 50));
            setHasMore(fullList.length > 50);
        } catch (err) {
            setError(err.message);
        } finally {
            setIsLoading(false);
        }
    }, []);

    useEffect(() => {
        fetchPlaylists();
    }, [fetchPlaylists]);

    const loadMore = useCallback(() => {
        if (!hasMore) return;
        const newVisibleCount = playlists.length + 50;
        setPlaylists(allPlaylists.slice(0, newVisibleCount));
        setHasMore(newVisibleCount < allPlaylists.length);
    }, [hasMore, playlists.length, allPlaylists]);

    const observer = useRef();
    const lastPlaylistElementRef = useCallback(node => {
        if (observer.current) observer.current.disconnect();
        observer.current = new IntersectionObserver(entries => {
            if (entries[0].isIntersecting && hasMore) {
                loadMore();
            }
        });
        if (node) observer.current.observe(node);
    }, [hasMore, loadMore]);

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
            const fingerprintData = await subsonicFetch('getSonicFingerprint.view');
            const songs = fingerprintData.directory?.song ? (Array.isArray(fingerprintData.directory.song) ? fingerprintData.directory.song : [fingerprintData.directory.song]) : [];

            if (songs.length === 0) {
                setSuccessMessage('Sonic Fingerprint generated an empty playlist.');
                setIsGenerating(false);
                return;
            }

            const playlistName = `Sonic Fingerprint ${new Date().toLocaleDateString()}`;
            await subsonicFetch('createPlaylist.view', { name: playlistName });
            
            const playlistsData = await subsonicFetch('getPlaylists.view');
            const allPlaylistsData = playlistsData.playlists?.playlist || [];
            const newPlaylist = (Array.isArray(allPlaylistsData) ? allPlaylistsData : [allPlaylistsData]).find(p => p.name === playlistName);

            if (!newPlaylist) throw new Error("Could not find the newly created playlist.");

            setSuccessMessage(`Created playlist "${playlistName}". Now adding ${songs.length} songs...`);
            for (const song of songs) {
                await subsonicFetch('updatePlaylist.view', { playlistId: newPlaylist.id, songIdToAdd: song.id });
            }

            setSuccessMessage(`Successfully created "${playlistName}" with ${songs.length} songs!`);
            fetchPlaylists();

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
            
            {playlists.length === 0 && !isLoading ? (
                <div className="flex flex-col items-center justify-center py-16 text-center">
                    <svg className="w-24 h-24 text-gray-600 mb-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M9 19V6l12-3v13M9 19c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zm12-3c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zM9 10l12-3"></path>
                    </svg>
                    <h3 className="text-xl font-semibold text-gray-400 mb-2">No Playlists Yet</h3>
                    <p className="text-gray-500 mb-6">Create your first playlist or generate a Sonic Fingerprint!</p>
                    <div className="flex gap-3">
                        <button onClick={() => setIsCreating(true)} className="bg-indigo-600 hover:bg-indigo-700 text-white font-bold py-2 px-6 rounded">
                            Create Playlist
                        </button>
                        <button onClick={handleCreateSonicFingerprintPlaylist} disabled={isGenerating} className="bg-teal-600 hover:bg-teal-700 text-white font-bold py-2 px-6 rounded disabled:bg-gray-500">
                            {isGenerating ? 'Generating...' : 'Sonic Fingerprint'}
                        </button>
                    </div>
                </div>
            ) : (
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                    {playlists.map((p, index) => (
                    <div 
                        ref={index === playlists.length - 1 ? lastPlaylistElementRef : null}
                        key={p.id} 
                        className="bg-gray-800 rounded-lg p-4 flex flex-col justify-between"
                    >
                        <div>
                            <h3 className="font-bold text-lg truncate hover:text-teal-400 cursor-pointer" onClick={() => onNavigate({ page: 'songs', title: p.name, filter: { playlistId: p.id } })}>
                                {p.name}
                            </h3>
                            <p className="text-sm text-gray-400">{p.songCount} songs</p>
                        </div>
                                <div className="mt-4 flex justify-end space-x-2">
                                    {/**
                                     * Delete button should only be visible when:
                                     * - The current user is the playlist owner (credentials.username === p.owner)
                                     * - OR the playlist was created by an admin (p.public === true) and the current user is an admin
                                     *
                                     * Backend sets `owner` to the creator username and marks admin-created playlists as public=true.
                                     */}
                                    {((p.owner && credentials.username === p.owner) || (p.public && isAdmin)) && (
                                        <button onClick={() => setPlaylistToDelete({ id: p.id, name: p.name })} className="text-xs font-medium text-red-500 hover:underline">
                                            Delete
                                        </button>
                                    )}
                                </div>
                    </div>
                ))}
                </div>
            )}
            {isLoading && allPlaylists.length === 0 && <p className="text-center text-gray-400 mt-4">Loading playlists...</p>}
            {!hasMore && playlists.length > 0 && <p className="text-center text-gray-500 mt-4">End of list.</p>}
        </div>
    );
}

export default Playlists;
