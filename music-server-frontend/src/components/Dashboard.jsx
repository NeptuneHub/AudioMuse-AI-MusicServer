// Suggested path: music-server-frontend/src/components/Dashboard.jsx
import React, { useState, useMemo, useCallback } from 'react';
import { Songs, Albums, Artists } from './MusicViews';
import Playlists from './Playlists'; // Assuming you create this file
import AdminPanel from './AdminPanel';
import AudioPlayer from './AudioPlayer';

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

export default Dashboard;
