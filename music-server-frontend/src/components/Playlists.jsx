// Suggested path: music-server-frontend/src/components/Playlists.jsx
import React, { useState, useEffect, useCallback, useRef } from 'react';
import { subsonicFetch } from '../api';

const Modal = ({ children, onClose }) => (
    <div className="fixed inset-0 bg-black bg-opacity-70 backdrop-blur-sm flex items-center justify-center z-50 p-4 animate-fade-in">
        <div className="glass rounded-2xl shadow-2xl w-full max-w-md relative animate-scale-in">
            <button onClick={onClose} className="absolute top-4 right-4 text-gray-400 hover:text-white transition-colors p-1 rounded-lg hover:bg-dark-700">
                <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12"></path>
                </svg>
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
            <div className="p-6 sm:p-8">
                <h3 className="text-2xl font-bold mb-6 bg-gradient-to-r from-accent-400 to-accent-600 bg-clip-text text-transparent">Create New Playlist</h3>
                <form onSubmit={handleSubmit}>
                    <div className="mb-6">
                        <label className="block text-gray-300 mb-2 font-medium">Playlist Name</label>
                        <input 
                            type="text" 
                            value={name} 
                            onChange={e => setName(e.target.value)} 
                            className="w-full p-3 bg-dark-700 rounded-lg border border-dark-600 focus:outline-none focus:border-accent-500 focus:ring-2 focus:ring-accent-500/20 text-white transition-all" 
                            placeholder="My Awesome Playlist"
                            required
                        />
                    </div>
                    <div className="flex justify-end gap-3">
                        <button 
                            type="button" 
                            onClick={onClose} 
                            className="px-5 py-2.5 rounded-lg bg-dark-700 hover:bg-dark-600 text-white font-semibold transition-all"
                        >
                            Cancel
                        </button>
                        <button 
                            type="submit" 
                            className="px-5 py-2.5 rounded-lg bg-gradient-accent text-white font-semibold shadow-lg hover:shadow-glow transition-all"
                        >
                            Create
                        </button>
                    </div>
                </form>
            </div>
        </Modal>
    );
};

