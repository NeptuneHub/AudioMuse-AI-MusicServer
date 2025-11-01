// Suggested path: music-server-frontend/src/components/MusicViews.jsx
import React, { useState, useEffect, useCallback, useRef } from 'react';
import { API_BASE, subsonicFetch, starSong, unstarSong, getStarredSongs, getGenres } from '../api';

const formatDate = (isoString) => {
    if (!isoString) return 'Never';
    try {
        const date = new Date(isoString);
        return date.toLocaleDateString(undefined, { year: '2-digit', month: 'short', day: 'numeric' });
    } catch (e) {
        return 'Invalid Date';
    }
};

export const AddToPlaylistModal = ({ song, credentials, onClose, onAdded }) => {
    const [playlists, setPlaylists] = useState([]);
    const [selectedPlaylist, setSelectedPlaylist] = useState('');
    const [error, setError] = useState('');
    const [success, setSuccess] = useState('');

    useEffect(() => {
        const fetchPlaylists = async () => {
            try {
                // The getPlaylists.view endpoint doesn't support pagination, so we fetch all.
                const data = await subsonicFetch('getPlaylists.view');
                const playlistData = data.playlists?.playlist || [];
                setPlaylists(Array.isArray(playlistData) ? playlistData : [playlistData]);
                if (playlistData.length > 0) {
                    setSelectedPlaylist(playlistData[0].id);
                }
            } catch (err)
                {
                setError('Failed to fetch playlists.');
            }
        };
        fetchPlaylists();
    }, [credentials]);

    const handleAddToPlaylist = async () => {
        if (!selectedPlaylist) {
            setError('Please select a playlist.');
            return;
        }
        setError('');
        setSuccess('');
        try {
            await subsonicFetch('updatePlaylist.view', {
                playlistId: selectedPlaylist,
                songIdToAdd: song.id,
            });
            setSuccess(`Successfully added "${song.title}" to the playlist!`);
            onAdded();
            setTimeout(onClose, 1500);
        } catch (err) {
            setError('Failed to add song to playlist.');
        }
    };

    return (
        <div className="fixed inset-0 bg-black bg-opacity-60 flex items-center justify-center z-[60] p-4">
            <div className="bg-gray-800 p-6 rounded-lg shadow-xl w-full sm:w-11/12 md:max-w-md relative">
                 <h3 className="text-xl font-bold mb-4 text-teal-400">Add "{song.title}" to...</h3>
                {error && <p className="text-red-500 mb-2">{error}</p>}
                {success && <p className="text-green-400 mb-2">{success}</p>}
                <select
                    value={selectedPlaylist}
                    onChange={(e) => setSelectedPlaylist(e.target.value)}
                    className="w-full p-2 bg-gray-700 rounded mb-4"
                >
                    {playlists.map((p) => (
                        <option key={p.id} value={p.id}>{p.name}</option>
                    ))}
                </select>
                <div className="flex justify-end space-x-4">
                    <button onClick={onClose} className="bg-gray-600 hover:bg-gray-700 text-white font-bold py-2 px-4 rounded">Cancel</button>
                    <button onClick={handleAddToPlaylist} className="bg-teal-500 hover:bg-teal-600 text-white font-bold py-2 px-4 rounded">Add to Playlist</button>
                </div>
            </div>
        </div>
    );
};


