// Suggested path: music-server-frontend/src/components/Dashboard.jsx
import React, { useState, useMemo, useCallback, useEffect } from 'react';
import { Songs, Albums, Artists, AddToPlaylistModal } from './MusicViews.jsx';
import SongAlchemy from './SongAlchemy.jsx';
import Map from './Map.jsx';
import Playlists from './Playlists.jsx';
import AdminPanel from './AdminPanel.jsx';
import CustomAudioPlayer from './AudioPlayer.jsx';
import PlayQueueView from './PlayQueueView.jsx';
import { subsonicFetch } from '../api';
import { useKeyboardShortcuts } from '../hooks/useKeyboardShortcuts';


function Dashboard({ onLogout, isAdmin, credentials }) {
    // Initialize navigation from localStorage or default to artists
    const [navigation, setNavigation] = useState(() => {
        try {
            const savedNavigation = localStorage.getItem('currentNavigation');
            if (savedNavigation) {
                const parsed = JSON.parse(savedNavigation);
                // Validate the parsed navigation structure
                if (Array.isArray(parsed) && parsed.length > 0 && parsed[0].page && parsed[0].title) {
                    // Check if user has access to admin page if that's where they were
                    const hasAdminPage = parsed.some(nav => nav.page === 'admin');
                    if (hasAdminPage && !isAdmin) {
                        // User was on admin page but is no longer admin, reset to artists
                        return [{ page: 'artists', title: 'Artists' }];
                    }
                    return parsed;
                }
            }
        } catch (error) {
            console.warn('Failed to restore navigation from localStorage:', error);
        }
        return [{ page: 'artists', title: 'Artists' }];
    });
    const [playQueue, setPlayQueue] = useState([]);
    const [currentTrackIndex, setCurrentTrackIndex] = useState(0);
    const [isMenuOpen, setIsMenuOpen] = useState(false);
    const [isQueueViewOpen, setQueueViewOpen] = useState(false);
    const [audioMuseUrl, setAudioMuseUrl] = useState('');
    const [selectedSongForPlaylist, setSelectedSongForPlaylist] = useState(null);
    const [mixMessage, setMixMessage] = useState('');
    const [isAudioPlaying, setIsAudioPlaying] = useState(false);
    
    const currentView = useMemo(() => navigation[navigation.length - 1], [navigation]);
    const currentSong = useMemo(() => playQueue.length > 0 ? playQueue[currentTrackIndex] : null, [playQueue, currentTrackIndex]);

    const fetchConfig = useCallback(async () => {
        try {
            const dataResponse = await subsonicFetch('getConfiguration.view', { f: 'json' });
            if (dataResponse && dataResponse.status === 'ok') {
                const configList = dataResponse?.configurations?.configuration || [];
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

    // Scroll to top when component mounts (after login)
    useEffect(() => {
        window.scrollTo(0, 0);
    }, []);

    // Track audio playing state
    useEffect(() => {
        const audio = document.querySelector('audio');
        if (!audio) return;

        const handlePlay = () => setIsAudioPlaying(true);
        const handlePause = () => setIsAudioPlaying(false);
        const handleEnded = () => setIsAudioPlaying(false);

        audio.addEventListener('play', handlePlay);
        audio.addEventListener('pause', handlePause);
        audio.addEventListener('ended', handleEnded);

        // Initialize state based on current audio state
        setIsAudioPlaying(!audio.paused);

        return () => {
            audio.removeEventListener('play', handlePlay);
            audio.removeEventListener('pause', handlePause);
            audio.removeEventListener('ended', handleEnded);
        };
    }, [currentSong]); // Re-attach when song changes

    // Save navigation state to localStorage whenever it changes
    useEffect(() => {
        try {
            localStorage.setItem('currentNavigation', JSON.stringify(navigation));
        } catch (error) {
            console.warn('Failed to save navigation to localStorage:', error);
        }
    }, [navigation]);

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
            const data = await subsonicFetch('getSimilarSongs.view', { id: song.id, count: 20 });
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
            const data = await subsonicFetch('getSongPath.view', { startId, endId });
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

    // Toggle play/pause for currently playing song
    const handleTogglePlayPause = useCallback(() => {
        const audio = document.querySelector('audio');
        if (audio) {
            if (audio.paused) {
                audio.play().catch(e => console.error('Play failed:', e));
            } else {
                audio.pause();
            }
        }
    }, []);

    // Keyboard shortcuts (inspired by LMS)
    useKeyboardShortcuts({
        onPlayPause: handleTogglePlayPause,
        onPrevious: handlePlayPrevious,
        onNext: handlePlayNext,
        onVolumeUp: () => {
            const audio = document.querySelector('audio');
            if (audio && audio.volume < 1) {
                audio.volume = Math.min(1, audio.volume + 0.1);
            }
        },
        onVolumeDown: () => {
            const audio = document.querySelector('audio');
            if (audio && audio.volume > 0) {
                audio.volume = Math.max(0, audio.volume - 0.1);
            }
        }
    });
    
    // --- Navigation ---
    const NavLink = ({ page, title, children }) => {
		const isActive = currentView.page === page && navigation.length === 1;
		return (
			<button 
				onClick={() => handleResetNavigation(page, title)} 
				className={`relative w-full md:w-auto text-left px-3 lg:px-4 py-2 rounded-lg font-semibold transition-all duration-300 text-sm lg:text-base ${
					isActive 
						? 'bg-gradient-accent text-white shadow-glow' 
						: 'text-gray-300 hover:bg-dark-700 hover:text-white'
				}`}
			>
				{children}
				{isActive && (
					<span className="absolute bottom-0 left-1/2 transform -translate-x-1/2 w-1/2 h-0.5 bg-accent-400"></span>
				)}
			</button>
		);
	};

	return (
		<div className="bg-dark-800 min-h-screen">
			{/* Enhanced Navigation Bar */}
			<nav className="glass fixed top-0 left-0 right-0 z-20 border-b border-dark-600">
				<div className="container mx-auto px-4 sm:px-6 py-2 sm:py-3 flex justify-between items-center">
					{/* Title only */}
					<h1 className="text-lg sm:text-xl font-bold bg-gradient-to-r from-accent-400 to-accent-600 bg-clip-text text-transparent">
						AudioMuse-AI
					</h1>
						
					{/* Desktop Navigation */}
					<div className="hidden md:flex items-center space-x-1 lg:space-x-2">
						<NavLink page="artists" title="Artists">Artists</NavLink>
						<NavLink page="albums" title="All Albums">Albums</NavLink>
						<NavLink page="songs" title="Songs">Songs</NavLink>
						<NavLink page="alchemy" title="Alchemy">Alchemy</NavLink>
						<NavLink page="map" title="Map">Map</NavLink>
						<NavLink page="playlists" title="Playlists">Playlists</NavLink>
						{isAdmin && <NavLink page="admin" title="Admin Panel">Admin</NavLink>}
						
						{/* Keyboard shortcuts help button */}
						<button 
							title="Keyboard Shortcuts: Space (Play/Pause), Ctrl+← (Previous), Ctrl+→ (Next), Ctrl+↑ (Vol+), Ctrl+↓ (Vol-)"
							className="px-2 py-2 text-gray-400 hover:text-accent-400 transition-colors rounded-lg"
						>
							<svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path>
							</svg>
						</button>
						
						<button 
							onClick={onLogout} 
							className="ml-2 px-4 py-2 rounded-lg bg-red-600 hover:bg-red-700 text-white font-semibold transition-all duration-300 shadow-md hover:shadow-lg text-sm lg:text-base"
						>
							Logout
						</button>
					</div>
					
					{/* Mobile Menu Button */}
					<div className="md:hidden">
						<button 
							onClick={() => setIsMenuOpen(!isMenuOpen)} 
							className="text-gray-300 hover:text-white focus:outline-none p-2 rounded-lg hover:bg-dark-700 transition-all"
						>
							<svg className={`w-6 h-6 transition-transform ${isMenuOpen ? 'rotate-90' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
								{isMenuOpen ? (
									<path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12"></path>
								) : (
									<path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M4 6h16M4 12h16m-7 6h7"></path>
								)}
							</svg>
						</button>
					</div>
				</div>
				
				{/* Mobile Navigation Menu */}
				{isMenuOpen && (
					<div className="md:hidden px-4 pt-2 pb-4 space-y-2 border-t border-dark-600 bg-dark-750 animate-slide-up">
						<NavLink page="artists" title="Artists">Artists</NavLink>
						<NavLink page="albums" title="All Albums">Albums</NavLink>
						<NavLink page="songs" title="Songs">Songs</NavLink>
						<NavLink page="alchemy" title="Alchemy">Alchemy</NavLink>
						<NavLink page="map" title="Map">Map</NavLink>
						<NavLink page="playlists" title="Playlists">Playlists</NavLink>
						{isAdmin && <NavLink page="admin" title="Admin Panel">Admin</NavLink>}
						<button 
							onClick={onLogout} 
							className="w-full text-left px-4 py-2 rounded-lg bg-red-600 hover:bg-red-700 text-white font-semibold transition-all duration-300"
						>
							Logout
						</button>
					</div>
				)}
			</nav>
			
			{/* Main Content - optimized padding for navbar and audio bar */}
			<main className="px-3 sm:px-6 pt-16 sm:pt-20 pb-20 sm:pb-24 bg-dark-800 min-h-screen">
				{navigation.length > 1 && (
					<button 
						onClick={handleBack} 
						className="flex items-center gap-2 text-accent-400 hover:text-accent-300 font-semibold mb-6 transition-all group"
					>
						<svg className="w-5 h-5 group-hover:-translate-x-1 transition-transform" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M15 19l-7-7 7-7"></path>
						</svg>
						Back
					</button>
				)}
				{mixMessage && (
					<div className="max-w-2xl mx-auto mb-6 p-4 bg-accent-500/10 border border-accent-500/50 rounded-lg text-center animate-fade-in">
						<p className="text-accent-400 font-medium">{mixMessage}</p>
					</div>
				)}
                    {currentView.page === 'songs' && <Songs credentials={credentials} filter={currentView.filter} onPlay={handlePlaySong} onTogglePlayPause={handleTogglePlayPause} onAddToQueue={handleAddToQueue} onRemoveFromQueue={handleRemoveSongById} playQueue={playQueue} currentSong={currentSong} isAudioPlaying={isAudioPlaying} onNavigate={handleNavigate} audioMuseUrl={audioMuseUrl} onInstantMix={handleInstantMix} onAddToPlaylist={setSelectedSongForPlaylist} />}
                    {currentView.page === 'albums' && <Albums credentials={credentials} filter={currentView.filter} onNavigate={handleNavigate} />}
                    {currentView.page === 'artists' && <Artists credentials={credentials} onNavigate={handleNavigate} />}
                    {currentView.page === 'playlists' && <Playlists credentials={credentials} isAdmin={isAdmin} onNavigate={handleNavigate} />}
                    {currentView.page === 'alchemy' && <SongAlchemy onNavigate={handleNavigate} onAddToQueue={handleAddToQueue} onPlay={handlePlaySong} />}
                    {currentView.page === 'map' && <Map onNavigate={handleNavigate} onAddToQueue={handleAddToQueue} onPlay={handlePlaySong} onRemoveFromQueue={handleRemoveFromQueue} onClearQueue={handleClearQueue} playQueue={playQueue} />}
                    {currentView.page === 'admin' && isAdmin && <AdminPanel onConfigChange={fetchConfig} />}
				</main>

            <CustomAudioPlayer
                song={currentSong}
                onPlayNext={handlePlayNext}
                onPlayPrevious={handlePlayPrevious}
                onEnded={handlePlayNext}
                credentials={credentials}
                hasQueue={playQueue.length > 1}
                onToggleQueueView={() => setQueueViewOpen(true)}
                queueCount={playQueue.length}
            />

            <PlayQueueView
                isOpen={isQueueViewOpen}
                onClose={() => setQueueViewOpen(false)}
                queue={playQueue}
                currentIndex={currentTrackIndex}
                onRemove={handleRemoveFromQueue}
                onSelect={handleSelectTrack}
                onTogglePlayPause={handleTogglePlayPause}
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