const ConfirmationModal = ({ onClose, onConfirm, title, message }) => (
    <Modal onClose={onClose}>
        <div className="p-6 sm:p-8">
            <div className="flex items-center gap-3 mb-4">
                <div className="bg-red-500/20 rounded-full p-3">
                    <svg className="w-6 h-6 text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"></path>
                    </svg>
                </div>
                <h3 className="text-xl font-bold text-white">{title}</h3>
            </div>
            <p className="text-gray-300 mb-6 ml-14">{message}</p>
            <div className="flex justify-end gap-3">
                <button 
                    type="button" 
                    onClick={onClose} 
                    className="px-5 py-2.5 rounded-lg bg-dark-700 hover:bg-dark-600 text-white font-semibold transition-all"
                >
                    Cancel
                </button>
                <button 
                    type="button" 
                    onClick={onConfirm} 
                    className="px-5 py-2.5 rounded-lg bg-red-600 hover:bg-red-700 text-white font-semibold shadow-lg transition-all"
                >
                    Delete
                </button>
            </div>
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
            {/* Action Buttons */}
            <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4 mb-6">
                <h2 className="text-2xl font-bold text-white">Your Playlists</h2>
                <div className="flex flex-wrap items-center gap-3">
                    <button 
                        onClick={handleCreateSonicFingerprintPlaylist} 
                        disabled={isGenerating} 
                        className="inline-flex items-center gap-2 bg-gradient-to-r from-accent-500 to-accent-600 hover:from-accent-600 hover:to-accent-700 text-white font-semibold py-2.5 px-5 rounded-lg shadow-lg transition-all disabled:opacity-50 disabled:cursor-not-allowed"
                    >
                        {isGenerating ? (
                            <>
                                <svg className="animate-spin h-5 w-5" fill="none" viewBox="0 0 24 24">
                                    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                                    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                                </svg>
                                Generating...
                            </>
                        ) : (
                            <>
                                <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M13 10V3L4 14h7v7l9-11h-7z"></path>
                                </svg>
                                Sonic Fingerprint
                            </>
                        )}
                    </button>
                    <button 
                        onClick={() => setIsCreating(true)} 
                        className="inline-flex items-center gap-2 bg-dark-750 hover:bg-dark-700 text-white border border-dark-600 hover:border-accent-500/50 font-semibold py-2.5 px-5 rounded-lg transition-all"
                    >
                        <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 4v16m8-8H4"></path>
                        </svg>
                        Create Playlist
                    </button>
                </div>
            </div>

            {/* Status Messages */}
            {error && (
                <div className="bg-red-500/10 border border-red-500/50 rounded-lg p-4 mb-6 animate-fade-in">
                    <p className="text-red-400 flex items-center gap-2">
                        <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                        </svg>
                        {error}
                    </p>
                </div>
            )}
            {successMessage && (
                <div className="bg-green-500/10 border border-green-500/50 rounded-lg p-4 mb-6 animate-fade-in">
                    <p className="text-green-400 flex items-center gap-2">
                        <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                        </svg>
                        {successMessage}
                    </p>
                </div>
            )}

            {isCreating && <CreatePlaylistModal onClose={() => setIsCreating(false)} onSubmit={handleCreatePlaylist} />}
            
            {playlistToDelete && (
                <ConfirmationModal
                    onClose={() => setPlaylistToDelete(null)}
                    onConfirm={confirmDeletePlaylist}
                    title="Confirm Deletion"
                    message={`Are you sure you want to delete the playlist "${playlistToDelete.name}"?`}
                />
            )}
            
            {/* Empty State */}
            {playlists.length === 0 && !isLoading ? (
                <div className="flex flex-col items-center justify-center py-20 text-center">
                    <div className="bg-dark-750 rounded-full p-6 mb-6">
                        <svg className="w-16 h-16 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M9 19V6l12-3v13M9 19c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zm12-3c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zM9 10l12-3"></path>
                        </svg>
                    </div>
                    <h3 className="text-2xl font-semibold text-gray-300 mb-2">No Playlists Yet</h3>
                    <p className="text-gray-500 mb-8 max-w-md">Create your first playlist to organize your favorite music, or let AI generate one for you!</p>
                    <div className="flex flex-col sm:flex-row gap-3">
                        <button 
                            onClick={() => setIsCreating(true)} 
                            className="inline-flex items-center gap-2 bg-gradient-accent text-white font-semibold py-3 px-6 rounded-lg shadow-lg hover:shadow-glow transition-all"
                        >
                            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 4v16m8-8H4"></path>
                            </svg>
                            Create Playlist
                        </button>
                        <button 
                            onClick={handleCreateSonicFingerprintPlaylist} 
                            disabled={isGenerating} 
                            className="inline-flex items-center gap-2 bg-dark-750 hover:bg-dark-700 text-white border border-dark-600 hover:border-accent-500/50 font-semibold py-3 px-6 rounded-lg transition-all disabled:opacity-50"
                        >
                            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M13 10V3L4 14h7v7l9-11h-7z"></path>
                            </svg>
                            {isGenerating ? 'Generating...' : 'Sonic Fingerprint'}
                        </button>
                    </div>
                </div>
            ) : (
                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
                    {playlists.map((p, index) => (
                        <div 
                            ref={index === playlists.length - 1 ? lastPlaylistElementRef : null}
                            key={p.id} 
                            className="group bg-dark-750 rounded-xl p-5 card-hover cursor-pointer border border-dark-600 hover:border-accent-500/30 transition-all"
                            onClick={() => onNavigate({ page: 'songs', title: p.name, filter: { playlistId: p.id } })}
                        >
                            <div className="flex items-start justify-between mb-4">
                                <div className="bg-gradient-to-br from-accent-500/20 to-purple-500/20 rounded-lg p-3">
                                    <svg className="w-8 h-8 text-accent-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M9 19V6l12-3v13M9 19c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zm12-3c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zM9 10l12-3"></path>
                                    </svg>
                                </div>
                                {((p.owner && credentials.username === p.owner) || (p.public && isAdmin)) && (
                                    <button 
                                        onClick={(e) => {
                                            e.stopPropagation();
                                            setPlaylistToDelete({ id: p.id, name: p.name });
                                        }}
                                        className="text-gray-500 hover:text-red-400 transition-colors p-2 rounded-lg hover:bg-dark-700"
                                        title="Delete playlist"
                                    >
                                        <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"></path>
                                        </svg>
                                    </button>
                                )}
                            </div>
                            <h3 className="font-bold text-lg text-white truncate group-hover:text-accent-400 transition-colors mb-1">
                                {p.name}
                            </h3>
                            <p className="text-sm text-gray-400">{p.songCount} songs</p>
                        </div>
                    ))}
                </div>
            )}
            
            {/* Loading State */}
            {isLoading && allPlaylists.length === 0 && (
                <div className="flex justify-center items-center py-12">
                    <svg className="animate-spin h-8 w-8 text-accent-400" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                        <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                        <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                    </svg>
                </div>
            )}
            {!hasMore && playlists.length > 0 && <p className="text-center text-gray-500 mt-8 text-sm">You've reached the end</p>}
        </div>
    );
}

export default Playlists;