export function Songs({ credentials, filter, onPlay, onAddToQueue, onRemoveFromQueue, playQueue = [], currentSong, onNavigate, audioMuseUrl, onInstantMix, onAddToPlaylist }) {
    const [songs, setSongs] = useState([]);
    const [allSongs, setAllSongs] = useState([]); // For client-side pagination
    const [searchTerm, setSearchTerm] = useState('');
    const [isLoading, setIsLoading] = useState(false);
    const [error, setError] = useState('');
    const [hasMore, setHasMore] = useState(true);
    const [refreshKey, setRefreshKey] = useState(0);
    const [genres, setGenres] = useState([]);
    const [selectedGenre, setSelectedGenre] = useState('');

    const isPlaylistView = !!filter?.playlistId;
    const PAGE_SIZE = 10;

    // Load genres on component mount
    useEffect(() => {
        const loadGenres = async () => {
            try {
                const data = await getGenres();
                const genreList = data.genres?.genre || [];
                const allGenres = Array.isArray(genreList) ? genreList : [genreList].filter(Boolean);
                
                // Split semicolon-separated genres and remove duplicates
                const individualGenres = [];
                allGenres.forEach(genre => {
                    if (genre.name) {
                        const splitGenres = genre.name.split(';').map(g => g.trim()).filter(g => g);
                        splitGenres.forEach(g => {
                            if (!individualGenres.find(existing => existing.name === g)) {
                                individualGenres.push({ name: g });
                            }
                        });
                    }
                });
                
                setGenres(individualGenres.sort((a, b) => a.name.localeCompare(b.name)));
            } catch (err) {
                console.error('Failed to load genres:', err);
            }
        };
        loadGenres();
    }, []);

    // Handle star/unstar
    const handleStarToggle = async (song) => {
        try {
            if (song.starred) {
                await unstarSong(song.id);
            } else {
                await starSong(song.id);
            }
            
            // Update song in state
            const updateSongStar = (songList) => 
                songList.map(s => s.id === song.id ? {...s, starred: !s.starred} : s);
            
            setSongs(updateSongStar);
            setAllSongs(updateSongStar);
        } catch (err) {
            setError('Failed to update star status: ' + err.message);
        }
    };

    useEffect(() => {
        setSongs([]);
        setAllSongs([]);
        setHasMore(true);
    }, [searchTerm, filter, refreshKey, selectedGenre]);

    const loadMoreSongs = useCallback(() => {
        if (isLoading || !hasMore) return;
        setIsLoading(true);
        setError('');

        const fetcher = async () => {
            try {
                if (searchTerm.length >= 3) {
                    const data = await subsonicFetch('search2.view', { query: searchTerm, songCount: PAGE_SIZE, songOffset: songs.length });
                    const songList = data.searchResult2?.song || data.searchResult3?.song || [];
                    let newSongs = Array.isArray(songList) ? songList : [songList].filter(Boolean);
                    
                    // Client-side genre filtering for search results
                    if (selectedGenre) {
                        newSongs = newSongs.filter(song => {
                            if (!song.genre) return false;
                            // Handle multiple genres separated by semicolons
                            const genres = song.genre.split(';').map(g => g.trim());
                            
                            // Check for exact match first, then case-insensitive
                            return genres.includes(selectedGenre) || 
                                   genres.some(g => g.toLowerCase() === selectedGenre.toLowerCase());
                        });
                    }
                    
                    setSongs(prev => [...prev, ...newSongs]);
                    setHasMore(newSongs.length === PAGE_SIZE);
                    return;
                }

                let baseList = allSongs;
                if (baseList.length === 0 && !searchTerm) {
                    let songList = [];
                    if (filter?.preloadedSongs) songList = filter.preloadedSongs;
                    else if (filter?.similarToSongId) {
                        const data = await subsonicFetch('getSimilarSongs.view', { id: filter.similarToSongId, count: PAGE_SIZE });
                        songList = data.directory?.song || [];
                    } else if (filter) {
                        const endpoint = filter.albumId ? 'getAlbum.view' : 'getPlaylist.view';
                        const idParam = filter.albumId || filter.playlistId;
                        if (idParam) {
                            const data = await subsonicFetch(endpoint, { id: idParam });
                            const songContainer = data.album || data.directory;
                            if (songContainer?.song) songList = Array.isArray(songContainer.song) ? songContainer.song : [songContainer.song];
                        }
                    } else if (selectedGenre && !filter) {
                        // Load songs by genre using the dedicated endpoint with pagination
                        const data = await subsonicFetch('getSongsByGenre.view', { 
                            genre: selectedGenre, 
                            size: PAGE_SIZE, 
                            offset: songs.length 
                        });
                        const newSongs = data.songsByGenre?.song || [];
                        
                        // For genre filtering, append new songs (like search pagination)
                        setSongs(prev => [...prev, ...newSongs]);
                        setHasMore(newSongs.length === PAGE_SIZE);
                        return;
                    }
                    
                    baseList = Array.isArray(songList) ? songList : [songList].filter(Boolean);
                    
                    // Apply genre filtering for other cases (albums, playlists, etc.)
                    if (selectedGenre && (filter?.albumId || filter?.playlistId || filter?.preloadedSongs)) {
                        baseList = baseList.filter(song => {
                            if (!song.genre) return false;
                            const genres = song.genre.split(';').map(g => g.trim());
                            return genres.includes(selectedGenre) || 
                                   genres.some(g => g.toLowerCase() === selectedGenre.toLowerCase());
                        });
                    }
                    
                    setAllSongs(baseList);
                }

                const currentCount = songs.length;
                const newCount = currentCount + PAGE_SIZE;
                setSongs(baseList.slice(0, newCount));
                setHasMore(newCount < baseList.length);

            } catch (e) {
                console.error("Failed to fetch songs:", e);
                setError(e.message || "Failed to fetch songs.");
            } finally {
                setIsLoading(false);
            }
        };

        fetcher();
    }, [filter, searchTerm, songs.length, allSongs, isLoading, hasMore, selectedGenre]);

    useEffect(() => {
        if (songs.length === 0 && hasMore && (searchTerm.length >= 3 || filter || selectedGenre)) {
            const timer = setTimeout(() => loadMoreSongs(), 300);
            return () => clearTimeout(timer);
        }
    }, [songs.length, hasMore, loadMoreSongs, searchTerm, filter, selectedGenre]);

    const observer = useRef();
    const lastSongElementRef = useCallback(node => {
        if (isLoading) return;
        if (observer.current) observer.current.disconnect();
        observer.current = new IntersectionObserver(entries => {
            if (entries[0].isIntersecting && hasMore) {
                loadMoreSongs();
            }
        });
        if (node) observer.current.observe(node);
    }, [isLoading, hasMore, loadMoreSongs]);

    const handlePlayAlbum = () => {
        const listToPlay = allSongs.length > 0 ? allSongs : songs;
        if (listToPlay.length > 0) onPlay(listToPlay[0], listToPlay);
    };

    const handleDeleteSong = async (songIdToRemove) => {
        if (!isPlaylistView) return;
        try {
            const newSongIds = allSongs.filter(s => s.id !== songIdToRemove).map(s => s.id);
            await subsonicFetch('updatePlaylist.view', { playlistId: filter.playlistId, songId: newSongIds });
            setRefreshKey(k => k + 1);
        } catch (err) {
            setError(err.message || 'Failed to delete song.');
        }
    };
    
    const handleMoveSong = async (index, direction) => {
        if (!isPlaylistView) return;
        const newSongs = [...allSongs];
        const [movedSong] = newSongs.splice(index, 1);
        const newIndex = direction === 'up' ? index - 1 : index + 1;
        if (newIndex < 0 || newIndex > newSongs.length) return;
        newSongs.splice(newIndex, 0, movedSong);
        
        setAllSongs(newSongs); // Optimistic update
        const currentVisibleSongs = newSongs.slice(0, songs.length);
        setSongs(currentVisibleSongs);

        try {
            await subsonicFetch('updatePlaylist.view', { playlistId: filter.playlistId, songId: newSongs.map(s => s.id) });
        } catch (err) {
            setError(err.message || 'Failed to move song.');
            setAllSongs(allSongs); // Revert on failure
            setSongs(songs);
        }
    };

    return (
        <div>
            {error && <p className="text-red-500 mb-4 p-3 bg-red-900/50 rounded">{error}</p>}
            <div className="mb-4 flex flex-col sm:flex-row gap-4">
                <input
                    type="text"
                    placeholder="Search for a song or artist..."
                    value={searchTerm}
                    onChange={(e) => setSearchTerm(e.target.value)}
                    className="flex-1 p-2 bg-gray-700 rounded border border-gray-600 focus:outline-none focus:border-teal-500"
                />
                <select
                    value={selectedGenre}
                    onChange={(e) => setSelectedGenre(e.target.value)}
                    className="p-2 bg-gray-700 rounded border border-gray-600 focus:outline-none focus:border-teal-500 min-w-[120px]"
                >
                    <option value="">All Genres</option>
                    {genres.map(genre => (
                        <option key={genre.name} value={genre.name}>{genre.name}</option>
                    ))}
                </select>
            </div>

            <div className="mb-4 flex flex-wrap gap-2">
                {(songs.length > 0 || allSongs.length > 0) && (
                    <button onClick={handlePlayAlbum} className="bg-teal-500 hover:bg-teal-600 text-white font-bold py-2 px-4 rounded">
                        ▶ Play All ({allSongs.length > 0 ? allSongs.length : songs.length})
                    </button>
                )}
                <button 
                    onClick={async () => {
                        try {
                            // Clear all state first to ensure clean reset
                            setSongs([]);
                            setAllSongs([]);
                            setIsLoading(true);
                            setError('');
                            setSearchTerm('');
                            setSelectedGenre('');
                            setHasMore(false);
                            
                            // Force refresh by incrementing refreshKey
                            setRefreshKey(prev => prev + 1);
                            
                            const data = await getStarredSongs();
                            const starredSongs = data.starred?.song;
                            
                            // Handle empty/missing starred songs properly
                            let songList = [];
                            if (starredSongs) {
                                songList = Array.isArray(starredSongs) ? starredSongs : [starredSongs];
                                // Filter out any invalid entries (null, undefined, or objects without id)
                                songList = songList.filter(s => s && s.id);
                            }
                            
                            setAllSongs(songList);
                            setSongs(songList.slice(0, PAGE_SIZE));
                            setHasMore(songList.length > PAGE_SIZE);
                        } catch (err) {
                            setError('Failed to load starred songs: ' + err.message);
                        } finally {
                            setIsLoading(false);
                        }
                    }}
                    className="bg-yellow-500 hover:bg-yellow-600 text-white font-bold py-2 px-4 rounded"
                >
                    ⭐ Starred Songs
                </button>
            </div>
            
            {!isLoading && songs.length === 0 && (searchTerm || filter) && <p className="text-center text-gray-500">No songs found.</p>}

            {!isLoading && songs.length === 0 && !searchTerm && !filter && (
                 <p className="text-center text-gray-500">Start typing in the search bar to find songs.</p>
            )}

            {songs.length > 0 && (
                <div className="overflow-x-auto">
                    <table className="min-w-full text-sm text-left text-gray-400">
                        <thead className="text-xs text-gray-300 uppercase bg-gray-700">
                            <tr>
                                <th className="px-4 py-3 w-12"></th>
                                <th className="px-4 py-3 w-12 text-center">⭐</th>
                                <th className="px-4 py-3">Title</th>
                                <th className="px-4 py-3 hidden sm:table-cell">Artist</th>
                                <th className="px-4 py-3 hidden md:table-cell">Album</th>
                                <th className="px-4 py-3 hidden lg:table-cell">Genre</th>
                                <th className="px-4 py-3 hidden xl:table-cell text-center">Plays</th>
                                <th className="px-4 py-3 hidden lg:table-cell">Last Played</th>
                                <th className="px-4 py-3 w-48 text-right">Actions</th>
                            </tr>
                        </thead>
                        <tbody>
                            {songs.map((song, index) => {
                                const isPlaying = currentSong && currentSong.id === song.id;
                                const isInQueue = playQueue.some(s => s.id === song.id);
                                return (
                                    <tr ref={index === songs.length - 1 ? lastSongElementRef : null} key={`${song.id}-${index}`} className={`border-b border-gray-700 transition-colors ${isPlaying ? 'bg-teal-900/50' : 'bg-gray-800 hover:bg-gray-600'}`}>
                                        <td className="px-4 py-4">
                                            <button onClick={() => onPlay(song, allSongs.length > 0 ? allSongs : songs)} title="Play song">
                                                {isPlaying ? (
                                                    <svg className="w-6 h-6 text-green-400" fill="currentColor" viewBox="0 0 20 20"><path fillRule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zM7 8a1 1 0 012 0v4a1 1 0 11-2 0V8zm5-1a1 1 0 00-1 1v4a1 1 0 102 0V8a1 1 0 00-1-1z" clipRule="evenodd"></path></svg>
                                                ) : (
                                                    <svg className="w-6 h-6 text-teal-400 hover:text-teal-200" fill="currentColor" viewBox="0 0 20 20"><path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM9.555 7.168A1 1 0 008 8v4a1 1 0 001.555.832l3-2a1 1 0 000-1.664l-3-2z" clipRule="evenodd"></path></svg>
                                                )}
                                            </button>
                                        </td>
                                        <td className="px-4 py-4 text-center">
                                            <button
                                                onClick={() => handleStarToggle(song)}
                                                className={`text-2xl hover:scale-110 transition-transform ${
                                                    song.starred ? 'text-yellow-400' : 'text-gray-600'
                                                }`}
                                                title={song.starred ? 'Remove from favorites' : 'Add to favorites'}
                                            >
                                                {song.starred ? '⭐' : '☆'}
                                            </button>
                                        </td>
                                        <td className={`px-4 py-4 font-medium ${isPlaying ? 'text-green-400' : 'text-white'}`}>
                                            <div>{song.title}</div>
                                            <div className="sm:hidden text-xs text-gray-400">{song.artist}</div>
                                        </td>
                                        <td className="px-4 py-4 hidden sm:table-cell">{song.artist}</td>
                                        <td className="px-4 py-4 hidden md:table-cell">{song.album}</td>
                                        <td className="px-4 py-4 hidden lg:table-cell text-gray-400">{song.genre || 'Unknown'}</td>
                                        <td className="px-4 py-3 hidden xl:table-cell text-center">{song.playCount > 0 ? song.playCount : ''}</td>
                                        <td className="px-4 py-3 hidden lg:table-cell">{formatDate(song.lastPlayed)}</td>
                                        <td className="px-4 py-4">
                                            <div className="flex items-center justify-end space-x-2">
                                                 {isPlaylistView && (
                                                    <>
                                                        <div className="flex flex-col -my-1">
                                                            <button onClick={() => handleMoveSong(index, 'up')} disabled={index === 0} className="p-1 text-gray-400 hover:text-white disabled:text-gray-600 disabled:cursor-not-allowed" title="Move up">
                                                                <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 20 20"><path fillRule="evenodd" d="M14.707 12.707a1 1 0 01-1.414 0L10 9.414l-3.293 3.293a1 1 0 01-1.414-1.414l4-4a1 1 0 011.414 0l4 4a1 1 0 010 1.414z" clipRule="evenodd"></path></svg>
                                                            </button>
                                                            <button onClick={() => handleMoveSong(index, 'down')} disabled={index === allSongs.length - 1} className="p-1 text-gray-400 hover:text-white disabled:text-gray-600 disabled:cursor-not-allowed" title="Move down">
                                                                <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 20 20"><path fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd"></path></svg>
                                                            </button>
                                                        </div>
                                                        <button onClick={() => handleDeleteSong(song.id)} title="Remove from playlist" className="p-1 text-gray-400 hover:text-red-500">
                                                            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M15 12H9m12 0a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg>
                                                        </button>
                                                        <div className="border-l border-gray-600 h-6 mx-1"></div>
                                                    </>
                                                )}
                                                <button
                                                    onClick={() => {
                                                        if (!audioMuseUrl) {
                                                            // Minimal error handler when AudioMuse isn't configured
                                                            alert('Instant Mix is not configured on the server. Ask an admin to enable AudioMuse.');
                                                            return;
                                                        }
                                                        onInstantMix(song);
                                                    }}
                                                    title="Instant Mix"
                                                    className="p-1 rounded-full transition-colors text-teal-400 hover:bg-gray-700"
                                                >
                                                    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M13 10V3L4 14h7v7l9-11h-7z"></path></svg>
                                                </button>
                                                {isInQueue ? (
                                                    <button onClick={() => onRemoveFromQueue(song.id)} title="Remove from queue" className="text-gray-400 hover:text-red-500">
                                                        <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"></path></svg>
                                                    </button>
                                                ) : (
                                                    <button onClick={() => onAddToQueue(song)} title="Add to queue" className="text-gray-400 hover:text-white">
                                                        <svg className="w-6 h-6" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M4 6h16M4 10h16M4 14h4" /><path d="M16 12v8m-4-4h8" className="stroke-green-500" /></svg>
                                                    </button>
                                                )}
                                                <button onClick={() => onAddToPlaylist(song)} title="Add to playlist" className="text-gray-400 hover:text-white">
                                                    <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 6v6m0 0v6m0-6h6m-6 0H6"></path></svg>
                                                </button>
                                            </div>
                                        </td>
                                    </tr>
                                );
                            })}
                        </tbody>
                    </table>
                    {isLoading && <p className="text-center text-gray-400 mt-4">Loading more songs...</p>}
                    {!hasMore && songs.length > 0 && <p className="text-center text-gray-500 mt-4">End of list.</p>}
                </div>
            )}
        </div>
    );
}

