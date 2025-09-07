// Suggested path: music-server-frontend/src/components/MusicViews.jsx
import React, { useState, useEffect, useCallback } from 'react';

const subsonicFetch = async (endpoint, creds, params = {}) => {
    const allParams = new URLSearchParams({
        u: creds.username, p: creds.password, v: '1.16.1', c: 'AudioMuse-AI', f: 'json', ...params
    });
    const response = await fetch(`/rest/${endpoint}?${allParams.toString()}`);
    if (!response.ok) {
        let errorData;
        try {
            errorData = await response.json();
            const subsonicResponse = errorData['subsonic-response'];
            throw new Error(subsonicResponse?.error?.message || `Server error: ${response.status}`);
        } catch (e) {
            // If parsing JSON fails, throw a generic error
            throw new Error(`Server error: ${response.status}`);
        }
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

    const handleAdd = async () => {
        if (!selectedPlaylist) {
            setError('Please select a playlist.');
            return;
        }
        setError('');
        setSuccess('');
        try {
            await subsonicFetch('updatePlaylist.view', credentials, {
                playlistId: selectedPlaylist,
                songIdToAdd: song.id
            });
            setSuccess(`Successfully added "${song.title}" to the playlist!`);
            setTimeout(() => {
                onAdded();
                onClose();
            }, 1500);
        } catch (err) {
            setError(err.message || 'Failed to add song.');
        }
    };

    return (
        <div className="fixed inset-0 bg-black bg-opacity-60 flex items-center justify-center z-50">
            <div className="bg-gray-800 p-6 rounded-lg shadow-xl w-full max-w-md relative">
                <button onClick={onClose} className="absolute top-2 right-2 text-gray-400 hover:text-white">
                    <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12"></path></svg>
                </button>
                <h3 className="text-xl font-bold mb-4 text-teal-400">Add to Playlist</h3>
                <p className="mb-4">Add <span className="font-semibold">{song.title}</span> to which playlist?</p>
                {error && <p className="text-red-500 mb-2">{error}</p>}
                {success && <p className="text-green-400 mb-2">{success}</p>}
                <select value={selectedPlaylist} onChange={e => setSelectedPlaylist(e.target.value)} className="w-full p-2 bg-gray-700 rounded mb-4">
                    {playlists.map(p => <option key={p.id} value={p.id}>{p.name}</option>)}
                </select>
                <div className="flex justify-end space-x-4">
                    <button onClick={onClose} className="bg-gray-600 hover:bg-gray-700 text-white font-bold py-2 px-4 rounded">Cancel</button>
                    <button onClick={handleAdd} disabled={success} className="bg-teal-500 hover:bg-teal-600 text-white font-bold py-2 px-4 rounded disabled:opacity-50">
                        {success ? 'Added!' : 'Add'}
                    </button>
                </div>
            </div>
        </div>
    );
};


export function Songs({ credentials, filter, onPlay, currentSong }) {
    const [songs, setSongs] = useState([]);
    const [songToAdd, setSongToAdd] = useState(null); // Track which song is being added to a playlist

    const fetchSongs = useCallback(async () => {
        if (!filter || (!filter.albumId && !filter.playlistId)) return;

        try {
            const endpoint = filter.albumId ? 'getAlbum.view' : 'getPlaylist.view';
            const idParam = filter.albumId ? filter.albumId : filter.playlistId;

            const data = await subsonicFetch(endpoint, credentials, { id: idParam });
            
            // The API returns songs differently for albums vs playlists
            const songListSource = data?.album || data?.directory;

            if (songListSource && songListSource.song) {
                const songList = Array.isArray(songListSource.song) ? songListSource.song : [songListSource.song];
                setSongs(songList);
            } else {
                setSongs([]);
            }
        } catch (e) {
            console.error("Failed to fetch songs:", e);
            setSongs([]);
        }
    }, [credentials, filter]);

    useEffect(() => {
        fetchSongs();
    }, [fetchSongs]);

    const handlePlayAlbum = () => {
        if (songs.length > 0) {
            onPlay(songs[0], songs);
        }
    };

    return (
        <div>
            {songToAdd && (
                <AddToPlaylistModal 
                    song={songToAdd} 
                    credentials={credentials} 
                    onClose={() => setSongToAdd(null)}
                    onAdded={fetchSongs} // Refetch songs to update play counts, etc.
                />
            )}
            {songs.length > 0 && (
                <button onClick={handlePlayAlbum} className="mb-4 bg-teal-500 hover:bg-teal-600 text-white font-bold py-2 px-4 rounded">Play All</button>
            )}
            <table className="min-w-full text-sm text-left text-gray-400">
                <thead className="text-xs text-gray-300 uppercase bg-gray-700">
                    <tr>
                        <th className="px-6 py-3 w-12"></th>
                        <th className="px-6 py-3">Title</th>
                        <th className="px-6 py-3">Artist</th>
                        <th className="px-6 py-3">Album</th>
                        <th className="px-6 py-3 text-center">Plays</th>
                        <th className="px-6 py-3">Last Played</th>
                        <th className="px-6 py-3 w-12"></th>
                    </tr>
                </thead>
                <tbody>
                    {songs.map(song => {
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
                                <td className="px-6 py-4 text-center">{song.playCount || 0}</td>
                                <td className="px-6 py-4">{song.lastPlayed ? new Date(song.lastPlayed).toLocaleString() : 'Never'}</td>
                                <td className="px-6 py-4">
                                    <button onClick={() => setSongToAdd(song)} title="Add to playlist">
                                        <svg className="w-6 h-6 text-gray-400 hover:text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 9v3m0 0v3m0-3h3m-3 0H9m12 0a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg>
                                    </button>
                                </td>
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
    useEffect(() => {
        const fetchAlbums = async () => {
            try {
                const data = await subsonicFetch('getAlbumList2.view', credentials, { type: 'alphabeticalByName', size: 500 });
                const albumList = data?.albumList2;
                if (albumList && albumList.album) {
                    const allAlbums = Array.isArray(albumList.album) ? albumList.album : [albumList.album];
                    const filteredAlbums = filter ? allAlbums.filter(a => a.artist === filter) : allAlbums;
                    setAlbums(filteredAlbums);
                } else {
                    setAlbums([]);
                }
            } catch (e) {
                console.error("Failed to fetch albums:", e);
                setAlbums([]);
            }
        };
        fetchAlbums();
    }, [credentials, filter]);

    return (
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-6">
            {albums.map((album) => (
                <button key={album.id} onClick={() => onNavigate({ page: 'songs', title: album.name, filter: { albumId: album.id } })} className="bg-gray-800 rounded-lg p-4 text-center hover:bg-gray-700 transition-colors">
                    <div className="w-full bg-gray-700 rounded aspect-square flex items-center justify-center mb-2 overflow-hidden">
                        {album.coverArt ? (
                            <img 
                                src={`/rest/getCoverArt.view?id=${album.id}&u=${credentials.username}&p=${credentials.password}&v=1.16.1&c=AudioMuse-AI`}
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
    );
}

export function Artists({ credentials, onNavigate }) {
    const [artists, setArtists] = useState([]);
     useEffect(() => {
        const fetchArtists = async () => {
            try {
                const data = await subsonicFetch('getArtists.view', credentials);
                const artistsContainer = data?.artists;
                if (artistsContainer && artistsContainer.artist) {
                    const artistList = Array.isArray(artistsContainer.artist) ? artistsContainer.artist : [artistsContainer.artist];
                    setArtists(artistList);
                } else {
                    setArtists([]);
                }
            } catch(e) {
                console.error("Failed to fetch artists:", e);
                setArtists([]);
            }
        };
        fetchArtists();
    }, [credentials]);

    return (
        <ul className="space-y-2">
            {artists.map((artist) => (
                <li key={artist.id}>
                    <button onClick={() => onNavigate({ page: 'albums', title: artist.name, filter: artist.name })} className="w-full text-left bg-gray-800 p-4 rounded-lg hover:bg-gray-700 transition-colors">
                        {artist.name}
                    </button>
                </li>
            ))}
        </ul>
    );
}

