// Suggested path: music-server-frontend/src/App.jsx
import React, { useState, useEffect, useCallback, useMemo } from 'react';

// --- Helper Components ---
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


// --- Main App Component ---
function App() {
	const [token, setToken] = useState(localStorage.getItem('token'));
	const [isAdmin, setIsAdmin] = useState(localStorage.getItem('isAdmin') === 'true');

	const handleLogin = (newToken, adminStatus) => {
		localStorage.setItem('token', newToken);
		localStorage.setItem('isAdmin', adminStatus);
		setToken(newToken);
		setIsAdmin(adminStatus);
	};

	const handleLogout = () => {
		localStorage.removeItem('token');
		localStorage.removeItem('isAdmin');
		setToken(null);
		setIsAdmin(false);
	};

	// Check for token on initial load
	useEffect(() => {
		const storedToken = localStorage.getItem('token');
		const storedIsAdmin = localStorage.getItem('isAdmin') === 'true';
		if (storedToken) {
			setToken(storedToken);
			setIsAdmin(storedIsAdmin);
		}
	}, []);


	return (
		<div className="bg-gray-900 text-white min-h-screen font-sans">
			{token ? (
				<Dashboard onLogout={handleLogout} isAdmin={isAdmin} />
			) : (
				<Login onLogin={handleLogin} />
			)}
		</div>
	);
}


// --- Login Component ---
function Login({ onLogin }) {
	const [username, setUsername] = useState('');
	const [password, setPassword] = useState('');
	const [error, setError] = useState('');

	const handleSubmit = async (e) => {
		e.preventDefault();
		setError('');
		try {
			const response = await fetch('/api/v1/user/login', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ username, password })
			});
			const data = await response.json();
			if (response.ok) {
				onLogin(data.token, data.is_admin);
			} else {
				setError(data.error || 'Login failed');
			}
		} catch (err) {
			setError('Network error');
		}
	};

	return (
		<div className="flex items-center justify-center h-screen">
			<div className="bg-gray-800 p-8 rounded-lg shadow-xl w-96">
				<h2 className="text-2xl font-bold mb-6 text-center text-teal-400">AudioMuse Server</h2>
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
function Dashboard({ onLogout, isAdmin }) {
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
                    <h1 className="text-xl font-bold text-teal-400">AudioMuse</h1>
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
				{currentView.page === 'songs' && <Songs filter={currentView.filter} onPlay={handlePlaySong} />}
				{currentView.page === 'albums' && <Albums filter={currentView.filter} onNavigate={handleNavigate} />}
				{currentView.page === 'artists' && <Artists onNavigate={handleNavigate} />}
				{currentView.page === 'playlists' && <Playlists />}
                {currentView.page === 'admin' && isAdmin && <AdminPanel />}
			</main>
            <AudioPlayer song={currentSong} onEnded={handlePlayNext} />
		</div>
	);
}

// --- Content Components ---
function Songs({ filter, onPlay }) {
    const [songs, setSongs] = useState([]);

    useEffect(() => {
        const fetchSongs = async () => {
            const token = localStorage.getItem('token');
            let url = '/api/v1/music/songs';
            if (filter) {
                const params = new URLSearchParams(filter);
                url += `?${params.toString()}`;
            }
            const response = await fetch(url, { headers: { 'Authorization': `Bearer ${token}` } });
            if(response.ok) {
                const data = await response.json();
                setSongs(data || []);
            }
        };
        fetchSongs();
    }, [filter]);

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
                    {songs.map(song => (
                        <tr key={song.id} className="bg-gray-800 border-b border-gray-700 hover:bg-gray-600">
                            <td className="px-6 py-4">
                                <button onClick={() => onPlay(song, songs)} title="Play song">
                                    <svg className="w-6 h-6 text-teal-400 hover:text-teal-200" fill="currentColor" viewBox="0 0 20 20"><path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM9.555 7.168A1 1 0 008 8v4a1 1 0 001.555.832l3-2a1 1 0 000-1.664l-3-2z" clipRule="evenodd"></path></svg>
                                </button>
                            </td>
                            <td className="px-6 py-4 font-medium text-white">{song.title}</td>
                            <td className="px-6 py-4">{song.artist}</td>
                            <td className="px-6 py-4">{song.album}</td>
                        </tr>
                    ))}
                </tbody>
            </table>
        </div>
    );
}