const AlbumPlaceholder = ({ name }) => {
	const initials = (name || '??').split(' ').slice(0, 2).map(word => word[0]).join('').toUpperCase();
	return (
		<div className="w-full h-full bg-gray-700 flex items-center justify-center">
			<span className="text-gray-400 text-3xl font-bold">{initials}</span>
		</div>
	);
};

const ArtistPlaceholder = () => (
	<div className="w-full h-full bg-gray-700 flex items-center justify-center rounded-full">
		<svg className="w-1/2 h-1/2 text-gray-500" fill="currentColor" viewBox="0 0 20 20"><path fillRule="evenodd" d="M10 9a3 3 0 100-6 3 3 0 000 6zm-7 9a7 7 0 1114 0H3z" clipRule="evenodd"></path></svg>
	</div>
);

const ImageWithFallback = ({ src, placeholder, alt }) => {
    const [hasError, setHasError] = useState(false);
    const [isVisible, setIsVisible] = useState(false);
    const [objectUrl, setObjectUrl] = useState(null);
    const objectUrlRef = useRef(null);
    const ref = useRef(null);

    useEffect(() => {
        const observer = new IntersectionObserver(
            ([entry]) => {
                if (entry.isIntersecting) {
                    setIsVisible(true);
                    observer.unobserve(entry.target);
                }
            },
            { rootMargin: '200px' }
        );

        const currentRef = ref.current;
        if (currentRef) {
            observer.observe(currentRef);
        }

        return () => {
            if (currentRef) {
                observer.unobserve(currentRef);
            }
        };
    }, []);

    useEffect(() => {
        setHasError(false);
        // Clean up previous objectUrl stored in ref
        if (objectUrlRef.current) {
            URL.revokeObjectURL(objectUrlRef.current);
            objectUrlRef.current = null;
            setObjectUrl(null);
        }
        // If src is an object with useAuthFetch=true and a token exists, fetch the image via fetch with Authorization
        const doFetch = async () => {
            try {
                if (!src || typeof src === 'string') return;
                const token = localStorage.getItem('token');
                if (src.useAuthFetch && token && src.url) {
                    const res = await fetch(src.url, { headers: { 'Authorization': `Bearer ${token}` } });
                    if (!res.ok) throw new Error('Failed to load image');
                    const blob = await res.blob();
                    const url = URL.createObjectURL(blob);
                    setObjectUrl(url);
                    objectUrlRef.current = url;
                }
            } catch (e) {
                console.error('Image fetch failed', e);
                setHasError(true);
            }
        };
        doFetch();
    }, [src]);

    return (
        <div ref={ref} className="w-full h-full">
            {isVisible && !hasError ? (
                (objectUrl && <img src={objectUrl} alt={alt} onError={() => setHasError(true)} className="w-full h-full object-cover" />)
                || (typeof src === 'string' && <img src={src} alt={alt} onError={() => setHasError(true)} className="w-full h-full object-cover" />)
                || placeholder
            ) : (
                placeholder
            )}
        </div>
    );
};


