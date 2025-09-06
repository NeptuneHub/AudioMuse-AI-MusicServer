// Suggested path: music-server-frontend/src/App.jsx
import React, { useState, useEffect, useCallback, useMemo } from 'react';
import AdminPanel from './components/AdminPanel';

// --- Main App Component ---
function App() {
	// Subsonic auth is session-based; we store credentials for the session.
	const [credentials, setCredentials] = useState(null);
	const [isAdmin, setIsAdmin] = useState(false);

	const handleLogin = (creds) => {
		setCredentials(creds);
		setIsAdmin(false); // Default to false until we verify
		localStorage.removeItem('token'); // Clear any old token

		// After successful Subsonic auth, get a JWT for the admin panel
		// and the user's actual admin status.
		fetch('/api/v1/user/login', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(creds)
		}).then(res => {
			if (res.ok) return res.json();
			return Promise.reject('Failed to get JWT for admin panel');
		}).then(data => {
			if (data.token) {
				localStorage.setItem('token', data.token);
				setIsAdmin(data.is_admin);
			}
		}).catch(err => {
			console.error("Could not retrieve JWT for admin panel:", err);
		});
	};

	const handleLogout = () => {
		localStorage.removeItem('token'); // For admin panel
		setCredentials(null);
		setIsAdmin(false);
	};


	return (
		<div className="bg-gray-900 text-white min-h-screen font-sans">
			{credentials ? (
				<Dashboard onLogout={handleLogout} isAdmin={isAdmin} credentials={credentials} />
			) : (
				<Login onLogin={handleLogin} />
			)}
		</div>
	);
}


// --- Subsonic API Helper ---
const subsonicFetch = async (endpoint, creds, params = {}) => {
    const allParams = new URLSearchParams({
        u: creds.username, p: creds.password, v: '1.16.1', c: 'AudioMuse-AI', f: 'json', ...params
    });
    return fetch(`/rest/${endpoint}?${allParams.toString()}`);
};
// --- Login Component ---
function Login({ onLogin }) {
	const [username, setUsername] = useState('');
	const [password, setPassword] = useState('');
	const [error, setError] = useState('');

	const handleSubmit = async (e) => {
		e.preventDefault();
		setError('');
		try {
			// Use getLicense.view for login validation, as it's lightweight and requires auth.
			// ping.view is now unauthenticated and cannot be used to verify credentials.
			const response = await subsonicFetch('getLicense.view', { username, password });

			if (!response.ok) { // Check HTTP status first
				// The server now sends 401 for bad creds
				const data = await response.json();
				const subsonicResponse = data['subsonic-response'];
				setError(subsonicResponse?.error?.message || 'Login failed: Invalid credentials');
				return;
			}

			const data = await response.json();
			const subsonicResponse = data['subsonic-response'];

			if (subsonicResponse.status === 'ok') {
				// The Subsonic login is successful. Now call the parent handler.
				onLogin({ username, password });
			} else {
				// This case might happen if server sends 200 OK but status: "failed"
				setError(subsonicResponse?.error?.message || 'Login failed');
			}
		} catch (err) {
			setError('Network error. Could not connect to server.');
		}
	};

	return (
		<div className="flex items-center justify-center h-screen">
			<div className="bg-gray-800 p-8 rounded-lg shadow-xl w-96">
				<h2 className="text-2xl font-bold mb-6 text-center text-teal-400">AudioMuse-AI</h2>
				<form onSubmit={handleSubmit}>
					<div className="mb-4">
						<label className="block text-gray-400 mb-2" htmlFor="username">Username</label>
						<input
							id="username"
							type="text"
							value={username}
							onChange={(e) => setUsername(e.target.value)}
							className="w-full p-2 bg-gray-700 rounded border border-gray-600 focus:outline-none focus:border-teal-500"
						/>
					</div>
					<div className="mb-6">
						<label className="block text-gray-400 mb-2" htmlFor="password">Password</label>
						<input
							id="password"
							type="password"
							value={password}
							onChange={(e) => setPassword(e.target.value)}
							className="w-full p-2 bg-gray-700 rounded border border-gray-600 focus:outline-none focus:border-teal-500"
						/>
					</div>
					{error && <p className="text-red-500 text-center mb-4">{error}</p>}
					<button type="submit" className="w-full bg-teal-500 hover:bg-teal-600 text-white font-bold py-2 px-4 rounded transition duration-300">
						Login
					</button>
				</form>
			</div>
		</div>
	);
}

