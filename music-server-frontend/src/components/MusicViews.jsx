// Suggested path: music-server-frontend/src/components/MusicViews.jsx
import React, { useState, useEffect, useCallback, useRef, useMemo } from 'react';
import { API_BASE, subsonicFetch, starSong, unstarSong, getStarredSongs, getGenres, getMusicCounts, getRecentlyAdded, getMostPlayed, getRecentlyPlayed } from '../api';

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
                const data = await subsonicFetch('getPlaylists.view');
                const playlistData = data.playlists?.playlist || [];
                setPlaylists(Array.isArray(playlistData) ? playlistData : [playlistData]);
                if (playlistData.length > 0) {
                    setSelectedPlaylist(playlistData[0].id);
                }
            } catch (err) {
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
            // First, fetch the playlist details to check if song already exists
            const playlistData = await subsonicFetch('getPlaylist.view', {
                id: selectedPlaylist
            });
            
            const existingSongs = playlistData.playlist?.entry || [];
            const songList = Array.isArray(existingSongs) ? existingSongs : (existingSongs ? [existingSongs] : []);
            
            // Check if song is already in the playlist
            const songAlreadyExists = songList.some(s => String(s.id) === String(song.id));
            
            if (songAlreadyExists) {
                setError('Song already present in the playlist');
                return;
            }
            
            // Add the song to the playlist
            await subsonicFetch('updatePlaylist.view', {
                playlistId: selectedPlaylist,
                songIdToAdd: song.id,
            });
            setSuccess(`Successfully added "${song.title}" to the playlist!`);
            onAdded();
            setTimeout(onClose, 1500);
        } catch (err) {
            console.error('Failed to add song to playlist:', err);
            setError('Failed to add song to playlist.');
        }
    };

    return (
        <div className="fixed inset-0 bg-black bg-opacity-70 backdrop-blur-sm flex items-center justify-center z-[60] p-4 animate-fade-in">
            <div className="glass rounded-2xl shadow-2xl w-full sm:w-11/12 md:max-w-md relative animate-scale-in">
                <div className="p-6 sm:p-8">
                    <div className="flex items-start justify-between mb-6">
                        <div>
                            <h3 className="text-xl font-bold text-white mb-1">Add to Playlist</h3>
                            <p className="text-sm text-gray-400 truncate max-w-[280px]">"{song.title}"</p>
                        </div>
                        <button 
                            onClick={onClose}
                            className="text-gray-400 hover:text-white transition-colors p-1"
                        >
                            <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12"></path>
                            </svg>
                        </button>
                    </div>

                    {error && (
                        <div className="bg-red-500/10 border border-red-500/50 rounded-lg p-3 mb-4 animate-fade-in">
                            <p className="text-red-400 text-sm flex items-center gap-2">
                                <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                                </svg>
                                {error}
                            </p>
                        </div>
                    )}
                    
                    {success && (
                        <div className="bg-green-500/10 border border-green-500/50 rounded-lg p-3 mb-4 animate-fade-in">
                            <p className="text-green-400 text-sm flex items-center gap-2">
                                <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                                </svg>
                                {success}
                            </p>
                        </div>
                    )}

                    <select
                        value={selectedPlaylist}
                        onChange={(e) => setSelectedPlaylist(e.target.value)}
                        className="w-full p-3 bg-dark-700 rounded-lg border border-dark-600 focus:outline-none focus:border-accent-500 focus:ring-2 focus:ring-accent-500/20 text-white mb-6 transition-all"
                    >
                        {playlists.map((p) => (
                            <option key={p.id} value={p.id}>{p.name}</option>
                        ))}
                    </select>

                    <div className="flex justify-end gap-3">
                        <button 
                            onClick={onClose} 
                            className="px-5 py-2.5 rounded-lg bg-dark-700 hover:bg-dark-600 text-white font-semibold transition-all"
                        >
                            Cancel
                        </button>
                        <button 
                            onClick={handleAddToPlaylist} 
                            className="px-5 py-2.5 rounded-lg bg-gradient-accent text-white font-semibold transition-all shadow-lg hover:shadow-glow"
                        >
                            Add to Playlist
                        </button>
                    </div>
                </div>
            </div>
        </div>
    );
};


