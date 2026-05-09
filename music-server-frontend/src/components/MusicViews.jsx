// Suggested path: music-server-frontend/src/components/MusicViews.jsx
import React, { useState, useEffect, useRef } from 'react';
import { subsonicFetch, starSong, unstarSong, getStarredSongs, getGenres, getMusicCounts, getRecentlyAdded, getMostPlayed, getRecentlyPlayed, getRadioSeed, getSimilarArtists, getAudioMuseSimilarArtists } from '../api';

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
    const [allSongs, setAllSongs] = useState([]); // All loaded songs from backend
    const [searchTerm, setSearchTerm] = useState('');
    const [isLoading, setIsLoading] = useState(false);
    const [error, setError] = useState('');
    const [offset, setOffset] = useState(0);
    const [refreshKey, setRefreshKey] = useState(0);
    const [genres, setGenres] = useState([]);
    const [selectedGenre, setSelectedGenre] = useState('');
    const [playlistOwner, setPlaylistOwner] = useState(null);
    const [discoveryView, setDiscoveryView] = useState('all'); // 'all', 'recent', 'popular', 'history'
    const [baseTotalCount, setBaseTotalCount] = useState(0); // Total count of ALL songs (unfiltered)
    const [totalCount, setTotalCount] = useState(0); // Count for current view (filtered by search/genre)
    const [isStarredFilter, setIsStarredFilter] = useState(false);
    const [radioFetching, setRadioFetching] = useState(false);
    const radioFetchedRef = useRef(false); // Track if we already fetched more songs

    const isPlaylistView = !!filter?.playlistId;
    const isRadioView = !!filter?.isRadio;
    const PAGE_SIZE = 50;

    // For discovery views, load 200 songs immediately instead of paginating
    const DISCOVERY_LOAD_SIZE = 200;
    const DISPLAY_BATCH_SIZE = 200; // Show 200 songs at a time in the UI

    // Check if playlist is read-only (owned by another user)
    const isPlaylistReadOnly = isPlaylistView && playlistOwner && credentials?.username && playlistOwner !== credentials.username;

    // Load genres on component mount - only genres with actual songs
    useEffect(() => {
        const loadGenres = async () => {
            try {
                const data = await getGenres();
                const genreList = data.genres?.genre || [];
                const allGenres = Array.isArray(genreList) ? genreList : [genreList].filter(Boolean);

                // Split semicolon-separated genres and remove duplicates
                const individualGenres = [];
                allGenres.forEach(genre => {
                    const genreName = genre.value || genre.name;
                    if (genreName) {
                        const splitGenres = genreName.split(';').map(g => g.trim()).filter(g => g);
                        splitGenres.forEach(g => {
                            if (!individualGenres.find(existing => existing.name === g)) {
                                individualGenres.push({ name: g });
                            }
                        });
                    }
                });

                // Validate each genre has at least one song before showing
                const validGenres = [];
                for (const genre of individualGenres) {
                    try {
                        const songData = await subsonicFetch('getSongsByGenre.view', {
                            genre: genre.name,
                            size: 1,
                            offset: 0
                        });
                        const songs = songData.songsByGenre?.song;
                        if (songs) {
                            validGenres.push(genre);
                        }
                    } catch (err) {
                        console.warn(`Genre "${genre.name}" has no songs or fetch failed`);
                    }
                }

                setGenres(validGenres.sort((a, b) => a.name.localeCompare(b.name)));
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
        setOffset(0);
        setTotalCount(0); // Reset count when filters change
        setDiscoveryView('all'); // Reset discovery view on filter/genre change
        radioFetchedRef.current = false; // Reset radio fetch tracker
    }, [searchTerm, filter, refreshKey, selectedGenre]);

    // Radio Auto-Rerun: Fetch more songs when approaching end of queue
    useEffect(() => {
        if (!isRadioView || !filter?.radioId || radioFetching || radioFetchedRef.current) return;
        if (!currentSong || playQueue.length === 0) return;

        // Find current song index in play queue
        const currentIndex = playQueue.findIndex(s => s.id === currentSong.id);
        if (currentIndex === -1) return;

        // When we reach 20 songs before the end, fetch more (180/200 = 90%)
        const songsRemaining = playQueue.length - currentIndex;
        if (songsRemaining <= 20) {
            console.log(`🔄 Radio auto-rerun triggered: ${songsRemaining} songs remaining`);

            const fetchMoreRadioSongs = async () => {
                setRadioFetching(true);
                radioFetchedRef.current = true;

                try {
                    // Get the radio seed configuration
                    const seedData = await getRadioSeed(filter.radioId);
                    const items = JSON.parse(seedData.seed_songs);

                    // Run alchemy with n=200
                    const alchemyPayload = {
                        items,
                        n: 200,
                        temperature: seedData.temperature,
                        subtract_distance: seedData.subtract_distance
                    };

                    const response = await fetch('/api/alchemy', {
                        method: 'POST',
                        headers: {
                            'Content-Type': 'application/json',
                            'Authorization': `Bearer ${localStorage.getItem('token')}`
                        },
                        body: JSON.stringify(alchemyPayload)
                    });

                    const data = await response.json();

                    if (!response.ok || data.error) {
                        console.error('Radio auto-rerun failed:', data.error);
                        setRadioFetching(false);
                        return;
                    }

                    // Map results and add to queue
                    const newSongs = (data.results || []).map(r => ({
                        id: r.item_id || r.id || r.songId || '',
                        title: r.title || r.name || '',
                        artist: r.author || r.artist || r.creator || ''
                    }));

                    console.log(`✨ Radio auto-rerun complete: ${newSongs.length} new songs added to queue`);

                    // Add new songs to the queue
                    newSongs.forEach(song => onAddToQueue(song));

                } catch (err) {
                    console.error('Radio auto-rerun error:', err);
                } finally {
                    setRadioFetching(false);
                    // Reset after delay so we can fetch again on next cycle
                    setTimeout(() => { radioFetchedRef.current = false; }, 10000);
                }
            };

            fetchMoreRadioSongs();
        }
    }, [isRadioView, filter?.radioId, currentSong, playQueue, radioFetching, onAddToQueue]);

    // Load base total (ALL songs unfiltered) - only once
    useEffect(() => {
        const loadBaseTotal = async () => {
            try {
                if (!filter && !isStarredFilter && discoveryView === 'all' && !searchTerm && !selectedGenre) {
                    const counts = await getMusicCounts('');
                    setBaseTotalCount(counts.songs);
                }
            } catch (err) {
                console.error('Failed to load base total:', err);
            }
        };
        loadBaseTotal();
    }, []);

    // Load view-specific counts when search/genre/filter changes
    useEffect(() => {
        const loadViewCounts = async () => {
            try {
                // When searching - count comes from search result
                if (searchTerm.length >= 3) return;

                // When in discovery view with non-all, it loads all at once
                if (discoveryView !== 'all') return;

                // Otherwise load base counts for current view
                if (!filter && !isStarredFilter && selectedGenre) {
                    const counts = await getMusicCounts(selectedGenre);
                    setTotalCount(counts.songs);
                }
            } catch (err) {
                console.error('Failed to load view counts:', err);
            }
        };
        loadViewCounts();
    }, [searchTerm, selectedGenre, filter, discoveryView, isStarredFilter]);

    // Load songs when offset or filters change
    useEffect(() => {
        const load = async () => {
            setIsLoading(true);
            setError('');

            try {
                // Handle "All Songs" view - fetch songs with pagination using search3
                if (!filter && !searchTerm && discoveryView === 'all' && !selectedGenre && !isStarredFilter) {
                    const data = await subsonicFetch('search3.view', {
                        query: ' ',
                        songCount: DISPLAY_BATCH_SIZE,
                        songOffset: offset,
                        artistCount: 0,
                        albumCount: 0
                    });
                    const newSongs = data.searchResult3?.song || [];
                    const songsArray = Array.isArray(newSongs) ? newSongs : [newSongs].filter(Boolean);

                    setSongs(prev => offset === 0 ? songsArray : [...prev, ...songsArray]);
                    setAllSongs(prev => offset === 0 ? songsArray : [...prev, ...songsArray]);
                    return;
                }

                // Handle discovery views - load all at once (up to 200)
                if (!filter && !searchTerm && discoveryView !== 'all') {
                    if (offset !== 0) return; // Only load once for discovery views

                    let newSongs = [];
                    try {
                        if (discoveryView === 'recent') {
                            newSongs = await getRecentlyAdded(DISCOVERY_LOAD_SIZE, 0, selectedGenre);
                        } else if (discoveryView === 'popular') {
                            newSongs = await getMostPlayed(DISCOVERY_LOAD_SIZE, 0, selectedGenre);
                        } else if (discoveryView === 'history') {
                            newSongs = await getRecentlyPlayed(DISCOVERY_LOAD_SIZE, 0, selectedGenre);
                        }
                        const songsArray = newSongs || [];
                        setSongs(songsArray);
                        setAllSongs(songsArray);
                    } catch (err) {
                        setError('Failed to load songs: ' + err.message);
                    } finally {
                        setIsLoading(false);
                    }
                    return;
                }

                if (searchTerm.length >= 3) {
                    if (offset !== 0) return; // Load search results only once

                    const data = await subsonicFetch('search2.view', { query: searchTerm, songCount: DISCOVERY_LOAD_SIZE, songOffset: 0 });
                    const songList = data.searchResult2?.song || data.searchResult3?.song || [];
                    let newSongs = Array.isArray(songList) ? songList : [songList].filter(Boolean);

                    const totalFromSearch = data.searchResult2?.songCount || data.searchResult3?.songCount || 0;
                    if (totalFromSearch > 0) {
                        setTotalCount(totalFromSearch);
                    }

                    if (selectedGenre) {
                        newSongs = newSongs.filter(song => {
                            if (!song.genre) return false;
                            const genres = song.genre.split(';').map(g => g.trim());
                            return genres.includes(selectedGenre) || genres.some(g => g.toLowerCase() === selectedGenre.toLowerCase());
                        });
                    }

                    setSongs(newSongs);
                    setAllSongs(newSongs);
                    return;
                }

                if (searchTerm.length > 0 && searchTerm.length < 3) {
                    setSongs([]);
                    setAllSongs([]);
                    return;
                }

                // Priority 1: Genre filter (highest priority, no caching)
                if (selectedGenre && !filter && !searchTerm && offset === 0) {
                    const data = await subsonicFetch('getSongsByGenre.view', {
                        genre: selectedGenre,
                        size: DISCOVERY_LOAD_SIZE,
                        offset: 0
                    });
                    const newSongs = data.songsByGenre?.song || [];
                    const songsArray = Array.isArray(newSongs) ? newSongs : [newSongs].filter(Boolean);

                    if (songsArray.length === 0) {
                        setError(`No songs found for genre: ${selectedGenre}`);
                    }

                    setSongs(songsArray);
                    setAllSongs(songsArray);
                    return;
                }

                // Priority 2: Load cached data if available
                let baseList = allSongs;
                if (baseList.length === 0 && !searchTerm) {
                    let songList = [];
                    if (filter?.preloadedSongs) songList = filter.preloadedSongs;
                    else if (filter?.type === 'clap-search' && filter?.results) {
                        songList = filter.results;
                    } else if (filter?.similarToSongId) {
                        const data = await subsonicFetch('getSimilarSongs.view', { id: filter.similarToSongId, count: PAGE_SIZE });
                        songList = data.directory?.song || [];
                    } else if (filter) {
                        const endpoint = filter.albumId ? 'getAlbum.view' : 'getPlaylist.view';
                        const idParam = filter.albumId || filter.playlistId;
                        if (idParam) {
                            const data = await subsonicFetch(endpoint, { id: idParam });
                            const songContainer = data.album || data.directory;

                            if (filter.playlistId && data.playlist) {
                                setPlaylistOwner(data.playlist.owner || null);
                            }

                            if (songContainer?.song) songList = Array.isArray(songContainer.song) ? songContainer.song : [songContainer.song];
                        }
                    }

                    baseList = Array.isArray(songList) ? songList : [songList].filter(Boolean);

                    if (selectedGenre && (filter?.albumId || filter?.playlistId || filter?.preloadedSongs)) {
                        baseList = baseList.filter(song => {
                            if (!song.genre) return false;
                            const genres = song.genre.split(';').map(g => g.trim());
                            return genres.includes(selectedGenre) || genres.some(g => g.toLowerCase() === selectedGenre.toLowerCase());
                        });
                    }

                    setAllSongs(baseList);
                }

                setSongs(baseList);
            } catch (e) {
                console.error("Failed to fetch songs:", e);
                setError(e.message || "Failed to fetch songs.");
            } finally {
                setIsLoading(false);
            }
        };

        load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [offset, filter, searchTerm, selectedGenre, credentials?.username, discoveryView, isStarredFilter]);


    // Infinite scroll observer for "All Songs" view
    const observerRef = useRef();
    const lastSongElementRef = useRef();

    useEffect(() => {
        // Only set up observer for "All Songs" view
        if (discoveryView !== 'all' || filter || searchTerm || selectedGenre || isStarredFilter || isLoading) {
            if (observerRef.current) observerRef.current.disconnect();
            return;
        }

        if (observerRef.current) observerRef.current.disconnect();

        observerRef.current = new IntersectionObserver(([entry]) => {
            if (entry.isIntersecting && songs.length < baseTotalCount) {
                setOffset(prev => prev + DISPLAY_BATCH_SIZE);
            }
        });

        if (lastSongElementRef.current) {
            observerRef.current.observe(lastSongElementRef.current);
        }

        return () => observerRef.current?.disconnect();
    }, [isLoading, songs.length, baseTotalCount, discoveryView, filter, searchTerm, selectedGenre, isStarredFilter])

    const handlePlayAlbum = () => {
        // Always use the full list of songs
        const listToPlay = allSongs.length > 0 ? allSongs : songs;
        if (listToPlay.length > 0) {
            // Refresh queue and start playing (even if paused)
            onPlay(listToPlay[0], listToPlay);
        }
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

    const handleSimilarArtists = async (song) => {
        if (!song || !song.artist) {
            setError('No artist found for this song');
            return;
        }

        try {
            setIsLoading(true);
            setError('');

            const artistName = song.artist;
            console.log(`Finding similar artists to "${artistName}"...`);

            // Get similar artists from AudioMuse
            const similarData = await getAudioMuseSimilarArtists(artistName, 10);
            let similarArtists = similarData.similar_artists || [];
            similarArtists = Array.isArray(similarArtists) ? similarArtists : [similarArtists].filter(Boolean);

            if (similarArtists.length === 0) {
                setError('No similar artists found');
                setIsLoading(false);
                return;
            }

            console.log(`Found ${similarArtists.length} similar artists`);

            // Get songs from all similar artists using Subsonic search
            const allArtistSongs = [];
            for (const artist of similarArtists) {
                try {
                    const songData = await subsonicFetch('search2.view', {
                        query: artist,
                        artistCount: 0,
                        albumCount: 0,
                        songCount: 50
                    });
                    const songList = songData.searchResult2?.song || [];
                    const songArray = Array.isArray(songList) ? songList : [songList].filter(Boolean);
                    allArtistSongs.push(...songArray);
                } catch (err) {
                    console.warn(`Failed to load songs for artist ${artist}`, err);
                }
            }

            // Cap at 200 songs
            const finalSongs = allArtistSongs.slice(0, 200);

            if (finalSongs.length === 0) {
                setError('No songs found from similar artists');
                setIsLoading(false);
                return;
            }

            setSongs(finalSongs);
            setAllSongs(finalSongs);
            setSearchTerm('');
            setSelectedGenre('');
            setOffset(0);

            console.log(`✨ Loaded ${finalSongs.length} songs from ${similarArtists.length} similar artists for "${artistName}"`);
        } catch (err) {
            console.error('Failed to load similar artists:', err);
            setError(err.message || 'Failed to load similar artists');
        } finally {
            setIsLoading(false);
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

            {/* Radio auto-fetch indicator */}
            {isRadioView && radioFetching && (
                <div className="bg-teal-500/10 border border-teal-500/50 rounded-lg p-4 mb-6 animate-fade-in">
                    <p className="text-teal-400 flex items-center gap-2">
                        <svg className="animate-spin h-5 w-5" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                            <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                            <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                        </svg>
                        📻 Radio generating more songs... (200 tracks incoming!)
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
                                setOffset(0);
                                setSearchTerm('');
                                setIsStarredFilter(false); // Reset starred filter
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
                                <span className={discoveryView === 'all' ? 'inline' : 'hidden sm:inline'}>All Songs {baseTotalCount > 0 && `(${baseTotalCount})`}</span>
                            </span>
                        </button>
                        <button
                            onClick={async () => {
                                setDiscoveryView('recent');
                                setSongs([]);
                                setAllSongs([]);
                                setOffset(0);
                                setIsLoading(true);
                                setIsStarredFilter(false); // Reset starred filter
                                try {
                                    const newSongs = await getRecentlyAdded(DISCOVERY_LOAD_SIZE, 0, selectedGenre);
                                    const songsArray = newSongs || [];
                                    setAllSongs(songsArray);
                                    setSongs(songsArray);  // Show all songs immediately
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
                                setOffset(0);
                                setIsLoading(true);
                                setIsStarredFilter(false); // Reset starred filter
                                try {
                                    const newSongs = await getMostPlayed(DISCOVERY_LOAD_SIZE, 0, selectedGenre);
                                    const songsArray = newSongs || [];
                                    setAllSongs(songsArray);
                                    setSongs(songsArray);  // Show all songs immediately
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
                                setOffset(0);
                                setIsLoading(true);
                                setIsStarredFilter(false); // Reset starred filter
                                try {
                                    const newSongs = await getRecentlyPlayed(DISCOVERY_LOAD_SIZE, 0, selectedGenre);
                                    const songsArray = newSongs || [];
                                    setAllSongs(songsArray);
                                    setSongs(songsArray);  // Show all songs immediately
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
                        Play All ({Math.min(allSongs.length > 0 ? allSongs.length : songs.length, 200)})
                    </button>
                )}
                <button
                    onClick={async () => {
                        // Toggle: if already showing starred, reset to all songs
                        if (isStarredFilter) {
                            setIsStarredFilter(false);
                            setSongs([]);
                            setAllSongs([]);
                            setOffset(0);
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
                            setOffset(0);
                            setIsStarredFilter(true);
                            setDiscoveryView('all'); // Switch to "All Songs" tab

                            const data = await getStarredSongs();
                            const starredSongs = data.starred2?.song;

                            // Handle empty/missing starred songs properly
                            let songList = [];
                            if (starredSongs) {
                                songList = Array.isArray(starredSongs) ? starredSongs : [starredSongs];
                                // Filter out any invalid entries (null, undefined, or objects without id)
                                songList = songList.filter(s => s && s.id);
                            }

                            setAllSongs(songList);
                            // Show all starred songs at once (no pagination) so displayed items match the total
                            setSongs(songList);
                            // Ensure total count reflects unique starred songs (deduplicated list)
                            setTotalCount(songList.length);
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
            {!isLoading && songs.length === 0 && (searchTerm || filter || isStarredFilter) && (
                <div className="text-center py-16">
                    <svg className="w-20 h-20 text-gray-600 mx-auto mb-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M9 19V6l12-3v13M9 19c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zm12-3c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zM9 10l12-3"></path>
                    </svg>
                    <p className="text-gray-400 text-lg">{isStarredFilter ? 'No starred songs' : 'No songs found'}</p>
                    <p className="text-gray-500 text-sm mt-2">{isStarredFilter ? 'Star some songs to see them here' : 'Try a different search term or filter'}</p>
                </div>
            )}

            {!isLoading && songs.length === 0 && !searchTerm && !filter && !isStarredFilter && (
                <div className="text-center py-16">
                    <svg className="w-20 h-20 text-gray-600 mx-auto mb-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"></path>
                    </svg>
                    <p className="text-gray-400 text-lg">Start searching for music</p>
                    <p className="text-gray-500 text-sm mt-2">Type in the search bar to find songs</p>
                </div>
            )}

            {songs.length > 0 && (
                <>
                    {/* Mobile Card View */}
                    <div className="lg:hidden space-y-3 mb-6">
                        {songs.map((song, index) => {
                            const isCurrentSong = currentSong && currentSong.id === song.id;
                            const isPlaying = isCurrentSong && isAudioPlaying;
                            const isInQueue = playQueue.some(s => s.id === song.id);
                            return (
                                <div
                                    ref={index === songs.length - 1 ? lastSongElementRef : null}
                                    key={`${song.id}-${index}`}
                                    className={`rounded-lg p-3 border border-dark-600 transition-all ${
                                        isCurrentSong ? 'bg-accent-500/10 border-accent-500' : 'bg-dark-750 hover:bg-dark-700'
                                    }`}
                                >
                                    {/* Title */}
                                    <div className={`font-semibold mb-1 text-sm ${isPlaying ? 'text-accent-400' : 'text-white'}`}>
                                        {song.title}
                                    </div>

                                    {/* Artist */}
                                    <div className="text-xs text-gray-400 mb-3">
                                        {song.artist}
                                    </div>

                                    {/* All 6 buttons in 3 rows × 2 columns */}
                                    <div className="grid grid-cols-2 gap-2">
                                        {/* Play Button */}
                                        <div className="relative group">
                                            <button
                                                onClick={() => {
                                                    if (isCurrentSong) {
                                                        onTogglePlayPause();
                                                    } else {
                                                        onPlay(song, [song]);
                                                    }
                                                }}
                                                className={`w-full p-2 rounded-lg border-2 transition-all flex items-center justify-center gap-1 ${
                                                    isPlaying
                                                        ? 'border-accent-500 text-accent-400 bg-accent-500/20'
                                                        : 'border-gray-600 text-gray-400 hover:border-accent-500 hover:text-accent-400'
                                                }`}
                                                title="Play Song"
                                            >
                                                {isPlaying ? (
                                                    <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
                                                        <path fillRule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zM7 8a1 1 0 012 0v4a1 1 0 11-2 0V8zm5-1a1 1 0 00-1 1v4a1 1 0 102 0V8a1 1 0 00-1-1z" clipRule="evenodd"></path>
                                                    </svg>
                                                ) : (
                                                    <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
                                                        <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM9.555 7.168A1 1 0 008 8v4a1 1 0 001.555.832l3-2a1 1 0 000-1.664l-3-2z" clipRule="evenodd"></path>
                                                    </svg>
                                                )}
                                                <span className="text-xs">Play</span>
                                            </button>
                                            <div className="hidden lg:group-hover:block absolute bottom-full mb-2 left-0 bg-gray-900 text-white text-xs rounded px-2 py-1 whitespace-nowrap border border-gray-700 z-10">
                                                Play Song
                                            </div>
                                        </div>

                                        {/* Star Button */}
                                        <div className="relative group">
                                            <button
                                                onClick={() => handleStarToggle(song)}
                                                className={`w-full p-2 rounded-lg border-2 transition-all flex items-center justify-center gap-1 ${
                                                    song.starred
                                                        ? 'border-yellow-500 text-yellow-400 bg-yellow-500/10'
                                                        : 'border-gray-600 text-gray-500 hover:border-yellow-500 hover:text-yellow-400'
                                                }`}
                                                title="Add to Favorites"
                                            >
                                                <svg className="w-4 h-4" fill={song.starred ? 'currentColor' : 'none'} stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24">
                                                    <path strokeLinecap="round" strokeLinejoin="round" d="M11.049 2.927c.3-.921 1.603-.921 1.902 0l1.519 4.674a1 1 0 00.95.69h4.915c.969 0 1.371 1.24.588 1.81l-3.976 2.888a1 1 0 00-.363 1.118l1.518 4.674c.3.922-.755 1.688-1.538 1.118l-3.976-2.888a1 1 0 00-1.176 0l-3.976 2.888c-.783.57-1.838-.197-1.538-1.118l1.518-4.674a1 1 0 00-.363-1.118l-3.976-2.888c-.784-.57-.38-1.81.588-1.81h4.914a1 1 0 00.951-.69l1.519-4.674z" />
                                                </svg>
                                                <span className="text-xs">Star</span>
                                            </button>
                                            <div className="hidden lg:group-hover:block absolute bottom-full mb-2 left-0 bg-gray-900 text-white text-xs rounded px-2 py-1 whitespace-nowrap border border-gray-700 z-10">
                                                Favorites
                                            </div>
                                        </div>

                                        {/* Similar Songs Button */}
                                        <div className="relative group">
                                            <button
                                                onClick={() => {
                                                    if (!audioMuseUrl) {
                                                        alert('Instant Mix is not configured on the server. Ask an admin to enable AudioMuse.');
                                                        return;
                                                    }
                                                    onInstantMix(song);
                                                }}
                                                className="w-full p-2 rounded-lg border-2 border-accent-500 text-accent-400 hover:bg-accent-500/10 flex items-center justify-center gap-1 transition-all"
                                                title="Similar Songs"
                                            >
                                                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M13 10V3L4 14h7v7l9-11h-7z"></path></svg>
                                                <span className="text-xs">Similar</span>
                                            </button>
                                            <div className="hidden lg:group-hover:block absolute bottom-full mb-2 left-0 bg-gray-900 text-white text-xs rounded px-2 py-1 whitespace-nowrap border border-gray-700 z-10">
                                                Similar Songs
                                            </div>
                                        </div>

                                        {/* Similar Artists Button */}
                                        <div className="relative group">
                                            <button
                                                onClick={() => handleSimilarArtists(song)}
                                                className="w-full p-2 rounded-lg border-2 border-blue-500 text-blue-400 hover:bg-blue-500/10 flex items-center justify-center gap-1 transition-all"
                                                title="Similar Artists"
                                            >
                                                <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
                                                    <path d="M9 6a3 3 0 11-6 0 3 3 0 016 0zM17 6a3 3 0 11-6 0 3 3 0 016 0zM12.93 11a6 6 0 00-11.86 0v2a1 1 0 001 1h9.86a1 1 0 001-1v-2z"></path>
                                                </svg>
                                                <span className="text-xs">Artists</span>
                                            </button>
                                            <div className="hidden lg:group-hover:block absolute bottom-full mb-2 left-0 bg-gray-900 text-white text-xs rounded px-2 py-1 whitespace-nowrap border border-gray-700 z-10">
                                                Similar Artists
                                            </div>
                                        </div>

                                        {/* Queue/Remove Button */}
                                        <div className="relative group">
                                            {isInQueue ? (
                                                <button
                                                    onClick={() => onRemoveFromQueue(song.id)}
                                                    className="w-full p-2 rounded-lg border-2 border-red-500 text-red-400 hover:bg-red-500/10 flex items-center justify-center gap-1 transition-all"
                                                    title="Remove from Queue"
                                                >
                                                    <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"></path></svg>
                                                    <span className="text-xs">Remove</span>
                                                </button>
                                            ) : (
                                                <button
                                                    onClick={() => onAddToQueue(song)}
                                                    className="w-full p-2 rounded-lg border-2 border-green-500 text-green-400 hover:bg-green-500/10 flex items-center justify-center gap-1 transition-all"
                                                    title="Add to Queue"
                                                >
                                                    <svg className="w-4 h-4" fill="none" stroke="currentColor" strokeWidth="2"><path d="M4 6h16M4 10h16M4 14h4" /><path d="M16 12v8m-4-4h8" /></svg>
                                                    <span className="text-xs">Queue</span>
                                                </button>
                                            )}
                                            <div className="hidden lg:group-hover:block absolute bottom-full mb-2 left-0 bg-gray-900 text-white text-xs rounded px-2 py-1 whitespace-nowrap border border-gray-700 z-10">
                                                {isInQueue ? 'Remove from Queue' : 'Add to Queue'}
                                            </div>
                                        </div>

                                        {/* Add to Playlist Button */}
                                        <div className="relative group">
                                            <button
                                                onClick={() => onAddToPlaylist(song)}
                                                className="w-full p-2 rounded-lg border-2 border-purple-500 text-purple-400 hover:bg-purple-500/10 flex items-center justify-center gap-1 transition-all"
                                                title="Add to Playlist"
                                            >
                                                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 6v6m0 0v6m0-6h6m-6 0H6"></path></svg>
                                                <span className="text-xs">Playlist</span>
                                            </button>
                                            <div className="hidden lg:group-hover:block absolute bottom-full mb-2 left-0 bg-gray-900 text-white text-xs rounded px-2 py-1 whitespace-nowrap border border-gray-700 z-10">
                                                Add to Playlist
                                            </div>
                                        </div>
                                    </div>
                                </div>
                            );
                        })}
                    </div>

                    {/* Desktop Table View */}
                    <div className="hidden lg:block overflow-x-auto rounded-lg border border-dark-600">
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
                                                        // Play ONLY this single song (not the whole list)
                                                        onPlay(song, [song]);
                                                    }
                                                }}
                                                title={isPlaying ? "Pause Song" : "Play Song"}
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
                                        <td className="px-2 sm:px-4 py-4 text-center flex-shrink-0">
                                            <div className="relative group">
                                                <button
                                                    onClick={() => handleStarToggle(song)}
                                                    className={`p-1.5 rounded-lg border-2 transition-all hover:scale-105 flex items-center justify-center ${
                                                        song.starred
                                                            ? 'border-yellow-500 text-yellow-400 bg-yellow-500/10 shadow-glow'
                                                            : 'border-gray-600 text-gray-500 hover:border-yellow-500 hover:text-yellow-400 hover:bg-yellow-500/10'
                                                    }`}
                                                >
                                                    <svg className="w-4 sm:w-5 h-4 sm:h-5" fill={song.starred ? 'currentColor' : 'none'} stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24">
                                                        <path strokeLinecap="round" strokeLinejoin="round" d="M11.049 2.927c.3-.921 1.603-.921 1.902 0l1.519 4.674a1 1 0 00.95.69h4.915c.969 0 1.371 1.24.588 1.81l-3.976 2.888a1 1 0 00-.363 1.118l1.518 4.674c.3.922-.755 1.688-1.538 1.118l-3.976-2.888a1 1 0 00-1.176 0l-3.976 2.888c-.783.57-1.838-.197-1.538-1.118l1.518-4.674a1 1 0 00-.363-1.118l-3.976-2.888c-.784-.57-.38-1.81.588-1.81h4.914a1 1 0 00.951-.69l1.519-4.674z" />
                                                    </svg>
                                                </button>
                                                <div className="hidden group-hover:block absolute bottom-full mb-2 right-0 bg-gray-900 text-white text-xs rounded px-2 py-1 whitespace-nowrap border border-gray-700">
                                                    Favorites
                                                </div>
                                            </div>
                                        </td>
                                        <td className={`px-2 sm:px-4 py-4 font-medium flex-grow ${isPlaying ? 'text-accent-400' : 'text-white'}`}>
                                            <div className="flex items-center gap-1 sm:gap-2 min-w-0">
                                                <span className="truncate text-sm sm:text-base">{song.title}</span>
                                                {isPlaying && (
                                                    <span className="flex gap-0.5 flex-shrink-0">
                                                        <span className="w-1 h-2 sm:h-3 bg-accent-400 rounded-full animate-pulse"></span>
                                                        <span className="w-1 h-2 sm:h-3 bg-accent-400 rounded-full animate-pulse" style={{animationDelay: '0.2s'}}></span>
                                                        <span className="w-1 h-2 sm:h-3 bg-accent-400 rounded-full animate-pulse" style={{animationDelay: '0.4s'}}></span>
                                                    </span>
                                                )}
                                            </div>
                                            <div className="text-xs sm:text-sm text-gray-400 truncate">{song.artist}</div>
                                        </td>
                                        <td className="px-4 py-4 hidden sm:table-cell">{song.artist}</td>
                                        <td className="px-4 py-4 hidden md:table-cell">{song.album}</td>
                                        <td className="px-4 py-4 hidden lg:table-cell text-gray-400">{song.genre || 'Unknown'}</td>
                                        <td className="px-4 py-3 hidden xl:table-cell text-center">{song.playCount > 0 ? song.playCount : ''}</td>
                                        <td className="px-4 py-3 hidden lg:table-cell">{formatDate(song.lastPlayed)}</td>
                                        <td className="px-1 sm:px-4 py-4 flex-shrink-0 w-16 sm:w-auto">
                                            {/* Desktop: Show all buttons */}
                                            <div className="hidden sm:flex items-center justify-end space-x-1 sm:space-x-2 gap-0.5 flex-wrap">
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
                                                <div className="relative group">
                                                    <button
                                                        onClick={() => {
                                                            if (!audioMuseUrl) {
                                                                alert('Instant Mix is not configured on the server. Ask an admin to enable AudioMuse.');
                                                                return;
                                                            }
                                                            onInstantMix(song);
                                                        }}
                                                        className="p-1.5 rounded-lg border-2 border-accent-500 text-accent-400 hover:bg-accent-500/10 transition-all hover:scale-105 flex items-center justify-center"
                                                    >
                                                        <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M13 10V3L4 14h7v7l9-11h-7z"></path></svg>
                                                    </button>
                                                    <div className="hidden group-hover:block absolute bottom-full mb-2 right-0 bg-gray-900 text-white text-xs rounded px-2 py-1 whitespace-nowrap border border-gray-700">
                                                        Similar Songs
                                                    </div>
                                                </div>
                                                <div className="relative group">
                                                    <button
                                                        onClick={() => handleSimilarArtists(song)}
                                                        className="p-1.5 rounded-lg border-2 border-blue-500 text-blue-400 hover:bg-blue-500/10 transition-all hover:scale-105 flex items-center justify-center"
                                                    >
                                                        <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
                                                            <path d="M9 6a3 3 0 11-6 0 3 3 0 016 0zM17 6a3 3 0 11-6 0 3 3 0 016 0zM12.93 11a6 6 0 00-11.86 0v2a1 1 0 001 1h9.86a1 1 0 001-1v-2z"></path>
                                                        </svg>
                                                    </button>
                                                    <div className="hidden group-hover:block absolute bottom-full mb-2 right-0 bg-gray-900 text-white text-xs rounded px-2 py-1 whitespace-nowrap border border-gray-700">
                                                        Similar Artists
                                                    </div>
                                                </div>
                                                {isInQueue ? (
                                                    <div className="relative group">
                                                        <button onClick={() => onRemoveFromQueue(song.id)} className="p-1.5 rounded-lg border-2 border-red-500 text-red-400 hover:bg-red-500/10 transition-all hover:scale-105 flex items-center justify-center">
                                                            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"></path></svg>
                                                        </button>
                                                        <div className="hidden group-hover:block absolute bottom-full mb-2 right-0 bg-gray-900 text-white text-xs rounded px-2 py-1 whitespace-nowrap border border-gray-700">
                                                            Remove from Queue
                                                        </div>
                                                    </div>
                                                ) : (
                                                    <div className="relative group">
                                                        <button onClick={() => onAddToQueue(song)} className="p-1.5 rounded-lg border-2 border-green-500 text-green-400 hover:bg-green-500/10 transition-all hover:scale-105 flex items-center justify-center">
                                                            <svg className="w-5 h-5" fill="none" stroke="currentColor" strokeWidth="2"><path d="M4 6h16M4 10h16M4 14h4" /><path d="M16 12v8m-4-4h8" /></svg>
                                                        </button>
                                                        <div className="hidden group-hover:block absolute bottom-full mb-2 right-0 bg-gray-900 text-white text-xs rounded px-2 py-1 whitespace-nowrap border border-gray-700">
                                                            Add to Queue
                                                        </div>
                                                    </div>
                                                )}
                                                <div className="relative group">
                                                    <button onClick={() => onAddToPlaylist(song)} className="p-1.5 rounded-lg border-2 border-purple-500 text-purple-400 hover:bg-purple-500/10 transition-all hover:scale-105 flex items-center justify-center">
                                                        <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 6v6m0 0v6m0-6h6m-6 0H6"></path></svg>
                                                    </button>
                                                    <div className="hidden group-hover:block absolute bottom-full mb-2 right-0 bg-gray-900 text-white text-xs rounded px-2 py-1 whitespace-nowrap border border-gray-700">
                                                        Add to Playlist
                                                    </div>
                                                </div>
                                            </div>

                                            {/* Mobile: Compact vertical buttons in a grid */}
                                            <div className="flex sm:hidden flex-wrap gap-0.5 items-center justify-end">
                                                <div className="relative group">
                                                    <button
                                                        onClick={() => {
                                                            if (!audioMuseUrl) {
                                                                alert('Instant Mix is not configured on the server. Ask an admin to enable AudioMuse.');
                                                                return;
                                                            }
                                                            onInstantMix(song);
                                                        }}
                                                        className="p-1 rounded border border-accent-500 text-accent-400 hover:bg-accent-500/10 flex items-center justify-center"
                                                    >
                                                        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M13 10V3L4 14h7v7l9-11h-7z"></path></svg>
                                                    </button>
                                                    <div className="hidden group-hover:block absolute bottom-full mb-1 right-0 bg-gray-900 text-white text-xs rounded px-2 py-0.5 whitespace-nowrap border border-gray-700 text-center">
                                                        Songs
                                                    </div>
                                                </div>
                                                <div className="relative group">
                                                    <button
                                                        onClick={() => handleSimilarArtists(song)}
                                                        className="p-1 rounded border border-blue-500 text-blue-400 hover:bg-blue-500/10 flex items-center justify-center"
                                                    >
                                                        <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
                                                            <path d="M9 6a3 3 0 11-6 0 3 3 0 016 0zM17 6a3 3 0 11-6 0 3 3 0 016 0zM12.93 11a6 6 0 00-11.86 0v2a1 1 0 001 1h9.86a1 1 0 001-1v-2z"></path>
                                                        </svg>
                                                    </button>
                                                    <div className="hidden group-hover:block absolute bottom-full mb-1 right-0 bg-gray-900 text-white text-xs rounded px-2 py-0.5 whitespace-nowrap border border-gray-700 text-center">
                                                        Artists
                                                    </div>
                                                </div>
                                                {isInQueue ? (
                                                    <div className="relative group">
                                                        <button onClick={() => onRemoveFromQueue(song.id)} className="p-1 rounded border border-red-500 text-red-400 hover:bg-red-500/10 flex items-center justify-center">
                                                            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"></path></svg>
                                                        </button>
                                                        <div className="hidden group-hover:block absolute bottom-full mb-1 right-0 bg-gray-900 text-white text-xs rounded px-2 py-0.5 whitespace-nowrap border border-gray-700 text-center">
                                                            Remove
                                                        </div>
                                                    </div>
                                                ) : (
                                                    <div className="relative group">
                                                        <button onClick={() => onAddToQueue(song)} className="p-1 rounded border border-green-500 text-green-400 hover:bg-green-500/10 flex items-center justify-center">
                                                            <svg className="w-4 h-4" fill="none" stroke="currentColor" strokeWidth="2"><path d="M4 6h16M4 10h16M4 14h4" /><path d="M16 12v8m-4-4h8" /></svg>
                                                        </button>
                                                        <div className="hidden group-hover:block absolute bottom-full mb-1 right-0 bg-gray-900 text-white text-xs rounded px-2 py-0.5 whitespace-nowrap border border-gray-700 text-center">
                                                            Queue
                                                        </div>
                                                    </div>
                                                )}
                                                <div className="relative group">
                                                    <button onClick={() => onAddToPlaylist(song)} className="p-1 rounded border border-purple-500 text-purple-400 hover:bg-purple-500/10 flex items-center justify-center">
                                                        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 6v6m0 0v6m0-6h6m-6 0H6"></path></svg>
                                                    </button>
                                                    <div className="hidden group-hover:block absolute bottom-full mb-1 right-0 bg-gray-900 text-white text-xs rounded px-2 py-0.5 whitespace-nowrap border border-gray-700 text-center">
                                                        Playlist
                                                    </div>
                                                </div>
                                            </div>
                                        </td>
                                    </tr>
                                );
                            })}
                        </tbody>
                    </table>
                    {isLoading && <p className="text-center text-gray-400 mt-4">Loading songs...</p>}
                    </div>
                </>
            )}
        </div>
    );
}