// --- Dashboard Component ---
function Dashboard({ onLogout, isAdmin, credentials }) {
    const [navigation, setNavigation] = useState([{ page: 'artists', title: 'Artists' }]);
    const [playQueue, setPlayQueue] = useState([]);
    const [currentTrackIndex, setCurrentTrackIndex] = useState(0);
    
    const currentView = useMemo(() => navigation[navigation.length - 1], [navigation]);
    const currentSong = useMemo(() => playQueue[currentTrackIndex], [playQueue, currentTrackIndex]);

    const handleNavigate = (newView) => setNavigation(prev => [...prev, newView]);
    const handleBack = () => navigation.length > 1 && setNavigation(prev => prev.slice(0, -1));
    const handleResetNavigation = (page, title) => setNavigation([{ page, title }]);

    const handlePlaySong = useCallback((song, songList) => {
        const queue = songList || [song];
        const songIndex = queue.findIndex(s => s.id === song.id);
        setPlayQueue(queue);
        setCurrentTrackIndex(songIndex >= 0 ? songIndex : 0);
    }, []);

    const handlePlayNext = useCallback(() => {
        if (playQueue.length > 0) {
            setCurrentTrackIndex(prev => (prev + 1) % playQueue.length);
        }
    }, [playQueue.length]);

	return (
		<div className="flex flex-col h-screen">
			<nav className="bg-gray-800 shadow-md">
                <div className="container mx-auto px-6 py-3 flex justify-between items-center">
                    <h1 className="text-xl font-bold text-teal-400">AudioMuse-AI</h1>
                    <div className="flex items-center space-x-4">
                        <button onClick={() => handleResetNavigation('songs', 'All Songs')} className={`px-3 py-2 rounded font-semibold transition duration-300 ${currentView.page === 'songs' && navigation.length === 1 ? 'bg-teal-500 text-white' : 'text-gray-300 hover:bg-gray-700'}`}>Songs</button>
                        <button onClick={() => handleResetNavigation('albums', 'All Albums')} className={`px-3 py-2 rounded font-semibold transition duration-300 ${currentView.page === 'albums' && navigation.length === 1 ? 'bg-teal-500 text-white' : 'text-gray-300 hover:bg-gray-700'}`}>Albums</button>
                        <button onClick={() => handleResetNavigation('artists', 'Artists')} className={`px-3 py-2 rounded font-semibold transition duration-300 ${currentView.page === 'artists' && navigation.length === 1 ? 'bg-teal-500 text-white' : 'text-gray-300 hover:bg-gray-700'}`}>Artists</button>
                        <button onClick={() => handleResetNavigation('playlists', 'Playlists')} className={`px-3 py-2 rounded font-semibold transition duration-300 ${currentView.page === 'playlists' ? 'bg-teal-500 text-white' : 'text-gray-300 hover:bg-gray-700'}`}>Playlists</button>
                        {isAdmin && <button onClick={() => handleResetNavigation('admin', 'Admin Panel')} className={`px-3 py-2 rounded font-semibold transition duration-300 ${currentView.page === 'admin' ? 'bg-teal-500 text-white' : 'text-gray-300 hover:bg-gray-700'}`}>Admin</button>}
                        <button onClick={onLogout} className="px-3 py-2 rounded bg-red-600 hover:bg-red-700 text-white font-semibold transition duration-300">Logout</button>
                    </div>
                </div>
			</nav>
			<main className="flex-1 p-8 bg-gray-900 overflow-y-auto pb-24">
                <div className="mb-4">
                    {navigation.length > 1 && (
                        <button onClick={handleBack} className="text-teal-400 hover:text-teal-200 font-semibold mb-4">&larr; Back</button>
                    )}
                    <h2 className="text-3xl font-bold">{currentView.title}</h2>
                </div>
				{currentView.page === 'songs' && <Songs credentials={credentials} filter={currentView.filter} onPlay={handlePlaySong} currentSong={currentSong} />}
				{currentView.page === 'albums' && <Albums credentials={credentials} filter={currentView.filter} onNavigate={handleNavigate} />}
				{currentView.page === 'artists' && <Artists credentials={credentials} onNavigate={handleNavigate} />}
				{currentView.page === 'playlists' && <Playlists credentials={credentials} />}
                {currentView.page === 'admin' && isAdmin && <AdminPanel />}
			</main>
            <AudioPlayer song={currentSong} onEnded={handlePlayNext} credentials={credentials} />
		</div>
	);
}