export function Songs({ credentials, filter, onPlay, onTogglePlayPause, onAddToQueue, onRemoveFromQueue, playQueue = [], currentSong, isAudioPlaying = false, onNavigate, audioMuseUrl, onInstantMix, onAddToPlaylist }) {
    const [songs, setSongs] = useState([]);
    const [allSongs, setAllSongs] = useState([]); // For client-side pagination
    const [searchTerm, setSearchTerm] = useState('');
    const [isLoading, setIsLoading] = useState(false);
    const [error, setError] = useState('');
    const [hasMore, setHasMore] = useState(true);
    const [refreshKey, setRefreshKey] = useState(0);
    const [genres, setGenres] = useState([]);
    const [selectedGenre, setSelectedGenre] = useState('');
    const [playlistOwner, setPlaylistOwner] = useState(null);
    const [discoveryView, setDiscoveryView] = useState('all'); // 'all', 'recent', 'popular', 'history'
    const [totalCount, setTotalCount] = useState(0);
    const [isStarredFilter, setIsStarredFilter] = useState(false);

    const isPlaylistView = !!filter?.playlistId;
    const PAGE_SIZE = 10;
    
    // Check if playlist is read-only (owned by another user)
    const isPlaylistReadOnly = isPlaylistView && playlistOwner && credentials?.username && playlistOwner !== credentials.username;

    // Load genres on component mount
    useEffect(() => {
        const loadGenres = async () => {
            try {
                const data = await getGenres();
                console.log('Raw genre data from API:', data);
                const genreList = data.genres?.genre || [];
                console.log('Genre list after extraction:', genreList);
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
                
                console.log('Processed individual genres:', individualGenres);
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
        setDiscoveryView('all'); // Reset discovery view on filter/genre change
    }, [searchTerm, filter, refreshKey, selectedGenre]);

    // Load counts when genre changes or component mounts
    useEffect(() => {
        const loadCounts = async () => {
            try {
                const counts = await getMusicCounts(selectedGenre);
                setTotalCount(counts.songs);
            } catch (err) {
                console.error('Failed to load counts:', err);
            }
        };
        if (!filter) {
            loadCounts();
        }
    }, [selectedGenre, filter]);

    const loadMoreSongs = useCallback(() => {
        if (isLoading || !hasMore) return;
        setIsLoading(true);
        setError('');

        const fetcher = async () => {
            try {
                // Handle discovery views with pagination
                if (!filter && !searchTerm && discoveryView !== 'all') {
                    let newSongs = [];
                    try {
                        if (discoveryView === 'recent') {
                            newSongs = await getRecentlyAdded(PAGE_SIZE, songs.length, selectedGenre);
                        } else if (discoveryView === 'popular') {
                            newSongs = await getMostPlayed(PAGE_SIZE, songs.length, selectedGenre);
                        } else if (discoveryView === 'history') {
                            newSongs = await getRecentlyPlayed(PAGE_SIZE, songs.length, selectedGenre);
                        }
                        const songsArray = newSongs || [];
                        setSongs(prev => [...prev, ...songsArray]);
                        setHasMore(songsArray.length === PAGE_SIZE);
                    } catch (err) {
                        setError('Failed to load more songs: ' + err.message);
                        setHasMore(false);
                    } finally {
                        setIsLoading(false);
                    }
                    return;
                }

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
                            
                            // Store playlist owner for access control
                            if (filter.playlistId && data.playlist) {
                                setPlaylistOwner(data.playlist.owner || null);
                                console.log('Playlist owner:', data.playlist.owner, 'Current user:', credentials?.username);
                            }
                            
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
    }, [filter, searchTerm, songs.length, allSongs, isLoading, hasMore, selectedGenre, credentials?.username, discoveryView]);

    useEffect(() => {
        if (songs.length === 0 && hasMore && (searchTerm.length >= 3 || filter || selectedGenre || discoveryView !== 'all')) {
            const timer = setTimeout(() => loadMoreSongs(), 300);
            return () => clearTimeout(timer);
        }
    }, [songs.length, hasMore, loadMoreSongs, searchTerm, filter, selectedGenre, discoveryView]);

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
            
            {/* Read-only playlist notice */}
            {isPlaylistReadOnly && (
                <div className="bg-yellow-500/10 border border-yellow-500/50 rounded-lg p-4 mb-6 animate-fade-in">
                    <p className="text-yellow-400 flex items-center gap-2">
                        <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"></path>
                        </svg>
                        Read-Only: This playlist is owned by {playlistOwner}. You can view and play songs but cannot modify it.
                    </p>
                </div>
            )}

            {/* Discovery Tabs - only show when not in a filtered view */}
            {!filter && (
                <div className="mb-6">
                    <div className="flex gap-2 overflow-x-auto pb-2">
                        <button
                            onClick={() => {
                                setDiscoveryView('all');
                                setSongs([]);
                                setAllSongs([]);
                                setHasMore(true);
                                setSearchTerm('');
                            }}
                            className={`px-3 sm:px-4 py-2 rounded-lg font-semibold whitespace-nowrap transition-all ${
                                discoveryView === 'all'
                                    ? 'bg-gradient-accent text-white shadow-glow'
                                    : 'bg-dark-750 text-gray-400 hover:bg-dark-700 hover:text-white'
                            }`}
                            title="All Songs"
                        >
                            <span className="flex items-center gap-2">
                                <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M9 19V6l12-3v13M9 19c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zm12-3c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zM9 10l12-3"></path>
                                </svg>
                                <span className={discoveryView === 'all' ? 'inline' : 'hidden sm:inline'}>All Songs {totalCount > 0 && `(${totalCount})`}</span>
                            </span>
                        </button>
                        <button
                            onClick={async () => {
                                setDiscoveryView('recent');
                                setSongs([]);
                                setAllSongs([]);
                                setIsLoading(true);
                                setHasMore(true);
                                try {
                                    const newSongs = await getRecentlyAdded(PAGE_SIZE, 0, selectedGenre);
                                    const songsArray = newSongs || [];
                                    setAllSongs(songsArray);
                                    setSongs(songsArray.slice(0, PAGE_SIZE));
                                    setHasMore(songsArray.length >= PAGE_SIZE);
                                } catch (err) {
                                    setError('Failed to load recently added songs');
                                } finally {
                                    setIsLoading(false);
                                }
                            }}
                            className={`px-3 sm:px-4 py-2 rounded-lg font-semibold whitespace-nowrap transition-all ${
                                discoveryView === 'recent'
                                    ? 'bg-gradient-accent text-white shadow-glow'
                                    : 'bg-dark-750 text-gray-400 hover:bg-dark-700 hover:text-white'
                            }`}
                            title="Recently Added"
                        >
                            <span className="flex items-center gap-2">
                                <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                                </svg>
                                <span className={discoveryView === 'recent' ? 'inline' : 'hidden sm:inline'}>Recently Added</span>
                            </span>
                        </button>
                        <button
                            onClick={async () => {
                                setDiscoveryView('popular');
                                setSongs([]);
                                setAllSongs([]);
                                setIsLoading(true);
                                setHasMore(true);
                                try {
                                    const newSongs = await getMostPlayed(PAGE_SIZE, 0, selectedGenre);
                                    const songsArray = newSongs || [];
                                    setAllSongs(songsArray);
                                    setSongs(songsArray.slice(0, PAGE_SIZE));
                                    setHasMore(songsArray.length >= PAGE_SIZE);
                                } catch (err) {
                                    setError('Failed to load most played songs');
                                } finally {
                                    setIsLoading(false);
                                }
                            }}
                            className={`px-3 sm:px-4 py-2 rounded-lg font-semibold whitespace-nowrap transition-all ${
                                discoveryView === 'popular'
                                    ? 'bg-gradient-accent text-white shadow-glow'
                                    : 'bg-dark-750 text-gray-400 hover:bg-dark-700 hover:text-white'
                            }`}
                            title="Most Played"
                        >
                            <span className="flex items-center gap-2">
                                <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6"></path>
                                </svg>
                                <span className={discoveryView === 'popular' ? 'inline' : 'hidden sm:inline'}>Most Played</span>
                            </span>
                        </button>
                        <button
                            onClick={async () => {
                                setDiscoveryView('history');
                                setSongs([]);
                                setAllSongs([]);
                                setIsLoading(true);
                                setHasMore(true);
                                try {
                                    const newSongs = await getRecentlyPlayed(PAGE_SIZE, 0, selectedGenre);
                                    const songsArray = newSongs || [];
                                    setAllSongs(songsArray);
                                    setSongs(songsArray.slice(0, PAGE_SIZE));
                                    setHasMore(songsArray.length >= PAGE_SIZE);
                                } catch (err) {
                                    setError('Failed to load recently played songs');
                                } finally {
                                    setIsLoading(false);
                                }
                            }}
                            className={`px-3 sm:px-4 py-2 rounded-lg font-semibold whitespace-nowrap transition-all ${
                                discoveryView === 'history'
                                    ? 'bg-gradient-accent text-white shadow-glow'
                                    : 'bg-dark-750 text-gray-400 hover:bg-dark-700 hover:text-white'
                            }`}
                            title="Play History"
                        >
                            <span className="flex items-center gap-2">
                                <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"></path>
                                </svg>
                                <span className={discoveryView === 'history' ? 'inline' : 'hidden sm:inline'}>Play History</span>
                            </span>
                        </button>
                    </div>
                </div>
            )}
            
            {/* Modern Search Bar */}
            <div className="mb-6 flex flex-col sm:flex-row gap-3">
                <div className="flex-1 relative">
                    <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                        <svg className="w-5 h-5 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"></path>
                        </svg>
                    </div>
                    <input
                        type="text"
                        placeholder="Search for a song or artist..."
                        value={searchTerm}
                        onChange={(e) => setSearchTerm(e.target.value)}
                        className="w-full pl-10 pr-4 py-3 bg-dark-750 rounded-lg border border-dark-600 focus:outline-none focus:border-accent-500 focus:ring-2 focus:ring-accent-500/20 text-white placeholder-gray-500 transition-all"
                    />
                    {searchTerm && (
                        <button
                            onClick={() => setSearchTerm('')}
                            className="absolute inset-y-0 right-0 pr-3 flex items-center text-gray-400 hover:text-white"
                        >
                            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12"></path>
                            </svg>
                        </button>
                    )}
                </div>
                <select
                    value={selectedGenre}
                    onChange={(e) => setSelectedGenre(e.target.value)}
                    className="px-4 py-3 bg-dark-750 rounded-lg border border-dark-600 focus:outline-none focus:border-accent-500 focus:ring-2 focus:ring-accent-500/20 text-white min-w-[150px] transition-all"
                >
                    <option value="">All Genres</option>
                    {genres.map(genre => (
                        <option key={genre.name} value={genre.name}>{genre.name}</option>
                    ))}
                </select>
            </div>

            {/* Action Buttons */}
            <div className="mb-6 flex flex-wrap gap-3">
                {(songs.length > 0 || allSongs.length > 0) && (
                    <button 
                        onClick={handlePlayAlbum} 
                        className="inline-flex items-center gap-2 border-2 border-green-500 text-green-400 bg-green-500/10 hover:bg-green-500/20 hover:scale-105 transition-all rounded-lg py-2.5 px-5 font-semibold"
                    >
                        <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
                            <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM9.555 7.168A1 1 0 008 8v4a1 1 0 001.555.832l3-2a1 1 0 000-1.664l-3-2z" clipRule="evenodd"></path>
                        </svg>
                        Play All ({allSongs.length > 0 ? allSongs.length : songs.length})
                    </button>
                )}
                <button 
                    onClick={async () => {
                        // Toggle: if already showing starred, reset to all songs
                        if (isStarredFilter) {
                            setIsStarredFilter(false);
                            setSongs([]);
                            setAllSongs([]);
                            setHasMore(true);
                            setRefreshKey(prev => prev + 1);
                            return;
                        }

                        try {
                            // Clear all state first to ensure clean reset
                            setSongs([]);
                            setAllSongs([]);
                            setIsLoading(true);
                            setError('');
                            setSearchTerm('');
                            setSelectedGenre('');
                            setHasMore(false);
                            setIsStarredFilter(true);
                            
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
                            setIsStarredFilter(false);
                        } finally {
                            setIsLoading(false);
                        }
                    }}
                    className={`inline-flex items-center gap-2 font-semibold py-2.5 px-5 rounded-lg transition-all ${
                        isStarredFilter
                            ? 'bg-yellow-500/20 text-yellow-400 border-2 border-yellow-400 shadow-glow'
                            : 'bg-dark-750 hover:bg-dark-700 text-yellow-400 border border-yellow-400/30 hover:border-yellow-400/50'
                    }`}
                >
                    <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
                        <path d="M9.049 2.927c.3-.921 1.603-.921 1.902 0l1.07 3.292a1 1 0 00.95.69h3.462c.969 0 1.371 1.24.588 1.81l-2.8 2.034a1 1 0 00-.364 1.118l1.07 3.292c.3.921-.755 1.688-1.54 1.118l-2.8-2.034a1 1 0 00-1.175 0l-2.8 2.034c-.784.57-1.838-.197-1.539-1.118l1.07-3.292a1 1 0 00-.364-1.118L2.98 8.72c-.783-.57-.38-1.81.588-1.81h3.461a1 1 0 00.951-.69l1.07-3.292z"></path>
                    </svg>
                    Starred Songs
                </button>
            </div>
            
            {/* Empty States */}
            {!isLoading && songs.length === 0 && (searchTerm || filter) && (
                <div className="text-center py-16">
                    <svg className="w-20 h-20 text-gray-600 mx-auto mb-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M9 19V6l12-3v13M9 19c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zm12-3c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zM9 10l12-3"></path>
                    </svg>
                    <p className="text-gray-400 text-lg">No songs found</p>
                    <p className="text-gray-500 text-sm mt-2">Try a different search term or filter</p>
                </div>
            )}

            {!isLoading && songs.length === 0 && !searchTerm && !filter && (
                <div className="text-center py-16">
                    <svg className="w-20 h-20 text-gray-600 mx-auto mb-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"></path>
                    </svg>
                    <p className="text-gray-400 text-lg">Start searching for music</p>
                    <p className="text-gray-500 text-sm mt-2">Type in the search bar to find songs</p>
                </div>
            )}

            {songs.length > 0 && (
                <div className="overflow-x-auto rounded-lg border border-dark-600">
                    <table className="min-w-full text-sm text-left text-gray-300">
                        <thead className="text-xs text-gray-400 uppercase bg-dark-750 border-b border-dark-600">
                            <tr>
                                <th className="px-4 py-3 w-12"></th>
                                <th className="px-4 py-3 w-12 text-center">⭐</th>
                                <th className="px-4 py-3">Title</th>
                                <th className="px-4 py-3 hidden sm:table-cell">Artist</th>
                                <th className="px-4 py-3 hidden md:table-cell">Album</th>
                                <th className="px-4 py-3 hidden lg:table-cell">Genre</th>
                                <th className="px-4 py-3 hidden xl:table-cell text-center">Play Count</th>
                                <th className="px-4 py-3 hidden lg:table-cell">Last Played</th>
                                <th className="px-2 sm:px-4 py-3 w-16 sm:w-48 text-right">Actions</th>
                            </tr>
                        </thead>
                        <tbody>
                            {songs.map((song, index) => {
                                const isCurrentSong = currentSong && currentSong.id === song.id;
                                const isPlaying = isCurrentSong && isAudioPlaying;
                                const isInQueue = playQueue.some(s => s.id === song.id);
                                const rowColor = isCurrentSong 
                                    ? 'bg-accent-500/10 border-l-4 border-l-accent-500' 
                                    : (index % 2 === 0 ? 'bg-dark-800' : 'bg-dark-750');
                                return (
                                    <tr ref={index === songs.length - 1 ? lastSongElementRef : null} key={`${song.id}-${index}`} className={`border-b border-dark-600 transition-all hover:bg-dark-700 ${rowColor}`}>
                                        <td className="px-4 py-4">
                                            <button 
                                                onClick={() => {
                                                    if (isCurrentSong) {
                                                        // If current song, toggle play/pause
                                                        onTogglePlayPause();
                                                    } else {
                                                        // If different song or nothing playing, play this song
                                                        onPlay(song, allSongs.length > 0 ? allSongs : songs);
                                                    }
                                                }}
                                                title={isPlaying ? "Pause song" : "Play song"}
                                                className={`p-1.5 rounded-lg border-2 transition-all hover:scale-105 flex items-center justify-center ${
                                                    isPlaying 
                                                        ? 'border-accent-500 text-accent-400 bg-accent-500/20 shadow-glow animate-pulse' 
                                                        : 'border-gray-600 text-gray-400 hover:border-accent-500 hover:text-accent-400 hover:bg-accent-500/10'
                                                }`}
                                            >
                                                {isPlaying ? (
                                                    <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
                                                        <path fillRule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zM7 8a1 1 0 012 0v4a1 1 0 11-2 0V8zm5-1a1 1 0 00-1 1v4a1 1 0 102 0V8a1 1 0 00-1-1z" clipRule="evenodd"></path>
                                                    </svg>
                                                ) : (
                                                    <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
                                                        <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM9.555 7.168A1 1 0 008 8v4a1 1 0 001.555.832l3-2a1 1 0 000-1.664l-3-2z" clipRule="evenodd"></path>
                                                    </svg>
                                                )}
                                            </button>
                                        </td>
                                        <td className="px-4 py-4 text-center">
                                            <button
                                                onClick={() => handleStarToggle(song)}
                                                className={`p-1.5 rounded-lg border-2 transition-all hover:scale-105 flex items-center justify-center ${
                                                    song.starred 
                                                        ? 'border-yellow-500 text-yellow-400 bg-yellow-500/10 shadow-glow' 
                                                        : 'border-gray-600 text-gray-500 hover:border-yellow-500 hover:text-yellow-400 hover:bg-yellow-500/10'
                                                }`}
                                                title={song.starred ? 'Remove from favorites' : 'Add to favorites'}
                                            >
                                                <svg className="w-5 h-5" fill={song.starred ? 'currentColor' : 'none'} stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24">
                                                    <path strokeLinecap="round" strokeLinejoin="round" d="M11.049 2.927c.3-.921 1.603-.921 1.902 0l1.519 4.674a1 1 0 00.95.69h4.915c.969 0 1.371 1.24.588 1.81l-3.976 2.888a1 1 0 00-.363 1.118l1.518 4.674c.3.922-.755 1.688-1.538 1.118l-3.976-2.888a1 1 0 00-1.176 0l-3.976 2.888c-.783.57-1.838-.197-1.538-1.118l1.518-4.674a1 1 0 00-.363-1.118l-3.976-2.888c-.784-.57-.38-1.81.588-1.81h4.914a1 1 0 00.951-.69l1.519-4.674z" />
                                                </svg>
                                            </button>
                                        </td>
                                        <td className={`px-4 py-4 font-medium ${isPlaying ? 'text-accent-400' : 'text-white'}`}>
                                            <div className="flex items-center gap-2">
                                                {song.title}
                                                {isPlaying && (
                                                    <span className="flex gap-0.5">
                                                        <span className="w-1 h-3 bg-accent-400 rounded-full animate-pulse"></span>
                                                        <span className="w-1 h-3 bg-accent-400 rounded-full animate-pulse" style={{animationDelay: '0.2s'}}></span>
                                                        <span className="w-1 h-3 bg-accent-400 rounded-full animate-pulse" style={{animationDelay: '0.4s'}}></span>
                                                    </span>
                                                )}
                                            </div>
                                            <div className="sm:hidden text-xs text-gray-400 mt-1">{song.artist}</div>
                                        </td>
                                        <td className="px-4 py-4 hidden sm:table-cell">{song.artist}</td>
                                        <td className="px-4 py-4 hidden md:table-cell">{song.album}</td>
                                        <td className="px-4 py-4 hidden lg:table-cell text-gray-400">{song.genre || 'Unknown'}</td>
                                        <td className="px-4 py-3 hidden xl:table-cell text-center">{song.playCount > 0 ? song.playCount : ''}</td>
                                        <td className="px-4 py-3 hidden lg:table-cell">{formatDate(song.lastPlayed)}</td>
                                        <td className="px-2 sm:px-4 py-4">
                                            {/* Desktop: Show all buttons */}
                                            <div className="hidden sm:flex items-center justify-end space-x-1 sm:space-x-2">
                                                 {isPlaylistView && !isPlaylistReadOnly && (
                                                    <>
                                                        <div className="flex flex-col -my-1">
                                                            <button onClick={() => handleMoveSong(index, 'up')} disabled={index === 0} className="p-1 text-gray-400 hover:text-white disabled:text-gray-600 disabled:cursor-not-allowed disabled:opacity-50" title="Move up">
                                                                <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 20 20"><path fillRule="evenodd" d="M14.707 12.707a1 1 0 01-1.414 0L10 9.414l-3.293 3.293a1 1 0 01-1.414-1.414l4-4a1 1 0 011.414 0l4 4a1 1 0 010 1.414z" clipRule="evenodd"></path></svg>
                                                            </button>
                                                            <button onClick={() => handleMoveSong(index, 'down')} disabled={index === allSongs.length - 1} className="p-1 text-gray-400 hover:text-white disabled:text-gray-600 disabled:cursor-not-allowed disabled:opacity-50" title="Move down">
                                                                <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 20 20"><path fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd"></path></svg>
                                                            </button>
                                                        </div>
                                                        <button onClick={() => handleDeleteSong(song.id)} title="Remove from playlist" className="p-1 text-gray-400 hover:text-red-500">
                                                            <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M15 12H9m12 0a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg>
                                                        </button>
                                                        <div className="border-l border-gray-600 h-6 mx-1"></div>
                                                    </>
                                                )}
                                                <button
                                                    onClick={() => {
                                                        if (!audioMuseUrl) {
                                                            alert('Instant Mix is not configured on the server. Ask an admin to enable AudioMuse.');
                                                            return;
                                                        }
                                                        onInstantMix(song);
                                                    }}
                                                    title="Instant Mix"
                                                    className="p-1.5 rounded-lg border-2 border-accent-500 text-accent-400 hover:bg-accent-500/10 transition-all hover:scale-105 flex items-center justify-center"
                                                >
                                                    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M13 10V3L4 14h7v7l9-11h-7z"></path></svg>
                                                </button>
                                                {isInQueue ? (
                                                    <button onClick={() => onRemoveFromQueue(song.id)} title="Remove from queue" className="p-1.5 rounded-lg border-2 border-red-500 text-red-400 hover:bg-red-500/10 transition-all hover:scale-105 flex items-center justify-center">
                                                        <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"></path></svg>
                                                    </button>
                                                ) : (
                                                    <button onClick={() => onAddToQueue(song)} title="Add to queue" className="p-1.5 rounded-lg border-2 border-green-500 text-green-400 hover:bg-green-500/10 transition-all hover:scale-105 flex items-center justify-center">
                                                        <svg className="w-5 h-5" fill="none" stroke="currentColor" strokeWidth="2"><path d="M4 6h16M4 10h16M4 14h4" /><path d="M16 12v8m-4-4h8" /></svg>
                                                    </button>
                                                )}
                                                <button onClick={() => onAddToPlaylist(song)} title="Add to playlist" className="p-1.5 rounded-lg border-2 border-purple-500 text-purple-400 hover:bg-purple-500/10 transition-all hover:scale-105 flex items-center justify-center">
                                                    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 6v6m0 0v6m0-6h6m-6 0H6"></path></svg>
                                                </button>
                                            </div>
                                            
                                            {/* Mobile: Compact vertical buttons - matching desktop style */}
                                            <div className="flex sm:hidden flex-col gap-1 items-end">
                                                <button
                                                    onClick={() => {
                                                        if (!audioMuseUrl) {
                                                            alert('Instant Mix is not configured on the server. Ask an admin to enable AudioMuse.');
                                                            return;
                                                        }
                                                        onInstantMix(song);
                                                    }}
                                                    title="Instant Mix"
                                                    className="p-1.5 rounded-lg border-2 border-accent-500 text-accent-400 hover:bg-accent-500/10 flex items-center justify-center"
                                                >
                                                    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M13 10V3L4 14h7v7l9-11h-7z"></path></svg>
                                                </button>
                                                {isInQueue ? (
                                                    <button onClick={() => onRemoveFromQueue(song.id)} title="Remove from queue" className="p-1.5 rounded-lg border-2 border-red-500 text-red-400 hover:bg-red-500/10 flex items-center justify-center">
                                                        <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"></path></svg>
                                                    </button>
                                                ) : (
                                                    <button onClick={() => onAddToQueue(song)} title="Add to queue" className="p-1.5 rounded-lg border-2 border-green-500 text-green-400 hover:bg-green-500/10 flex items-center justify-center">
                                                        <svg className="w-5 h-5" fill="none" stroke="currentColor" strokeWidth="2"><path d="M4 6h16M4 10h16M4 14h4" /><path d="M16 12v8m-4-4h8" /></svg>
                                                    </button>
                                                )}
                                                <button onClick={() => onAddToPlaylist(song)} title="Add to playlist" className="p-1.5 rounded-lg border-2 border-purple-500 text-purple-400 hover:bg-purple-500/10 flex items-center justify-center">
                                                    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 6v6m0 0v6m0-6h6m-6 0H6"></path></svg>
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
	const initial = name ? name.charAt(0).toUpperCase() : '?';
	return (
		<div className="w-full h-full bg-gradient-to-br from-dark-700 to-dark-600 flex items-center justify-center">
			<span className="text-4xl sm:text-5xl font-bold text-gray-500">{initial}</span>
		</div>
	);
};

const ArtistPlaceholder = () => (
	<div className="w-full h-full bg-gradient-to-br from-dark-700 to-dark-600 flex items-center justify-center rounded-full">
		<svg className="w-12 h-12 text-gray-500" fill="currentColor" viewBox="0 0 20 20">
			<path fillRule="evenodd" d="M10 9a3 3 0 100-6 3 3 0 000 6zm-7 9a7 7 0 1114 0H3z" clipRule="evenodd" />
		</svg>
	</div>
);

const ImageWithFallback = ({ src, placeholder, alt }) => {
    const [hasError, setHasError] = useState(false);
    const [isLoaded, setIsLoaded] = useState(false);

    // Build authenticated URL with JWT token - MUST call API!
    const imageSrc = useMemo(() => {
        if (!src) return null;
        
        // If already a string URL, use it
        if (typeof src === 'string') {
            return src;
        }
        
        // Build authenticated URL with JWT token
        const token = localStorage.getItem('token');
        if (src.useAuthFetch && token && src.url) {
            const url = new URL(src.url, window.location.origin);
            url.searchParams.set('jwt', token);
            return url.toString();
        }
        
        return src.url || null;
    }, [src]);

    // Reset state when src changes - ALWAYS load new images!
    useEffect(() => {
        setHasError(false);
        setIsLoaded(false);
    }, [imageSrc]);

    // Show placeholder if error or no src
    if (hasError || !imageSrc) {
        return <div className="w-full h-full">{placeholder}</div>;
    }

    // Show the actual image - ALWAYS render to trigger API call!
    return (
        <div className="w-full h-full relative">
            {/* Placeholder shown until image loads - NO FLICKER */}
            {!isLoaded && (
                <div className="absolute inset-0">{placeholder}</div>
            )}
            {/* Image loads and calls API */}
            <img 
                src={imageSrc}
                alt={alt}
                loading="lazy"
                onLoad={() => setIsLoaded(true)}
                onError={() => setHasError(true)}
                className={`w-full h-full object-cover transition-opacity duration-200 ${isLoaded ? 'opacity-100' : 'opacity-0'}`}
            />
        </div>
    );
};

// Memoized album card to prevent unnecessary re-renders during scroll
const AlbumCard = React.memo(({ album, isLastElement, lastAlbumElementRef, onNavigate }) => {
    return (
        <div 
            ref={isLastElement ? lastAlbumElementRef : null}
            key={album.id}
            className="group bg-dark-750 rounded-xl p-3 sm:p-4 text-center hover:bg-dark-700 card-hover cursor-pointer transition-all"
            onClick={() => onNavigate({ page: 'songs', title: album.name, filter: { albumId: album.id } })}
        >
            <div className="relative w-full bg-dark-700 rounded-lg aspect-square flex items-center justify-center mb-3 overflow-hidden shadow-md">
                <ImageWithFallback
                    src={album.coverArt ? (() => {
                        const params = new URLSearchParams({ id: album.coverArt, v: '1.16.1', c: 'AudioMuse-AI', size: '512' });
                        const url = `${API_BASE}/rest/getCoverArt.view?${params.toString()}`;
                        return { url, useAuthFetch: true };
                    })() : ''}
                    placeholder={<AlbumPlaceholder name={album.name} />}
                    alt={album.name}
                />
                {/* Play button overlay on hover */}
                <div className="absolute inset-0 bg-black bg-opacity-0 group-hover:bg-opacity-60 transition-all duration-300 flex items-center justify-center opacity-0 group-hover:opacity-100">
                    <div className="bg-accent-500 rounded-full p-3 shadow-glow transform group-hover:scale-110 transition-transform">
                        <svg className="w-8 h-8 text-white" fill="currentColor" viewBox="0 0 20 20">
                            <path d="M6.3 2.841A1.5 1.5 0 004 4.11V15.89a1.5 1.5 0 002.3 1.269l9.344-5.89a1.5 1.5 0 000-2.538L6.3 2.84z"></path>
                        </svg>
                    </div>
                </div>
            </div>
            <p className="font-semibold text-white truncate text-sm sm:text-base group-hover:text-accent-400 transition-colors">{album.name}</p>
            <p className="text-xs sm:text-sm text-gray-400 truncate mt-1">{album.artist}</p>
        </div>
    );
});

export function Albums({ credentials, filter, onNavigate }) {
    const [albums, setAlbums] = useState([]);
    const [searchTerm, setSearchTerm] = useState('');
    const [isLoading, setIsLoading] = useState(false);
    const [hasMore, setHasMore] = useState(true);
    const [genres, setGenres] = useState([]);
    const [selectedGenre, setSelectedGenre] = useState('');
    const [totalCount, setTotalCount] = useState(0);
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
    }, [searchTerm, selectedGenre]);

    // Load album counts
    useEffect(() => {
        const loadCounts = async () => {
            try {
                const counts = await getMusicCounts(selectedGenre);
                setTotalCount(counts.albums);
            } catch (err) {
                console.error('Failed to load album counts:', err);
            }
        };
        loadCounts();
    }, [selectedGenre])


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
            {/* Count Header */}
            {totalCount > 0 && (
                <div className="mb-4">
                    <h2 className="text-2xl font-bold text-white flex items-center gap-2">
                        Albums
                        <span className="text-accent-400 text-lg">({totalCount})</span>
                    </h2>
                </div>
            )}
            
            {/* Modern Search Bar */}
            <div className="mb-6 flex flex-col sm:flex-row gap-3">
                <div className="flex-1 relative">
                    <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                        <svg className="w-5 h-5 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"></path>
                        </svg>
                    </div>
                    <input
                        type="text"
                        placeholder="Search for an album or artist..."
                        value={searchTerm}
                        onChange={(e) => setSearchTerm(e.target.value)}
                        className="w-full pl-10 pr-4 py-3 bg-dark-750 rounded-lg border border-dark-600 focus:outline-none focus:border-accent-500 focus:ring-2 focus:ring-accent-500/20 text-white placeholder-gray-500 transition-all"
                    />
                    {searchTerm && (
                        <button
                            onClick={() => setSearchTerm('')}
                            className="absolute inset-y-0 right-0 pr-3 flex items-center text-gray-400 hover:text-white"
                        >
                            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12"></path>
                            </svg>
                        </button>
                    )}
                </div>
                <select
                    value={selectedGenre}
                    onChange={(e) => setSelectedGenre(e.target.value)}
                    className="px-4 py-3 bg-dark-750 rounded-lg border border-dark-600 focus:outline-none focus:border-accent-500 focus:ring-2 focus:ring-accent-500/20 text-white min-w-[150px] transition-all"
                >
                    <option value="">All Genres</option>
                    {genres.map(genre => (
                        <option key={genre.name} value={genre.name}>{genre.name}</option>
                    ))}
                </select>
            </div>
            
            {/* Album Grid */}
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-3 sm:gap-4">
                {albums.map((album, index) => (
                    <AlbumCard
                        key={album.id}
                        album={album}
                        isLastElement={index === albums.length - 1}
                        lastAlbumElementRef={lastAlbumElementRef}
                        onNavigate={onNavigate}
                    />
                ))}
            </div>
            
            {isLoading && (
                <div className="flex justify-center items-center mt-8">
                    <svg className="animate-spin h-8 w-8 text-accent-400" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                        <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                        <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                    </svg>
                </div>
            )}
            {!hasMore && albums.length > 0 && <p className="text-center text-gray-500 mt-8 text-sm">You've reached the end</p>}
        </div>
    );
}

export function Artists({ credentials, onNavigate }) {
    const [artists, setArtists] = useState([]);
    const [searchTerm, setSearchTerm] = useState('');
    const [isLoading, setIsLoading] = useState(false);
    const [hasMore, setHasMore] = useState(true);
    const [totalCount, setTotalCount] = useState(0);
    const PAGE_SIZE = 50; // Increased page size for better performance

    useEffect(() => {
        setArtists([]);
        setHasMore(true);
    }, [searchTerm]);

    // Load artist counts
    useEffect(() => {
        const loadCounts = async () => {
            try {
                const counts = await getMusicCounts('');
                setTotalCount(counts.artists);
            } catch (err) {
                console.error('Failed to load artist counts:', err);
            }
        };
        loadCounts();
    }, []);

    const loadMoreArtists = useCallback(() => {
        if (isLoading || !hasMore) return;
        setIsLoading(true);

        const fetcher = async () => {
            try {
                // Use search2.view for all cases with proper pagination
                // Use "*" as query for listing all artists (backend supports this)
                const query = searchTerm.length >= 1 ? searchTerm : '*';
                const data = await subsonicFetch('search2.view', { 
                    query: query, 
                    artistCount: PAGE_SIZE, 
                    artistOffset: artists.length,
                    albumCount: 0,  // Don't fetch albums
                    songCount: 0    // Don't fetch songs
                });
                
                const artistList = data.searchResult2?.artist || data.searchResult3?.artist || [];
                const newArtists = Array.isArray(artistList) ? artistList : [artistList].filter(Boolean);
                setArtists(prev => [...prev, ...newArtists]);
                setHasMore(newArtists.length === PAGE_SIZE);
            } catch (e) {
                console.error("Failed to fetch artists:", e);
            } finally {
                setIsLoading(false);
            }
        };

        fetcher();
    }, [searchTerm, artists.length, isLoading, hasMore]);
    
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
            {/* Count Header */}
            {totalCount > 0 && (
                <div className="mb-4">
                    <h2 className="text-2xl font-bold text-white flex items-center gap-2">
                        Artists
                        <span className="text-accent-400 text-lg">({totalCount})</span>
                    </h2>
                </div>
            )}
            
            {/* Modern Search Bar */}
            <div className="mb-6">
                <div className="relative">
                    <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                        <svg className="w-5 h-5 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"></path>
                        </svg>
                    </div>
                    <input
                        type="text"
                        placeholder="Search for an artist..."
                        value={searchTerm}
                        onChange={(e) => setSearchTerm(e.target.value)}
                        className="w-full pl-10 pr-4 py-3 bg-dark-750 rounded-lg border border-dark-600 focus:outline-none focus:border-accent-500 focus:ring-2 focus:ring-accent-500/20 text-white placeholder-gray-500 transition-all"
                    />
                    {searchTerm && (
                        <button
                            onClick={() => setSearchTerm('')}
                            className="absolute inset-y-0 right-0 pr-3 flex items-center text-gray-400 hover:text-white"
                        >
                            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12"></path>
                            </svg>
                        </button>
                    )}
                </div>
            </div>
            
            {/* Artist Grid */}
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-3 sm:gap-4">
                {artists.map((artist, index) => (
                    <div 
                        ref={index === artists.length - 1 ? lastArtistElementRef : null}
                        key={`${artist.id}-${index}`} 
                        onClick={() => onNavigate({ page: 'albums', title: artist.name, filter: artist.name })} 
                        className="group bg-dark-750 rounded-xl p-3 sm:p-4 text-center hover:bg-dark-700 card-hover flex flex-col items-center cursor-pointer">
                        <div className="relative w-28 h-28 sm:w-32 sm:h-32 rounded-full bg-gradient-to-br from-accent-500/20 to-purple-500/20 flex items-center justify-center mb-3 overflow-hidden flex-shrink-0 shadow-lg border-2 border-dark-600 group-hover:border-accent-500/50 transition-all">
                             <ImageWithFallback
                                src={artist.artistImageUrl ? (() => {
                                    const params = new URLSearchParams({ id: artist.artistImageUrl, v: '1.16.1', c: 'AudioMuse-AI', size: '512' });
                                    const url = `${API_BASE}/rest/getCoverArt.view?${params.toString()}`;
                                    return { url, useAuthFetch: true };
                                })() : ''}
                                placeholder={<ArtistPlaceholder />}
                                alt={artist.name}
                            />
                            {/* Play button overlay on hover */}
                            <div className="absolute inset-0 bg-black bg-opacity-0 group-hover:bg-opacity-60 transition-all duration-300 flex items-center justify-center opacity-0 group-hover:opacity-100 rounded-full">
                                <div className="bg-accent-500 rounded-full p-3 shadow-glow transform group-hover:scale-110 transition-transform">
                                    <svg className="w-8 h-8 text-white" fill="currentColor" viewBox="0 0 20 20">
                                        <path d="M6.3 2.841A1.5 1.5 0 004 4.11V15.89a1.5 1.5 0 002.3 1.269l9.344-5.89a1.5 1.5 0 000-2.538L6.3 2.84z"></path>
                                    </svg>
                                </div>
                            </div>
                        </div>
                        <p className="font-semibold text-white truncate w-full text-sm sm:text-base group-hover:text-accent-400 transition-colors">{artist.name}</p>
                    </div>
                ))}
            </div>
            
            {isLoading && (
                <div className="flex justify-center items-center mt-8">
                    <svg className="animate-spin h-8 w-8 text-accent-400" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                        <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                        <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                    </svg>
                </div>
            )}
            {!hasMore && artists.length > 0 && <p className="text-center text-gray-500 mt-8 text-sm">You've reached the end</p>}
        </div>
    );
}

