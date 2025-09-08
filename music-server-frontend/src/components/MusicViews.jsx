// Suggested path: music-server-frontend/src/components/MusicViews.jsx
import React, { useState, useEffect, useCallback, useMemo } from 'react';

const subsonicFetch = async (endpoint, creds, params = {}) => {
    const allParams = new URLSearchParams({
        u: creds.username, p: creds.password, v: '1.16.1', c: 'AudioMuse-AI', f: 'json', ...params
    });
    // For POST requests, we'll handle params differently, but for now, GET is fine.
    const response = await fetch(`/rest/${endpoint}?${allParams.toString()}`);
    if (!response.ok) {
        const data = await response.json();
        const subsonicResponse = data['subsonic-response'];
        throw new Error(subsonicResponse?.error?.message || `Server error: ${response.status}`);
    }
    const data = await response.json();
    return data['subsonic-response'];
};


// A debounced function to delay API calls
const debounce = (func, delay) => {
    let timeout;
    return (...args) => {
        clearTimeout(timeout);
        timeout = setTimeout(() => func(...args), delay);
    };
};


export function Songs({ credentials, filter, onPlay, currentSong }) {
    const [songs, setSongs] = useState([]);
    const [searchTerm, setSearchTerm] = useState('');
    const [isLoading, setIsLoading] = useState(false);


    const searchSongs = useCallback(debounce(async (query) => {
        if (query.length < 3) {
            setSongs([]);
            return;
        }
        setIsLoading(true);
        try {
            const data = await subsonicFetch('search2.view', credentials, { query, songCount: 100 });
            const songResults = data?.searchResult2?.song || [];
            setSongs(Array.isArray(songResults) ? songResults : [songResults]);
        } catch (e) {
            console.error("Failed to search songs:", e);
            setSongs([]);
        }
        setIsLoading(false);
    }, 500), [credentials]);


    useEffect(() => {
        const fetchSongs = async () => {
            // This part handles loading songs for a specific album or playlist
            if (!filter) return;
            const endpoint = filter.albumId ? 'getAlbum.view' : 'getPlaylist.view';
            const idParam = filter.albumId || filter.playlistId;
            
            setIsLoading(true);
            try {
                const data = await subsonicFetch(endpoint, credentials, { id: idParam });
                const songList = data?.album?.song || data?.directory?.song || [];
                setSongs(Array.isArray(songList) ? songList : [songList]);
            } catch (e) {
                console.error("Failed to fetch songs:", e);
                setSongs([]);
            }
            setIsLoading(false);
        };

        if (filter) {
            fetchSongs();
        } else {
            // This part handles the global song search
            searchSongs(searchTerm);
        }
    }, [credentials, filter, searchTerm, searchSongs]);


    const handlePlayAlbum = () => {
        if (songs.length > 0) {
            onPlay(songs[0], songs);
        }
    };
    
    // Client-side filtering for when viewing a specific album/playlist
    const filteredSongs = useMemo(() => {
        if (!filter || searchTerm.length < 1) {
            return songs;
        }
        return songs.filter(song =>
            song.title.toLowerCase().includes(searchTerm.toLowerCase()) ||
            song.artist.toLowerCase().includes(searchTerm.toLowerCase())
        );
    }, [songs, searchTerm, filter]);

    return (
        <div>
            <div className="mb-4 flex justify-between items-center">
                 <input
                    type="text"
                    placeholder="Search for a song..."
                    value={searchTerm}
                    onChange={(e) => setSearchTerm(e.target.value)}
                    className="w-1/3 p-2 bg-gray-700 rounded border border-gray-600 focus:outline-none focus:border-teal-500"
                />
                {songs.length > 0 && (
                    <button onClick={handlePlayAlbum} className="bg-teal-500 hover:bg-teal-600 text-white font-bold py-2 px-4 rounded">Play All</button>
                )}
            </div>
            {isLoading && <p>Loading...</p>}
            <table className="min-w-full text-sm text-left text-gray-400">
                <thead className="text-xs text-gray-300 uppercase bg-gray-700">
                    <tr>
                        <th className="px-6 py-3 w-12"></th>
                        <th className="px-6 py-3">Title</th>
                        <th className="px-6 py-3">Artist</th>
                        <th className="px-6 py-3">Album</th>
                        <th className="px-6 py-3 text-right">Plays</th>
                        <th className="px-6 py-3">Last Played</th>
                    </tr>
                </thead>
                <tbody>
                    {filteredSongs.map(song => {
                        const isPlaying = currentSong && currentSong.id === song.id;
                        return (
                            <tr key={song.id} className={`border-b border-gray-700 transition-colors ${isPlaying ? 'bg-teal-900/50' : 'bg-gray-800 hover:bg-gray-600'}`}>
                                <td className="px-6 py-4">
                                     {isPlaying ? (
                                        <span title="Currently playing">
                                            <svg className="w-6 h-6 text-green-400" fill="currentColor" viewBox="0 0 20 20"><path fillRule="evenodd" d="M9.383 3.076A1 1 0 0110 4v12a1 1 0 01-1.707.707L4.586 13H2a1 1 0 01-1-1V8a1 1 0 011-1h2.586l3.707-3.707a1 1 0 011.09-.217zM14.657 2.929a1 1 0 011.414 0A9.972 9.972 0 0119 10a9.972 9.972 0 01-2.929 7.071 1 1 0 01-1.414-1.414A7.971 7.971 0 0017 10c0-2.21-.894-4.208-2.343-5.657a1 1 0 010-1.414zm-2.829 2.828a1 1 0 011.415 0A5.983 5.983 0 0115 10a5.984 5.984 0 01-1.757 4.243 1 1 0 01-1.415-1.415A3.984 3.984 0 0013 10a3.983 3.983 0 00-1.172-2.828 1 1 0 010-1.415z" clipRule="evenodd"></path></svg>
                                        </span>
                                    ) : (
                                        <button onClick={() => onPlay(song, songs)} title="Play song">
                                            <svg className="w-6 h-6 text-teal-400 hover:text-teal-200" fill="currentColor" viewBox="0 0 20 20"><path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM9.555 7.168A1 1 0 008 8v4a1 1 0 001.555.832l3-2a1 1 0 000-1.664l-3-2z" clipRule="evenodd"></path></svg>
                                        </button>
                                    )}
                                </td>
                                <td className={`px-6 py-4 font-medium ${isPlaying ? 'text-green-400' : 'text-white'}`}>{song.title}</td>
                                <td className="px-6 py-4">{song.artist}</td>
                                <td className="px-6 py-4">{song.album}</td>
                                <td className="px-6 py-4 text-right">{song.playCount || 0}</td>
                                <td className="px-6 py-4">{song.lastPlayed ? new Date(song.lastPlayed).toLocaleString() : 'Never'}</td>
                            </tr>
                        );
                    })}
                </tbody>
            </table>
        </div>
    );
}