// --- Content Components ---
function Songs({ credentials, filter, onPlay, currentSong }) {
    const [songs, setSongs] = useState([]);

    useEffect(() => {
        const fetchSongs = async () => {
            // In Subsonic, you get songs by getting an album's content.
            if (!filter || !filter.album) return;

            try {
                const response = await subsonicFetch('getAlbum.view', credentials, { id: filter.album });
                const data = await response.json();
                const directory = data['subsonic-response']?.directory;
                if (directory && directory.song) {
                    // Subsonic can return a single object or an array.
                    const songList = Array.isArray(directory.song) ? directory.song : [directory.song];
                    setSongs(songList);
                } else {
                    setSongs([]);
                }
            } catch (e) {
                console.error("Failed to fetch songs:", e);
                setSongs([]);
            }
        };
        fetchSongs();
    }, [credentials, filter]);

    const handlePlayAlbum = () => {
        if (songs.length > 0) {
            onPlay(songs[0], songs);
        }
    };

    return (
        <div>
            {filter?.album && songs.length > 0 && (
                <button onClick={handlePlayAlbum} className="mb-4 bg-teal-500 hover:bg-teal-600 text-white font-bold py-2 px-4 rounded">Play Album</button>
            )}
            <table className="min-w-full text-sm text-left text-gray-400">
                <thead className="text-xs text-gray-300 uppercase bg-gray-700">
                    <tr>
                        <th className="px-6 py-3 w-12"></th>
                        <th className="px-6 py-3">Title</th>
                        <th className="px-6 py-3">Artist</th>
                        <th className="px-6 py-3">Album</th>
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
                                            <svg className="w-6 h-6 text-green-400" fill="currentColor" viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg"><path fillRule="evenodd" d="M9.383 3.076A1 1 0 0110 4v12a1 1 0 01-1.707.707L4.586 13H2a1 1 0 01-1-1V8a1 1 0 011-1h2.586l3.707-3.707a1 1 0 011.09-.217zM14.657 2.929a1 1 0 011.414 0A9.972 9.972 0 0119 10a9.972 9.972 0 01-2.929 7.071 1 1 0 01-1.414-1.414A7.971 7.971 0 0017 10c0-2.21-.894-4.208-2.343-5.657a1 1 0 010-1.414zm-2.829 2.828a1 1 0 011.415 0A5.983 5.983 0 0115 10a5.984 5.984 0 01-1.757 4.243 1 1 0 01-1.415-1.415A3.984 3.984 0 0013 10a3.983 3.983 0 00-1.172-2.828 1 1 0 010-1.415z" clipRule="evenodd"></path></svg>
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
                            </tr>
                        );
                    })}
                </tbody>
            </table>
        </div>
    );
}

function Albums({ credentials, filter, onNavigate }) {
    const [albums, setAlbums] = useState([]);
    useEffect(() => {
        const fetchAlbums = async () => {
            try {
                // Subsonic uses getAlbumList2 for this. The 'type' param tells it how to sort.
                // We can filter by artist on the frontend if needed, as getAlbumList2 returns all albums.
                const response = await subsonicFetch('getAlbumList2.view', credentials, { type: 'alphabeticalByName' });
                const data = await response.json();
                const albumList = data['subsonic-response']?.albumList2;
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
                <button key={album.id} onClick={() => onNavigate({ page: 'songs', title: album.name, filter: { album: album.name, artist: album.artist } })} className="bg-gray-800 rounded-lg p-4 text-center hover:bg-gray-700 transition-colors">
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
    );
}

function Artists({ credentials, onNavigate }) {
    const [artists, setArtists] = useState([]);
     useEffect(() => {
        const fetchArtists = async () => {
            try {
                const response = await subsonicFetch('getArtists.view', credentials);
                const data = await response.json();
                const artistsContainer = data['subsonic-response']?.artists;
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

function Playlists() {
	return (<p>Playlist functionality is not yet implemented.</p>);
}

function AudioPlayer({ song, onEnded, credentials }) {
    const [audioSrc, setAudioSrc] = useState(null);

    useEffect(() => {
        if (!song) {
            setAudioSrc(null);
            return;
        }

        let objectUrl;
        const fetchAndSetAudio = async () => {
            try {
                const response = await subsonicFetch('stream.view', credentials, { id: song.id });
                if (!response.ok) throw new Error('Failed to fetch song');
                const blob = await response.blob();
                objectUrl = URL.createObjectURL(blob);
                setAudioSrc(objectUrl);
            } catch (error) {
                console.error("Error streaming song:", error);
                setAudioSrc(null);
            }
        };

        fetchAndSetAudio();
        return () => { // Cleanup function
            if (objectUrl) URL.revokeObjectURL(objectUrl);
        };
    }, [song, credentials]);

    if (!song) {
        return (
            <div className="fixed bottom-0 left-0 right-0 bg-gray-800 p-4 text-center text-gray-500">
                No song selected.
            </div>
        );
    }

    return (
        <div className="fixed bottom-0 left-0 right-0 bg-gray-800 p-4 shadow-lg border-t border-gray-700 flex items-center space-x-4">
            <div className="flex-shrink-0 w-64">
                <p className="font-bold text-white truncate">{song.title}</p>
                <p className="text-sm text-gray-400 truncate">{song.artist}</p>
            </div>
            <audio key={song.id} controls autoPlay src={audioSrc || ''} onEnded={onEnded} className="w-full">
                Your browser does not support the audio element.
            </audio>
        </div>
    );
}


export default App;