export function Albums({ credentials, filter, onNavigate }) {
    const [albums, setAlbums] = useState([]);
    const [searchTerm, setSearchTerm] = useState('');
    const [isLoading, setIsLoading] = useState(false);
    const [hasMore, setHasMore] = useState(true);
    const [genres, setGenres] = useState([]);
    const [selectedGenre, setSelectedGenre] = useState('');
    const PAGE_SIZE = 10;
    
    // Load genres on component mount
    useEffect(() => {
        const loadGenres = async () => {
            try {
                const data = await getGenres();
                const genreList = data.genres?.genre || [];
                const allGenres = Array.isArray(genreList) ? genreList : [genreList].filter(Boolean);
                
                // Split semicolon-separated genres and remove duplicates
                const individualGenres = [];
                allGenres.forEach(genre => {
                    if (genre.name) {
                        const splitGenres = genre.name.split(';').map(g => g.trim()).filter(g => g);
                        splitGenres.forEach(g => {
                            if (!individualGenres.find(existing => existing.name === g)) {
                                individualGenres.push({ name: g });
                            }
                        });
                    }
                });
                
                setGenres(individualGenres.sort((a, b) => a.name.localeCompare(b.name)));
            } catch (err) {
                console.error('Failed to load genres:', err);
            }
        };
        loadGenres();
    }, []);
    
    useEffect(() => {
        setAlbums([]);
        setHasMore(true);
        if(filter) setSearchTerm(filter);
    }, [filter]);
    
    useEffect(() => {
        setAlbums([]);
        setHasMore(true);
    }, [searchTerm, selectedGenre])


    const loadMoreAlbums = useCallback(() => {
        if (isLoading || !hasMore) return;
        setIsLoading(true);

        const fetcher = async () => {
            try {
                let albumList = [];
                const query = searchTerm || filter;

                if (query) {
                    const data = await subsonicFetch('search2.view', { query, albumCount: PAGE_SIZE, albumOffset: albums.length });
                    albumList = data.searchResult2?.album || data.searchResult3?.album || [];
                } else {
                    const params = { type: 'alphabeticalByName', size: PAGE_SIZE, offset: albums.length };
                    if (selectedGenre) params.genre = selectedGenre;
                    const data = await subsonicFetch('getAlbumList2.view', params);
                    albumList = data.albumList2?.album || [];
                }
                const newAlbums = Array.isArray(albumList) ? albumList : [albumList].filter(Boolean);
                setAlbums(prev => [...prev, ...newAlbums]);
                setHasMore(newAlbums.length === PAGE_SIZE);

            } catch (e) {
                console.error("Failed to fetch albums:", e);
            } finally {
                setIsLoading(false);
            }
        };
        
        fetcher();
    }, [filter, searchTerm, albums.length, isLoading, hasMore, selectedGenre]);
    
    useEffect(() => {
        if (albums.length === 0 && hasMore) {
            const timer = setTimeout(() => loadMoreAlbums(), 300);
            return () => clearTimeout(timer);
        }
    }, [albums.length, hasMore, loadMoreAlbums]);


    const observer = useRef();
    const lastAlbumElementRef = useCallback(node => {
        if (isLoading) return;
        if (observer.current) observer.current.disconnect();
        observer.current = new IntersectionObserver(entries => {
            if (entries[0].isIntersecting && hasMore) {
                loadMoreAlbums();
            }
        });
        if (node) observer.current.observe(node);
    }, [isLoading, hasMore, loadMoreAlbums]);

    return (
        <div>
            <div className="mb-4 flex flex-col sm:flex-row gap-4">
                <input
                    type="text"
                    placeholder="Search for an album or artist..."
                    value={searchTerm}
                    onChange={(e) => setSearchTerm(e.target.value)}
                    className="flex-1 p-2 bg-gray-700 rounded border border-gray-600 focus:outline-none focus:border-teal-500"
                />
                <select
                    value={selectedGenre}
                    onChange={(e) => setSelectedGenre(e.target.value)}
                    className="p-2 bg-gray-700 rounded border border-gray-600 focus:outline-none focus:border-teal-500 min-w-[120px]"
                >
                    <option value="">All Genres</option>
                    {genres.map(genre => (
                        <option key={genre.name} value={genre.name}>{genre.name}</option>
                    ))}
                </select>
            </div>
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-6">
                {albums.map((album, index) => (
                    <button 
                        ref={index === albums.length - 1 ? lastAlbumElementRef : null}
                        key={`${album.id}-${index}`} 
                        onClick={() => onNavigate({ page: 'songs', title: album.name, filter: { albumId: album.id } })} 
                        className="bg-gray-800 rounded-lg p-4 text-center hover:bg-gray-700 transition-colors">
                        <div className="w-full bg-gray-700 rounded aspect-square flex items-center justify-center mb-2 overflow-hidden">
                             <ImageWithFallback
                                src={album.coverArt ? (() => {
                                    const params = new URLSearchParams({ id: album.coverArt, v: '1.16.1', c: 'AudioMuse-AI', size: '512' });
                                    const url = `${API_BASE}/rest/getCoverArt.view?${params.toString()}`;
                                    return { url, useAuthFetch: true };
                                })() : ''}
                                placeholder={<AlbumPlaceholder name={album.name} />}
                                alt={album.name}
                            />
                        </div>
                        <p className="font-bold text-white truncate">{album.name}</p>
                        <p className="text-sm text-gray-400 truncate">{album.artist}</p>
                    </button>
                ))}
            </div>
            {isLoading && <p className="text-center text-gray-400 mt-4">Loading more albums...</p>}
            {!hasMore && albums.length > 0 && <p className="text-center text-gray-500 mt-4">End of list.</p>}
        </div>
    );
}