export function Albums({ credentials, filter, onNavigate }) {
    const [albums, setAlbums] = useState([]);
    const [searchTerm, setSearchTerm] = useState('');
    const [isLoading, setIsLoading] = useState(false);

    const fetchAlbums = useCallback(async () => {
        setIsLoading(true);
        try {
            const data = await subsonicFetch('getAlbumList2.view', credentials, { type: 'alphabeticalByName', size: 500, id: filter });
            const albumList = data?.albumList2?.album || [];
            setAlbums(Array.isArray(albumList) ? albumList : [albumList]);
        } catch (e) {
            console.error("Failed to fetch albums:", e);
            setAlbums([]);
        }
        setIsLoading(false);
    }, [credentials, filter]);

    const searchAlbums = useCallback(debounce(async (query) => {
        if (query.length < 3) {
            fetchAlbums(); // Re-fetch all if search is cleared
            return;
        }
        setIsLoading(true);
        try {
            const data = await subsonicFetch('search2.view', credentials, { query, albumCount: 100 });
            const albumResults = data?.searchResult2?.album || [];
            setAlbums(Array.isArray(albumResults) ? albumResults : [albumResults]);
        } catch (e) {
            console.error("Failed to search albums:", e);
        }
        setIsLoading(false);
    }, 500), [credentials, fetchAlbums]);

    useEffect(() => {
        if (searchTerm.length >= 3) {
            searchAlbums(searchTerm);
        } else {
            fetchAlbums();
        }
    }, [credentials, searchTerm, searchAlbums, fetchAlbums]);

    return (
        <div>
            <div className="mb-6">
                <input
                    type="text"
                    placeholder="Search for an album..."
                    value={searchTerm}
                    onChange={(e) => setSearchTerm(e.target.value)}
                    className="w-1/3 p-2 bg-gray-700 rounded border border-gray-600 focus:outline-none focus:border-teal-500"
                />
            </div>
            {isLoading && <p>Loading...</p>}
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-6">
                {albums.map((album) => (
                    <button key={album.id} onClick={() => onNavigate({ page: 'songs', title: album.name, filter: { albumId: album.id, artist: album.artist } })} className="bg-gray-800 rounded-lg p-4 text-center hover:bg-gray-700 transition-colors">
                        <div className="w-full bg-gray-700 rounded aspect-square flex items-center justify-center mb-2 overflow-hidden">
                            {album.coverArt ? (
                                <img
                                    src={`/rest/getCoverArt.view?id=${album.coverArt}&u=${credentials.username}&p=${credentials.password}&v=1.16.1&c=AudioMuse-AI`}
                                    alt={album.name}
                                    className="w-full h-full object-cover"
                                />
                            ) : (
                                <svg className="w-1/2 h-1/2 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="1" d="M9 19V6l12-3v13M9 19c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zm12-3c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zM9 6l12-3"></path></svg>
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
    const [isLoading, setIsLoading] = useState(false);

    const fetchArtists = useCallback(async () => {
        setIsLoading(true);
        try {
            const data = await subsonicFetch('getArtists.view', credentials);
            const artistsContainer = data?.artists;
            const artistList = artistsContainer?.artist || [];
            setArtists(Array.isArray(artistList) ? artistList : [artistList]);
        } catch (e) {
            console.error("Failed to fetch artists:", e);
            setArtists([]);
        }
        setIsLoading(false);
    }, [credentials]);

    const searchArtists = useCallback(debounce(async (query) => {
        if (query.length < 3) {
            fetchArtists(); // Re-fetch all if search is cleared
            return;
        }
        setIsLoading(true);
        try {
            const data = await subsonicFetch('search2.view', credentials, { query, artistCount: 100 });
            const artistResults = data?.searchResult2?.artist || [];
            setArtists(Array.isArray(artistResults) ? artistResults : [artistResults]);
        } catch (e) {
            console.error("Failed to search artists:", e);
        }
        setIsLoading(false);
    }, 500), [credentials, fetchArtists]);


    useEffect(() => {
        if (searchTerm.length >= 3) {
            searchArtists(searchTerm);
        } else {
            fetchArtists();
        }
    }, [searchTerm, fetchArtists, searchArtists]);

    return (
        <div>
            <div className="mb-6">
                <input
                    type="text"
                    placeholder="Search for an artist..."
                    value={searchTerm}
                    onChange={(e) => setSearchTerm(e.target.value)}
                    className="w-1/3 p-2 bg-gray-700 rounded border border-gray-600 focus:outline-none focus:border-teal-500"
                />
            </div>
            {isLoading && <p>Loading...</p>}
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

