// Suggested path: music-server-frontend/src/components/Dashboard.jsx
import React, { useState, useMemo, useCallback, useEffect } from 'react';
import { Songs, Albums, Artists, AddToPlaylistModal } from './MusicViews.jsx';
import Playlists from './Playlists.jsx';
import AdminPanel from './AdminPanel.jsx';
import CustomAudioPlayer from './AudioPlayer.jsx';
import PlayQueueView from './PlayQueueView.jsx';

// This needs to be defined once, preferably in a separate api.js file, but here for simplicity.
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


function Dashboard({ onLogout, isAdmin, credentials }) {
    const [navigation, setNavigation] = useState([{ page: 'artists', title: 'Artists' }]);
    const [playQueue, setPlayQueue] = useState([]);
    const [currentTrackIndex, setCurrentTrackIndex] = useState(0);
    const [isMenuOpen, setIsMenuOpen] = useState(false);
    const [isQueueViewOpen, setQueueViewOpen] = useState(false);
    const [audioMuseUrl, setAudioMuseUrl] = useState('');
    const [selectedSongForPlaylist, setSelectedSongForPlaylist] = useState(null);
    const [mixMessage, setMixMessage] = useState('');
    
    const currentView = useMemo(() => navigation[navigation.length - 1], [navigation]);
    const currentSong = useMemo(() => playQueue.length > 0 ? playQueue[currentTrackIndex] : null, [playQueue, currentTrackIndex]);

    const fetchConfig = useCallback(async () => {
        try {
            const token = localStorage.getItem('token');
            const headers = {};
            if (token) headers['Authorization'] = `Bearer ${token}`;
            const response = await fetch(`/rest/getConfiguration.view?f=json`, { headers });
            const data = await response.json();
            const subsonicResponse = data["subsonic-response"];
            if (subsonicResponse.status === 'ok') {
                const configList = subsonicResponse?.configurations?.configuration || [];
                 const urlConfig = Array.isArray(configList) 
                    ? configList.find(c => c.name === 'audiomuse_ai_core_url')
                    : (configList.name === 'audiomuse_ai_core_url' ? configList : null);

                setAudioMuseUrl(urlConfig?.value || '');
            }
        } catch (e) {
            console.error("Failed to fetch AudioMuse URL config", e);
        }
    }, []);

    useEffect(() => {
        fetchConfig();
    }, [fetchConfig]);

    const handleNavigate = (newView) => {
        setNavigation(prev => [...prev, newView]);
        setIsMenuOpen(false);
    };
    const handleBack = () => navigation.length > 1 && setNavigation(prev => prev.slice(0, -1));
    const handleResetNavigation = (page, title) => {
        setNavigation([{ page, title }]);
        setIsMenuOpen(false);
    }

    // --- Queue Management ---
    const handlePlaySong = useCallback((song, songList) => {
        const queue = songList || [song];
        const songIndex = queue.findIndex(s => s.id === song.id);
        setPlayQueue(queue);
        setCurrentTrackIndex(songIndex >= 0 ? songIndex : 0);
    }, []);

    const handleAddToQueue = useCallback((song) => {
        setPlayQueue(prevQueue => {
            if (prevQueue.find(s => s.id === song.id)) return prevQueue;
            if (prevQueue.length === 0) setCurrentTrackIndex(0);
            return [...prevQueue, song];
        });
    }, []);

    const handleRemoveFromQueue = useCallback((indexToRemove) => {
        setPlayQueue(prevQueue => {
            const newQueue = prevQueue.filter((_, index) => index !== indexToRemove);
            if (newQueue.length === 0) {
                setCurrentTrackIndex(0);
                return [];
            }
            if (indexToRemove < currentTrackIndex) {
                setCurrentTrackIndex(prev => prev - 1);
            } else if (indexToRemove === currentTrackIndex && currentTrackIndex >= newQueue.length) {
                setCurrentTrackIndex(0); 
            }
            return newQueue;
        });
    }, [currentTrackIndex]);

    const handleClearQueue = useCallback(() => {
        setPlayQueue([]);
        setCurrentTrackIndex(0);
    }, []);
    
    const handleReorderQueue = useCallback((oldIndex, newIndex) => {
        if (newIndex < 0 || newIndex >= playQueue.length) return;

        setPlayQueue(prevQueue => {
            const newQueue = [...prevQueue];
            const [movedItem] = newQueue.splice(oldIndex, 1);
            newQueue.splice(newIndex, 0, movedItem);

            const currentSongId = prevQueue[currentTrackIndex]?.id;
            if (currentSongId) {
                const newPlayingIndex = newQueue.findIndex(s => s.id === currentSongId);
                if (newPlayingIndex !== -1) {
                    setCurrentTrackIndex(newPlayingIndex);
                }
            }
            return newQueue;
        });
    }, [playQueue, currentTrackIndex]);

    const handleRemoveSongById = useCallback((songId) => {
        const indexToRemove = playQueue.findIndex(s => s.id === songId);
        if (indexToRemove > -1) handleRemoveFromQueue(indexToRemove);
    }, [playQueue, handleRemoveFromQueue]);

    const handleSelectTrack = useCallback((index) => {
        setCurrentTrackIndex(index);
        setQueueViewOpen(false);
    }, []);

    const handlePlayNext = useCallback(() => {
        if (playQueue.length > 0) setCurrentTrackIndex(prev => (prev + 1) % playQueue.length);
    }, [playQueue.length]);

    const handlePlayPrevious = useCallback(() => {
        if (playQueue.length > 0) setCurrentTrackIndex(prev => (prev - 1 + playQueue.length) % playQueue.length);
    }, [playQueue.length]);

    const handleInstantMix = async (song) => {
        if (!audioMuseUrl) return;

        setMixMessage(`Generating Instant Mix for "${song.title}"...`);
        setQueueViewOpen(false); // Close queue if it's open
        try {
            const data = await subsonicFetch('getSimilarSongs.view', credentials, { id: song.id, count: 20 });
            let similarSongs = data.directory?.song || [];
            similarSongs = Array.isArray(similarSongs) ? similarSongs : [similarSongs].filter(Boolean);

            const newQueue = [song, ...similarSongs];
            handlePlaySong(song, newQueue); 
            
            handleNavigate({
                page: 'songs',
                title: `Instant Mix: ${song.title}`,
                filter: { similarToSongId: song.id, preloadedSongs: newQueue } 
            });
            setMixMessage('');
        } catch (error) {
            console.error("Failed to create Instant Mix:", error);
            setMixMessage('Error creating Instant Mix.');
            setTimeout(() => setMixMessage(''), 3000);
        }
    };

    const handleCreateSongPath = async (startId, endId) => {
        if (!audioMuseUrl) return;
        setMixMessage(`Creating Song Path...`);
        setQueueViewOpen(false);
        try {
            const data = await subsonicFetch('getSongPath.view', credentials, { startId, endId });
            let pathSongs = data.directory?.song || [];
            pathSongs = Array.isArray(pathSongs) ? pathSongs : [pathSongs].filter(Boolean);

            if (pathSongs.length > 0) {
                // This function correctly replaces the entire queue and starts playing the first song.
                handlePlaySong(pathSongs[0], pathSongs);
                // This function navigates to the songs view with the new list preloaded.
                handleNavigate({ page: 'songs', title: `Song Path`, filter: { preloadedSongs: pathSongs } });
                setMixMessage('');
            } else {
                 setMixMessage('No path found between the selected songs.');
                 setTimeout(() => setMixMessage(''), 3000);
            }
        } catch (error) {
            console.error("Failed to create Song Path:", error);
            setMixMessage('Error creating Song Path.');
            setTimeout(() => setMixMessage(''), 3000);
        }
    };
    
    // --- Navigation ---
    const NavLink = ({ page, title, children }) => (
         <button 
            onClick={() => handleResetNavigation(page, title)} 
            className={`w-full text-left px-3 py-2 rounded font-semibold transition duration-300 ${currentView.page === page && navigation.length === 1 ? 'bg-teal-500 text-white' : 'text-gray-300 hover:bg-gray-700'}`}>
            {children}
        </button>
    );

	return (
		<div className="bg-gray-900">
			<div className="pb-24">
				<nav className="bg-gray-800 shadow-md sticky top-0 z-20">
					 <div className="container mx-auto px-4 sm:px-6 py-3 flex justify-between items-center">
						<h1 className="text-xl font-bold text-teal-400">AudioMuse-AI</h1>
						
						<div className="hidden md:flex items-center space-x-2">
							<NavLink page="artists" title="Artists">Artists</NavLink>
							<NavLink page="albums" title="All Albums">Albums</NavLink>
                            <NavLink page="songs" title="Songs">Songs</NavLink>
							<NavLink page="playlists" title="Playlists">Playlists</NavLink>
							{isAdmin && <NavLink page="admin" title="Admin Panel">Admin</NavLink>}
							<button onClick={onLogout} className="px-3 py-2 rounded bg-red-600 hover:bg-red-700 text-white font-semibold transition duration-300">Logout</button>
						</div>

						<div className="md:hidden">
							<button onClick={() => setIsMenuOpen(!isMenuOpen)} className="text-gray-300 hover:text-white focus:outline-none p-2 rounded-md">
								 <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M4 6h16M4 12h16m-7 6h7"></path></svg>
							</button>
						</div>
					</div>
					{isMenuOpen && (
						<div className="md:hidden px-2 pt-2 pb-3 space-y-1 sm:px-3">
							<NavLink page="artists" title="Artists">Artists</NavLink>
							<NavLink page="albums" title="All Albums">Albums</NavLink>
                            <NavLink page="songs" title="Songs">Songs</NavLink>
							<NavLink page="playlists" title="Playlists">Playlists</NavLink>
							{isAdmin && <NavLink page="admin" title="Admin Panel">Admin</NavLink>}
							<button onClick={onLogout} className="w-full text-left px-3 py-2 rounded bg-red-600 hover:bg-red-700 text-white font-semibold transition duration-300">Logout</button>
						</div>
					)}
				</nav>
				<main className="p-4 sm:p-8">
					<div className="mb-4">
						{navigation.length > 1 && (
							<button onClick={handleBack} className="text-teal-400 hover:text-teal-200 font-semibold mb-4">&larr; Back</button>
						)}
						<h2 className="text-3xl font-bold text-white">{currentView.title}</h2>
					</div>
                    {mixMessage && <p className="text-center text-teal-400 mb-4">{mixMessage}</p>}
					{currentView.page === 'songs' && <Songs credentials={credentials} filter={currentView.filter} onPlay={handlePlaySong} onAddToQueue={handleAddToQueue} onRemoveFromQueue={handleRemoveSongById} playQueue={playQueue} currentSong={currentSong} onNavigate={handleNavigate} audioMuseUrl={audioMuseUrl} onInstantMix={handleInstantMix} onAddToPlaylist={setSelectedSongForPlaylist} />}
					{currentView.page === 'albums' && <Albums credentials={credentials} filter={currentView.filter} onNavigate={handleNavigate} />}
					{currentView.page === 'artists' && <Artists credentials={credentials} onNavigate={handleNavigate} />}
                    {currentView.page === 'playlists' && <Playlists credentials={credentials} isAdmin={isAdmin} onNavigate={handleNavigate} />}
					{currentView.page === 'admin' && isAdmin && <AdminPanel onConfigChange={fetchConfig} />}
				</main>
			</div>

            <CustomAudioPlayer
                song={currentSong}
                onPlayNext={handlePlayNext}
                onPlayPrevious={handlePlayPrevious}
                onEnded={handlePlayNext}
                credentials={credentials}
                hasQueue={playQueue.length > 1}
                onToggleQueueView={() => setQueueViewOpen(true)}
            />

            <PlayQueueView
                isOpen={isQueueViewOpen}
                onClose={() => setQueueViewOpen(false)}
                queue={playQueue}
                currentIndex={currentTrackIndex}
                onRemove={handleRemoveFromQueue}
                onSelect={handleSelectTrack}
                onAddToPlaylist={setSelectedSongForPlaylist}
                onInstantMix={handleInstantMix}
                audioMuseUrl={audioMuseUrl}
                onClearQueue={handleClearQueue}
                onReorder={handleReorderQueue}
                onCreateSongPath={handleCreateSongPath}
            />
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

export default Dashboard;