export function Artists({ credentials, onNavigate }) {
    const [artists, setArtists] = useState([]);
    const [allArtists, setAllArtists] = useState([]); // For client-side pagination
    const [searchTerm, setSearchTerm] = useState('');
    const [isLoading, setIsLoading] = useState(false);
    const [hasMore, setHasMore] = useState(true);
    const PAGE_SIZE = 10;

    useEffect(() => {
        setArtists([]);
        setHasMore(true);
        if (searchTerm.length === 0) setAllArtists([]);
    }, [searchTerm]);

    const loadMoreArtists = useCallback(() => {
        if (isLoading || !hasMore) return;
        setIsLoading(true);

        const fetcher = async () => {
            try {
                if (searchTerm.length >= 2) {
                    const data = await subsonicFetch('search2.view', { query: searchTerm, artistCount: PAGE_SIZE, artistOffset: artists.length });
                    const artistList = data.searchResult2?.artist || data.searchResult3?.artist || [];
                    const newArtists = Array.isArray(artistList) ? artistList : [artistList].filter(Boolean);
                    setArtists(prev => [...prev, ...newArtists]);
                    setHasMore(newArtists.length === PAGE_SIZE);
                } 
                else if (searchTerm.length === 0) { 
                    let baseList = allArtists;
                    if (baseList.length === 0) {
                        const data = await subsonicFetch('getArtists.view');
                        const indexData = data.artists?.index || [];
                        const indices = Array.isArray(indexData) ? indexData : [indexData].filter(Boolean);
                        baseList = indices.flatMap(i => i.artist || []);
                        setAllArtists(baseList);
                    }
                    const currentCount = artists.length;
                    const newCount = currentCount + PAGE_SIZE;
                    setArtists(baseList.slice(0, newCount));
                    setHasMore(newCount < baseList.length);
                } else {
                    setHasMore(false);
                }
            } catch (e) {
                console.error("Failed to fetch artists:", e);
            } finally {
                setIsLoading(false);
            }
        };

        fetcher();
    }, [searchTerm, artists.length, allArtists, isLoading, hasMore]);
    
    useEffect(() => {
        if (artists.length === 0 && hasMore) {
            const timer = setTimeout(() => loadMoreArtists(), 300);
            return () => clearTimeout(timer);
        }
    }, [artists.length, hasMore, loadMoreArtists]);


    const observer = useRef();
    const lastArtistElementRef = useCallback(node => {
        if (isLoading) return;
        if (observer.current) observer.current.disconnect();
        observer.current = new IntersectionObserver(entries => {
            if (entries[0].isIntersecting && hasMore) {
                loadMoreArtists();
            }
        });
        if (node) observer.current.observe(node);
    }, [isLoading, hasMore, loadMoreArtists]);

    return (
        <div>
            <div className="mb-4">
                <input
                    type="text"
                    placeholder="Search for an artist..."
                    value={searchTerm}
                    onChange={(e) => setSearchTerm(e.target.value)}
                    className="w-full p-2 bg-gray-700 rounded border border-gray-600 focus:outline-none focus:border-teal-500"
                />
            </div>
             <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-6">
                {artists.map((artist, index) => (
                    <button 
                        ref={index === artists.length - 1 ? lastArtistElementRef : null}
                        key={`${artist.id}-${index}`} 
                        onClick={() => onNavigate({ page: 'albums', title: artist.name, filter: artist.name })} 
                        className="bg-gray-800 rounded-lg p-4 text-center hover:bg-gray-700 transition-colors flex flex-col items-center">
                        <div className="w-32 h-32 sm:w-40 sm:h-40 rounded-full bg-gray-700 flex items-center justify-center mb-2 overflow-hidden flex-shrink-0">
                             <ImageWithFallback
                                src={artist.artistImageUrl ? (() => {
                                    const params = new URLSearchParams({ id: artist.artistImageUrl, v: '1.16.1', c: 'AudioMuse-AI', size: '512' });
                                    const url = `${API_BASE}/rest/getCoverArt.view?${params.toString()}`;
                                    return { url, useAuthFetch: true };
                                })() : ''}
                                placeholder={<ArtistPlaceholder />}
                                alt={artist.name}
                            />
                        </div>
                        <p className="font-bold text-white truncate w-full">{artist.name}</p>
                    </button>
                ))}
            </div>
            {isLoading && <p className="text-center text-gray-400 mt-4">Loading more artists...</p>}
            {!hasMore && artists.length > 0 && <p className="text-center text-gray-500 mt-4">End of list.</p>}
        </div>
    );
}