function Albums({ filter, onNavigate }) {
    const [albums, setAlbums] = useState([]);
    useEffect(() => {
        const fetchAlbums = async () => {
            const token = localStorage.getItem('token');
            let url = '/api/v1/music/albums';
            if (filter) {
                url += `?artist=${encodeURIComponent(filter)}`;
            }
            const response = await fetch(url, { headers: { 'Authorization': `Bearer ${token}` } });
            if(response.ok) {
                const data = await response.json();
                setAlbums(data || []);
            }
        };
        fetchAlbums();
    }, [filter]);

    return (
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-6">
            {albums.map((album, index) => (
                <button key={index} onClick={() => onNavigate({ page: 'songs', title: album.name, filter: { album: album.name, artist: album.artist } })} className="bg-gray-800 rounded-lg p-4 text-center hover:bg-gray-700 transition-colors">
                    <div className="w-full h-auto bg-gray-700 rounded aspect-square flex items-center justify-center mb-2">
                        <svg className="w-1/2 h-1/2 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="1" d="M9 19V6l12-3v13M9 19c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zm12-3c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zM9 6l12-3"></path></svg>
                    </div>
                    <p className="font-bold text-white truncate">{album.name}</p>
                    <p className="text-sm text-gray-400 truncate">{album.artist}</p>
                </button>
            ))}
        </div>
    );
}

function Artists({ onNavigate }) {
    const [artists, setArtists] = useState([]);
     useEffect(() => {
        const fetchArtists = async () => {
            const token = localStorage.getItem('token');
            const response = await fetch('/api/v1/music/artists', { headers: { 'Authorization': `Bearer ${token}` } });
            if(response.ok) {
                const data = await response.json();
                setArtists(data || []);
            }
        };
        fetchArtists();
    }, []);

    return (
        <ul className="space-y-2">
            {artists.map((artist, index) => (
                <li key={index}>
                    <button onClick={() => onNavigate({ page: 'albums', title: artist, filter: artist })} className="w-full text-left bg-gray-800 p-4 rounded-lg hover:bg-gray-700 transition-colors">
                        {artist}
                    </button>
                </li>
            ))}
        </ul>
    );
}

function Playlists() {
	return (<p>Playlist functionality is not yet implemented.</p>);
}

