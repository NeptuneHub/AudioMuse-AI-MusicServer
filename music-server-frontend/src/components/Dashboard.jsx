// Suggested path: music-server-frontend/src/components/Dashboard.jsx
import React, { useState, useMemo, useCallback } from 'react';
import { Songs, Albums, Artists } from './MusicViews.jsx';
import Playlists from './Playlists.jsx';
import AdminPanel from './AdminPanel.jsx';
import AudioPlayer from './AudioPlayer.jsx';

function Dashboard({ onLogout, isAdmin, credentials }) {
    const [navigation, setNavigation] = useState([{ page: 'artists', title: 'Artists' }]);
    const [playQueue, setPlayQueue] = useState([]);
    const [currentTrackIndex, setCurrentTrackIndex] = useState(0);
    const [isMenuOpen, setIsMenuOpen] = useState(false); // State for mobile menu
    
    const currentView = useMemo(() => navigation[navigation.length - 1], [navigation]);
    const currentSong = useMemo(() => playQueue[currentTrackIndex], [playQueue, currentTrackIndex]);

    const handleNavigate = (newView) => {
        setNavigation(prev => [...prev, newView]);
        setIsMenuOpen(false); // Close menu on navigation
    };
    const handleBack = () => navigation.length > 1 && setNavigation(prev => prev.slice(0, -1));
    const handleResetNavigation = (page, title) => {
        setNavigation([{ page, title }]);
        setIsMenuOpen(false); // Close menu on navigation
    }

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
    
    const NavLink = ({ page, title, children }) => (
         <button 
            onClick={() => handleResetNavigation(page, title)} 
            className={`w-full text-left px-3 py-2 rounded font-semibold transition duration-300 ${currentView.page === page && navigation.length === 1 ? 'bg-teal-500 text-white' : 'text-gray-300 hover:bg-gray-700'}`}>
            {children}
        </button>
    );

	return (
		<div className="flex flex-col h-screen">
			<nav className="bg-gray-800 shadow-md">
                <div className="container mx-auto px-4 sm:px-6 py-3 flex justify-between items-center">
                    <h1 className="text-xl font-bold text-teal-400">AudioMuse-AI</h1>
                    
                    {/* Desktop Menu */}
                    <div className="hidden md:flex items-center space-x-2">
                        <NavLink page="songs" title="All Songs">Songs</NavLink>
                        <NavLink page="albums" title="All Albums">Albums</NavLink>
                        <NavLink page="artists" title="Artists">Artists</NavLink>
                        <NavLink page="playlists" title="Playlists">Playlists</NavLink>
                        {isAdmin && <NavLink page="admin" title="Admin Panel">Admin</NavLink>}
                        <button onClick={onLogout} className="px-3 py-2 rounded bg-red-600 hover:bg-red-700 text-white font-semibold transition duration-300">Logout</button>
                    </div>

                    {/* Mobile Menu Button */}
                    <div className="md:hidden">
                        <button onClick={() => setIsMenuOpen(!isMenuOpen)} className="text-gray-300 hover:text-white focus:outline-none">
                             <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M4 6h16M4 12h16m-7 6h7"></path></svg>
                        </button>
                    </div>
                </div>
                 {/* Mobile Menu Dropdown */}
                {isMenuOpen && (
                    <div className="md:hidden px-2 pt-2 pb-3 space-y-1 sm:px-3">
                        <NavLink page="songs" title="All Songs">Songs</NavLink>
                        <NavLink page="albums" title="All Albums">Albums</NavLink>
                        <NavLink page="artists" title="Artists">Artists</NavLink>
                        <NavLink page="playlists" title="Playlists">Playlists</NavLink>
                        {isAdmin && <NavLink page="admin" title="Admin Panel">Admin</NavLink>}
                        <button onClick={onLogout} className="w-full text-left px-3 py-2 rounded bg-red-600 hover:bg-red-700 text-white font-semibold transition duration-300">Logout</button>
                    </div>
                )}
			</nav>
			<main className="flex-1 p-4 sm:p-8 bg-gray-900 overflow-y-auto pb-24">
                <div className="mb-4">
                    {navigation.length > 1 && (
                        <button onClick={handleBack} className="text-teal-400 hover:text-teal-200 font-semibold mb-4">&larr; Back</button>
                    )}
                    <h2 className="text-3xl font-bold">{currentView.title}</h2>
                </div>
				{currentView.page === 'songs' && <Songs credentials={credentials} filter={currentView.filter} onPlay={handlePlaySong} currentSong={currentSong} />}
				{currentView.page === 'albums' && <Albums credentials={credentials} filter={currentView.filter} onNavigate={handleNavigate} />}
				{currentView.page === 'artists' && <Artists credentials={credentials} onNavigate={handleNavigate} />}
				{currentView.page === 'playlists' && <Playlists credentials={credentials} onNavigate={handleNavigate} />}
                {currentView.page === 'admin' && isAdmin && <AdminPanel />}
			</main>
            <AudioPlayer song={currentSong} onEnded={handlePlayNext} credentials={credentials}/>
		</div>
	);
}

export default Dashboard;

