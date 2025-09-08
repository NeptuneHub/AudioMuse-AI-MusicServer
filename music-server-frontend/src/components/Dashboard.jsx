// Suggested path: music-server-frontend/src/components/Dashboard.jsx
import React, { useState, useMemo, useCallback, useEffect } from 'react';
import { Songs, Albums, Artists } from './MusicViews.jsx';
import Playlists from './Playlists.jsx';
import AdminPanel from './AdminPanel.jsx';
import CustomAudioPlayer from './AudioPlayer.jsx';
import PlayQueueView from './PlayQueueView.jsx';

function Dashboard({ onLogout, isAdmin, credentials }) {
    const [navigation, setNavigation] = useState([{ page: 'artists', title: 'Artists' }]);
    const [playQueue, setPlayQueue] = useState([]);
    const [currentTrackIndex, setCurrentTrackIndex] = useState(0);
    const [isMenuOpen, setIsMenuOpen] = useState(false);
    const [isQueueViewOpen, setQueueViewOpen] = useState(false);
    const [audioMuseUrl, setAudioMuseUrl] = useState('');
    
    const currentView = useMemo(() => navigation[navigation.length - 1], [navigation]);
    const currentSong = useMemo(() => playQueue.length > 0 ? playQueue[currentTrackIndex] : null, [playQueue, currentTrackIndex]);

    const fetchConfig = useCallback(async () => {
        if (!isAdmin) return;
        try {
            const token = localStorage.getItem('token');
            const response = await fetch(`/rest/getConfiguration.view?f=json`, {
                headers: { 'Authorization': `Bearer ${token}` }
            });
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
    }, [isAdmin]);

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
            // Avoid adding duplicates
            if (prevQueue.find(s => s.id === song.id)) {
                return prevQueue;
            }
            // If the queue is empty, start playing the new song
            if (prevQueue.length === 0) {
                setCurrentTrackIndex(0);
            }
            return [...prevQueue, song];
        });
    }, []);

    const handleRemoveFromQueue = useCallback((indexToRemove) => {
        setPlayQueue(prevQueue => {
            const newQueue = prevQueue.filter((_, index) => index !== indexToRemove);
            
            if (newQueue.length === 0) {
                 // If the queue is now empty, reset the index.
                setCurrentTrackIndex(0);
                return [];
            }

            if (indexToRemove < currentTrackIndex) {
                // If removing a song before the current one, decrement the index
                setCurrentTrackIndex(prev => prev - 1);
            } else if (indexToRemove === currentTrackIndex) {
                // If removing the current song, and it's the last song, wrap around to the start
                if (currentTrackIndex >= newQueue.length) {
                    setCurrentTrackIndex(0); 
                }
            }
            return newQueue;
        });
    }, [currentTrackIndex]);
    
    const handleRemoveSongById = useCallback((songId) => {
        const indexToRemove = playQueue.findIndex(s => s.id === songId);
        if (indexToRemove > -1) {
            handleRemoveFromQueue(indexToRemove);
        }
    }, [playQueue, handleRemoveFromQueue]);


    const handleSelectTrack = useCallback((index) => {
        setCurrentTrackIndex(index);
        setQueueViewOpen(false); // Close queue view when a new track is selected
    }, []);


    const handlePlayNext = useCallback(() => {
        if (playQueue.length > 0) {
            setCurrentTrackIndex(prev => (prev + 1) % playQueue.length);
        }
    }, [playQueue.length]);

    const handlePlayPrevious = useCallback(() => {
        if (playQueue.length > 0) {
            setCurrentTrackIndex(prev => (prev - 1 + playQueue.length) % playQueue.length);
        }
    }, [playQueue.length]);
    
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
			{/* The main content area now has padding-bottom to prevent overlap with the fixed player */}
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
					{currentView.page === 'songs' && <Songs credentials={credentials} filter={currentView.filter} onPlay={handlePlaySong} onAddToQueue={handleAddToQueue} onRemoveFromQueue={handleRemoveSongById} playQueue={playQueue} currentSong={currentSong} onNavigate={handleNavigate} audioMuseUrl={audioMuseUrl} />}
					{currentView.page === 'albums' && <Albums credentials={credentials} filter={currentView.filter} onNavigate={handleNavigate} />}
					{currentView.page === 'artists' && <Artists credentials={credentials} onNavigate={handleNavigate} />}
					{currentView.page === 'playlists' && <Playlists credentials={credentials} onNavigate={handleNavigate} />}
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
            />
		</div>
	);
}

export default Dashboard;

