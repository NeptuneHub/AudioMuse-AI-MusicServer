// Suggested path: music-server-frontend/src/components/MusicViews.jsx
import React, { useState, useEffect } from 'react';

const subsonicFetch = async (endpoint, creds, params = {}) => {
    const allParams = new URLSearchParams({
        u: creds.username, p: creds.password, v: '1.16.1', c: 'AudioMuse-AI', f: 'json', ...params
    });
    const response = await fetch(`/rest/${endpoint}?${allParams.toString()}`);
    if (!response.ok) {
        const data = await response.json();
        const subsonicResponse = data['subsonic-response'];
        throw new Error(subsonicResponse?.error?.message || `Server error: ${response.status}`);
    }
    const data = await response.json();
    return data['subsonic-response'];
};

const AddToPlaylistModal = ({ song, credentials, onClose, onAdded }) => {
    const [playlists, setPlaylists] = useState([]);
    const [selectedPlaylist, setSelectedPlaylist] = useState('');
    const [error, setError] = useState('');
    const [success, setSuccess] = useState('');

    useEffect(() => {
        const fetchPlaylists = async () => {
            try {
                const data = await subsonicFetch('getPlaylists.view', credentials);
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
            await subsonicFetch('updatePlaylist.view', credentials, {
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


export function Songs({ credentials, filter, onPlay, onAddToQueue, currentSong }) {
    const [songs, setSongs] = useState([]);
    const [searchTerm, setSearchTerm] = useState('');
    const [selectedSongForPlaylist, setSelectedSongForPlaylist] = useState(null);

    useEffect(() => {
        const fetchSongs = async () => {
            try {
                let songList = [];
                if (searchTerm.length >= 3) {
                    const data = await subsonicFetch('search2.view', credentials, { query: searchTerm, songCount: 100 });
                    songList = data.searchResult2?.song || [];
                } else if (filter && searchTerm.length === 0) {
                    const endpoint = filter.albumId ? 'getAlbum.view' : 'getPlaylist.view';
                    const idParam = filter.albumId || filter.playlistId;
                    if (!idParam) { setSongs([]); return; }

                    const data = await subsonicFetch(endpoint, credentials, { id: idParam });
                    const songContainer = data.album || data.directory;
                    if (songContainer && songContainer.song) {
                        songList = Array.isArray(songContainer.song) ? songContainer.song : [songContainer.song];
                    }
                } else {
                     setSongs([]);
                }
                setSongs(songList);
            } catch (e) {
                console.error("Failed to fetch songs:", e);
                setSongs([]);
            }
        };

        const debounceFetch = setTimeout(() => {
            fetchSongs();
        }, 300);

        return () => clearTimeout(debounceFetch);
    }, [credentials, filter, searchTerm]);


    const handlePlayAlbum = () => {
        if (songs.length > 0) {
            onPlay(songs[0], songs);
        }
    };
    
    const formatDateTime = (isoString) => {
        if (!isoString) return 'N/A';
        try {
            const date = new Date(isoString);
            return date.toLocaleString();
        } catch (e) {
            return 'Invalid Date';
        }
    };


    return (
        <div>
            <div className="mb-4">
                <input
                    type="text"
                    placeholder="Search for a song or artist..."
                    value={searchTerm}
                    onChange={(e) => setSearchTerm(e.target.value)}
                    className="w-full p-2 bg-gray-700 rounded border border-gray-600 focus:outline-none focus:border-teal-500"
                />
            </div>

            {songs.length > 0 && !searchTerm &&(
                <button onClick={handlePlayAlbum} className="mb-4 bg-teal-500 hover:bg-teal-600 text-white font-bold py-2 px-4 rounded">Play All</button>
            )}
            <div className="overflow-x-auto">
                <table className="min-w-full text-sm text-left text-gray-400">
                    <thead className="text-xs text-gray-300 uppercase bg-gray-700">
                        <tr>
                            <th className="px-4 py-3 w-12"></th>
                            <th className="px-4 py-3">Title</th>
                            <th className="px-4 py-3 hidden sm:table-cell">Artist</th>
                            <th className="px-4 py-3 hidden md:table-cell">Album</th>
                            <th className="px-4 py-3 w-24">Actions</th>
                        </tr>
                    </thead>
                    <tbody>
                        {songs.map(song => {
                            const isPlaying = currentSong && currentSong.id === song.id;
                            return (
                                <tr key={song.id} className={`border-b border-gray-700 transition-colors ${isPlaying ? 'bg-teal-900/50' : 'bg-gray-800 hover:bg-gray-600'}`}>
                                    <td className="px-4 py-4">
                                        <button onClick={() => onPlay(song, songs)} title="Play song">
                                            {isPlaying ? (
                                                <svg className="w-6 h-6 text-green-400" fill="currentColor" viewBox="0 0 20 20"><path fillRule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zM7 8a1 1 0 012 0v4a1 1 0 11-2 0V8zm5-1a1 1 0 00-1 1v4a1 1 0 102 0V8a1 1 0 00-1-1z" clipRule="evenodd"></path></svg>
                                            ) : (
                                                <svg className="w-6 h-6 text-teal-400 hover:text-teal-200" fill="currentColor" viewBox="0 0 20 20"><path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM9.555 7.168A1 1 0 008 8v4a1 1 0 001.555.832l3-2a1 1 0 000-1.664l-3-2z" clipRule="evenodd"></path></svg>
                                            )}
                                        </button>
                                    </td>
                                    <td className={`px-4 py-4 font-medium ${isPlaying ? 'text-green-400' : 'text-white'}`}>
                                        <div>{song.title}</div>
                                        <div className="sm:hidden text-xs text-gray-400">{song.artist}</div>
                                    </td>
                                    <td className="px-4 py-4 hidden sm:table-cell">{song.artist}</td>
                                    <td className="px-4 py-4 hidden md:table-cell">{song.album}</td>
                                    <td className="px-4 py-4">
                                        <div className="flex items-center space-x-2">
                                            <button onClick={() => onAddToQueue(song)} title="Add to queue" className="text-gray-400 hover:text-white">
                                                 <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 9v3m0 0v3m0-3h3m-3 0H9m12 0a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg>
                                            </button>
                                            <button onClick={() => setSelectedSongForPlaylist(song)} title="Add to playlist" className="text-gray-400 hover:text-white">
                                                <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 6v6m0 0v6m0-6h6m-6 0H6"></path></svg>
                                            </button>
                                        </div>
                                    </td>
                                </tr>
                            );
                        })}
                    </tbody>
                </table>
            </div>
             {selectedSongForPlaylist && (
                <AddToPlaylistModal
                    song={selectedSongForPlaylist}
                    credentials={credentials}
                    onClose={() => setSelectedSongForPlaylist(null)}
                    onAdded={() => {}}
                />
            )}
        </div>
    );
}

export function Albums({ credentials, filter, onNavigate }) {
    const [albums, setAlbums] = useState([]);
    const [searchTerm, setSearchTerm] = useState('');

    useEffect(() => {
        if (filter) {
            setSearchTerm(filter);
        }
    }, [filter]);

    useEffect(() => {
        const fetchAlbums = async () => {
            try {
                let albumList = [];
                const query = searchTerm || filter;

                if (query) {
                    const data = await subsonicFetch('search2.view', credentials, { query: query, albumCount: 100, artistCount: 0, songCount: 0 });
                    albumList = data.searchResult2?.album || [];
                } else {
                    const data = await subsonicFetch('getAlbumList2.view', credentials, { type: 'alphabeticalByName' });
                     albumList = data.albumList2?.album || [];
                }
                setAlbums(Array.isArray(albumList) ? albumList : [albumList].filter(Boolean));
            } catch (e) {
                console.error("Failed to fetch albums:", e);
                setAlbums([]);
            }
        };

        const debounceFetch = setTimeout(() => {
            fetchAlbums();
        }, 300);

        return () => clearTimeout(debounceFetch);
    }, [credentials, filter, searchTerm]);

    return (
        <div>
            <div className="mb-4">
                <input
                    type="text"
                    placeholder="Search for an album or artist..."
                    value={searchTerm}
                    onChange={(e) => setSearchTerm(e.target.value)}
                    className="w-full p-2 bg-gray-700 rounded border border-gray-600 focus:outline-none focus:border-teal-500"
                />
            </div>
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 xl:grid-cols-8 gap-4">
                {albums.map((album) => (
                    <button key={album.id} onClick={() => onNavigate({ page: 'songs', title: album.name, filter: { albumId: album.id } })} className="bg-gray-800 rounded-lg p-4 text-center hover:bg-gray-700 transition-colors">
                        <div className="w-full bg-gray-700 rounded aspect-square flex items-center justify-center mb-2 overflow-hidden">
                            {album.coverArt ? (
                                <img
                                    src={`/rest/getCoverArt.view?id=${album.coverArt}&u=${credentials.username}&p=${credentials.password}&v=1.16.1&c=AudioMuse-AI`}
                                    alt={album.name}
                                    className="w-full h-full object-cover"
                                />
                            ) : (
                                <svg className="w-12 h-12 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="1" d="M9 19V6l12-3v13M9 19c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zm12-3c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zM9 6l12-3"></path></svg>
                            )}
                        </div>
                        <p className="font-bold text-white truncate">{album.name}</p>
                        <p className="text-sm text-gray-400 truncate">{album.artist}</p>
                    </button>
                ))}
            </div>
        </div>
    );
}

export function Artists({ credentials, onNavigate }) {
    const [artists, setArtists] = useState([]);
    const [searchTerm, setSearchTerm] = useState('');

    useEffect(() => {
        const fetchArtists = async () => {
            try {
                let artistList = [];
                if (searchTerm.length >= 3) {
                    const data = await subsonicFetch('search2.view', credentials, { query: searchTerm, artistCount: 50 });
                    artistList = data.searchResult2?.artist || [];
                } else if (searchTerm.length === 0) {
                    const data = await subsonicFetch('getArtists.view', credentials);
                    const artistsContainer = data?.artists;
                    if (artistsContainer && artistsContainer.artist) {
                        artistList = Array.isArray(artistsContainer.artist) ? artistsContainer.artist : [artistsContainer.artist];
                    }
                } else {
                    setArtists([]);
                }
                setArtists(artistList);
            } catch (e) {
                console.error("Failed to fetch artists:", e);
                setArtists([]);
            }
        };

        const debounceFetch = setTimeout(() => {
            fetchArtists();
        }, 300);

        return () => clearTimeout(debounceFetch);
    }, [credentials, searchTerm]);

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
            <ul className="space-y-2">
                {artists.map((artist) => (
                    <li key={artist.id}>
                        <button onClick={() => onNavigate({ page: 'albums', title: artist.name, filter: artist.name })} className="w-full text-left bg-gray-800 p-4 rounded-lg hover:bg-gray-700 transition-colors">
                            {artist.name}
                        </button>
                    </li>
                ))}
            </ul>
        </div>
    );
}