function AudioPlayer({ song, onEnded }) {
    const [audioSrc, setAudioSrc] = useState(null);
    const token = localStorage.getItem('token');

    useEffect(() => {
        if (!song) {
            setAudioSrc(null);
            return;
        }

        let objectUrl;
        const fetchAndSetAudio = async () => {
            try {
                const response = await fetch(`/api/v1/music/stream/${song.id}`, {
                    headers: { 'Authorization': `Bearer ${token}` }
                });
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
    }, [song, token]);

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


// --- File Browser Component ---
function FileBrowser({ onSelect, onClose }) {
    const [currentPath, setCurrentPath] = useState('/');
    const [items, setItems] = useState([]);
    const [error, setError] = useState('');

    const fetchDirectory = useCallback(async (path) => {
        setError('');
        const token = localStorage.getItem('token');
        try {
            const response = await fetch(`/api/v1/admin/browse?path=${encodeURIComponent(path)}`, {
                headers: { 'Authorization': `Bearer ${token}` }
            });
            if (!response.ok) {
                const data = await response.json();
                throw new Error(data.error || `Server error: ${response.status}`);
            }
            const data = await response.json();
            let dirItems = (data.items || []).filter(i => i.type === 'dir');
            if (path !== '/' && !dirItems.some(i => i.name === '..')) {
                dirItems.unshift({ name: '..', type: 'dir' });
            }
            setItems(dirItems);
            setCurrentPath(data.path || path);
        } catch (err) {
            setError(err.message);
            setItems([]);
        }
    }, []);

    useEffect(() => {
        fetchDirectory('/');
    }, [fetchDirectory]);

    const handleItemClick = (item) => {
        let newPath;
        if (item.name === '..') {
            if (currentPath === '/') return;
            newPath = currentPath.split('/').slice(0, -1).join('/') || '/';
        } else {
            newPath = currentPath === '/' ? `/${item.name}` : `${currentPath}/${item.name}`;
        }
        fetchDirectory(newPath);
    };

    return (
        <Modal onClose={onClose}>
             <h3 className="text-xl font-bold mb-4 text-teal-400">Browse For Folder</h3>
             <div className="bg-gray-900 p-2 rounded mb-4 font-mono text-sm">Path: {currentPath}</div>
             {error && <p className="text-red-500 mb-4">Error: {error}</p>}
             <ul className="h-64 overflow-y-auto border border-gray-700 rounded p-2 mb-4">
                 {items.map((item, index) => (
                     <li key={index} onClick={() => handleItemClick(item)} className="p-2 hover:bg-gray-700 rounded cursor-pointer flex items-center"><svg className="w-5 h-5 mr-2 text-teal-400" fill="none" stroke="currentColor" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z"></path></svg>{item.name}</li>
                 ))}
             </ul>
             <div className="flex justify-end space-x-4">
                 <button onClick={onClose} className="bg-gray-600 hover:bg-gray-700 text-white font-bold py-2 px-4 rounded">Cancel</button>
                 <button onClick={() => onSelect(currentPath)} className="bg-teal-500 hover:bg-teal-600 text-white font-bold py-2 px-4 rounded">Select Folder</button>
             </div>
        </Modal>
    );
}

// --- Admin Panel Component ---
function AdminPanel() {
	return (
		<div className="grid grid-cols-1 md:grid-cols-2 gap-8">
            <LibraryManagement />
            <UserManagement />
        </div>
	);
}

function LibraryManagement() {
    const [path, setPath] = useState('');
	const [message, setMessage] = useState('');
    const [isScanning, setIsScanning] = useState(false);
	const [showBrowser, setShowBrowser] = useState(false);

    const startScan = async () => {
        if (!path) {
            setMessage('Please select a directory to scan first.');
            return;
        }
		setMessage('');
        setIsScanning(true);
		const token = localStorage.getItem('token');
		try {
			const response = await fetch('/api/v1/admin/library/scan', {
				method: 'POST',
				headers: {
                    'Authorization': `Bearer ${token}`,
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({ path })
			});
			const data = await response.json();
			setMessage(response.ok ? data.message : `Error: ${data.error}`);
		} catch (err) {
			setMessage('Network error during scan');
		} finally {
            setIsScanning(false);
        }
	};

    return (
        <div className="bg-gray-800 p-6 rounded-lg">
            <h3 className="text-xl font-bold mb-4">Library Management</h3>
            <div className="flex flex-col space-y-4">
                <div className="flex space-x-2">
                    <input type="text" value={path} placeholder="Select a library path by browsing..." className="flex-grow p-2 bg-gray-700 rounded border border-gray-600 focus:outline-none focus:border-teal-500" readOnly />
                    <button onClick={() => setShowBrowser(true)} disabled={isScanning} className="bg-blue-600 hover:bg-blue-700 text-white font-bold py-2 px-4 rounded disabled:bg-blue-400 disabled:cursor-not-allowed">Browse</button>
                </div>
                <button onClick={startScan} disabled={isScanning} className="bg-green-600 hover:bg-green-700 text-white font-bold py-2 px-4 rounded disabled:bg-green-400 disabled:cursor-not-allowed">
                    {isScanning ? 'Scanning...' : 'Scan Library'}
                </button>
                {message && <p className="text-sm text-center mt-2 p-3 bg-gray-700 rounded">{message}</p>}
            </div>
            {showBrowser && <FileBrowser
				onSelect={(selectedPath) => { setPath(selectedPath); setShowBrowser(false); }}
				onClose={() => setShowBrowser(false)}
			/>}
        </div>
    );
}

function UserManagement() {
	const [users, setUsers] = useState([]);
	const [editingUser, setEditingUser] = useState(null);
    const [isCreatingUser, setIsCreatingUser] = useState(false);
	const [error, setError] = useState('');

	const fetchUsers = useCallback(async () => {
		const token = localStorage.getItem('token');
		try {
			const response = await fetch('/api/v1/admin/users', {
				headers: { 'Authorization': `Bearer ${token}` }
			});
			if (response.ok) {
				const data = await response.json();
				setUsers(data || []);
			} else {
				const data = await response.json();
				setError(data.error || 'Failed to fetch users');
			}
		} catch (err) {
			setError('Network error');
		}
	}, []);

	useEffect(() => {
		fetchUsers();
	}, [fetchUsers]);

    const handleCreate = async (userData) => {
        const token = localStorage.getItem('token');
        try {
            const response = await fetch('/api/v1/admin/users', {
                method: 'POST',
                headers: { 'Authorization': `Bearer ${token}`, 'Content-Type': 'application/json' },
                body: JSON.stringify(userData)
            });
            if (response.ok) {
                setIsCreatingUser(false);
                fetchUsers();
            } else {
                const data = await response.json();
                alert(`Error: ${data.error}`);
            }
        } catch (err) {
            alert('Network Error');
        }
    };

	const handlePasswordChange = async (userId, password) => {
        const token = localStorage.getItem('token');
        try {
            const response = await fetch(`/api/v1/admin/users/${userId}/password`, {
                method: 'PUT',
                headers: { 'Authorization': `Bearer ${token}`, 'Content-Type': 'application/json' },
                body: JSON.stringify({ password })
            });
            if (response.ok) {
                setEditingUser(null);
            } else {
                 const data = await response.json();
                alert(`Error: ${data.error}`);
            }
        } catch (err) {
            alert('Network Error');
        }
    };

	const handleDelete = async (userId) => {
		if (window.confirm('Are you sure you want to delete this user?')) {
			const token = localStorage.getItem('token');
			try {
				const response = await fetch(`/api/v1/admin/users/${userId}`, {
					method: 'DELETE',
					headers: { 'Authorization': `Bearer ${token}` }
				});
				if(response.ok) {
					fetchUsers();
				} else {
                    const data = await response.json();
                    alert(`Error: ${data.error}`);
                }
			} catch (err) {
                alert('Network Error');
			}
		}
	};

	return (
		<div className="bg-gray-800 p-6 rounded-lg">
			<div className="flex justify-between items-center mb-4">
				<h3 className="text-xl font-bold">User Management</h3>
				<button onClick={() => setIsCreatingUser(true)} className="bg-indigo-600 hover:bg-indigo-700 text-white font-bold py-2 px-4 rounded">Create User</button>
			</div>
			{error && <p className="text-red-500">{error}</p>}
			<div className="overflow-x-auto">
				<table className="min-w-full text-sm text-left text-gray-400">
					<thead className="text-xs text-gray-300 uppercase bg-gray-700">
						<tr>
							<th scope="col" className="px-6 py-3">Username</th>
							<th scope="col" className="px-6 py-3">Admin</th>
							<th scope="col" className="px-6 py-3 text-right">Actions</th>
						</tr>
					</thead>
					<tbody>
						{users.map(user => (
							<tr key={user.id} className="bg-gray-800 border-b border-gray-700 hover:bg-gray-600">
								<td className="px-6 py-4 font-medium text-white">{user.username}</td>
								<td className="px-6 py-4">{user.is_admin ? 'Yes' : 'No'}</td>
								<td className="px-6 py-4 text-right space-x-2">
									<button onClick={() => setEditingUser(user)} className="font-medium text-blue-500 hover:underline">Edit Password</button>
									<button onClick={() => handleDelete(user.id)} className="font-medium text-red-500 hover:underline">Delete</button>
								</td>
							</tr>
						))}
					</tbody>
				</table>
			</div>
            {isCreatingUser && (
                <UserFormModal
                    onClose={() => setIsCreatingUser(false)}
                    onSubmit={handleCreate}
                    title="Create New User"
                />
            )}
			{editingUser && (
				<PasswordEditModal
					user={editingUser}
					onClose={() => setEditingUser(null)}
					onSubmit={handlePasswordChange}
				/>
			)}
		</div>
	);
}

const UserFormModal = ({ onClose, onSubmit, title }) => {
    const [username, setUsername] = useState('');
    const [password, setPassword] = useState('');
    const [isAdmin, setIsAdmin] = useState(false);

    const handleSubmit = (e) => {
        e.preventDefault();
        onSubmit({ username, password, is_admin: isAdmin });
    };

    return (
        <Modal onClose={onClose}>
            <h3 className="text-xl font-bold mb-4">{title}</h3>
            <form onSubmit={handleSubmit}>
                <div className="mb-4">
                    <label className="block text-gray-400 mb-2">Username</label>
                    <input type="text" value={username} onChange={e => setUsername(e.target.value)} className="w-full p-2 bg-gray-700 rounded" required/>
                </div>
                <div className="mb-4">
                    <label className="block text-gray-400 mb-2">Password</label>
                    <input type="password" value={password} onChange={e => setPassword(e.target.value)} className="w-full p-2 bg-gray-700 rounded" required />
                </div>
                <div className="mb-4 flex items-center">
                    <input type="checkbox" checked={isAdmin} onChange={e => setIsAdmin(e.target.checked)} id="isAdminCheck" className="w-4 h-4 text-teal-600 bg-gray-700 border-gray-600 rounded focus:ring-teal-500" />
                    <label htmlFor="isAdminCheck" className="ml-2 text-sm font-medium text-gray-300">Is Admin?</label>
                </div>
                <div className="flex justify-end space-x-4">
                    <button type="button" onClick={onClose} className="bg-gray-600 hover:bg-gray-700 text-white font-bold py-2 px-4 rounded">Cancel</button>
                    <button type="submit" className="bg-teal-500 hover:bg-teal-600 text-white font-bold py-2 px-4 rounded">Save</button>
                </div>
            </form>
        </Modal>
    )
}

const PasswordEditModal = ({ user, onClose, onSubmit }) => {
	const [password, setPassword] = useState('');
	const handleSubmit = (e) => {
		e.preventDefault();
		onSubmit(user.id, password);
	};
	return (
		<Modal onClose={onClose}>
			<h3 className="text-xl font-bold mb-4">Edit Password for {user.username}</h3>
			<form onSubmit={handleSubmit}>
				<div className="mb-4">
					<label className="block text-gray-400 mb-2">New Password</label>
					<input
						type="password"
						value={password}
						onChange={(e) => setPassword(e.target.value)}
						className="w-full p-2 bg-gray-700 rounded"
						required
					/>
				</div>
                <div className="flex justify-end space-x-4">
                    <button type="button" onClick={onClose} className="bg-gray-600 hover:bg-gray-700 text-white font-bold py-2 px-4 rounded">Cancel</button>
                    <button type="submit" className="bg-teal-500 hover:bg-teal-600 text-white font-bold py-2 px-4 rounded">Update Password</button>
                </div>
			</form>
		</Modal>
	);
};

export default App;

