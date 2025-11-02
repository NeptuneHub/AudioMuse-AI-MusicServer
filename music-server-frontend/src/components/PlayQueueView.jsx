import React, { useState, useRef, useEffect, useCallback } from 'react';
import { subsonicFetch } from '../api';

const SaveAsPlaylistModal = ({ isOpen, onClose, queue, onSuccess }) => {
    const [playlistName, setPlaylistName] = useState('');
    const [error, setError] = useState('');
    const [success, setSuccess] = useState('');
    const [isCreating, setIsCreating] = useState(false);

    useEffect(() => {
        if (isOpen) {
            // Generate a default name based on current time
            const now = new Date();
            setPlaylistName(`Queue - ${now.toLocaleDateString()} ${now.toLocaleTimeString([], {hour: '2-digit', minute:'2-digit'})}`);
            setError('');
            setSuccess('');
            setIsCreating(false);
        }
    }, [isOpen]);

    const handleCreatePlaylist = async () => {
        if (!playlistName.trim()) {
            setError('Please enter a playlist name.');
            return;
        }

        if (queue.length === 0) {
            setError('Queue is empty.');
            return;
        }

        setError('');
        setSuccess('');
        setIsCreating(true);

        try {
            // Step 1: Create the playlist
            await subsonicFetch('createPlaylist.view', { name: playlistName.trim() });
            
            // Step 2: Get the newly created playlist
            const playlistsData = await subsonicFetch('getPlaylists.view');
            const allPlaylists = playlistsData.playlists?.playlist || [];
            const playlists = Array.isArray(allPlaylists) ? allPlaylists : [allPlaylists];
            const newPlaylist = playlists.find(p => p.name === playlistName.trim());

            if (!newPlaylist) {
                throw new Error("Could not find the newly created playlist.");
            }

            // Step 3: Add all queue songs to the playlist
            for (const song of queue) {
                await subsonicFetch('updatePlaylist.view', {
                    playlistId: newPlaylist.id,
                    songIdToAdd: song.id
                });
            }

            setSuccess(`Successfully created "${playlistName}" with ${queue.length} songs!`);
            onSuccess?.(playlistName, queue.length);
            
            // Close modal after a short delay
            setTimeout(() => {
                onClose();
            }, 1500);

        } catch (err) {
            setError(err.message || 'Failed to create playlist.');
        } finally {
            setIsCreating(false);
        }
    };

    if (!isOpen) return null;

    return (
        <div className="fixed inset-0 bg-black bg-opacity-60 flex items-center justify-center z-[70] p-4">
            <div className="bg-gray-800 p-6 rounded-lg shadow-xl w-full sm:w-11/12 md:max-w-md">
                <h3 className="text-xl font-bold mb-4 text-teal-400">Save Queue as Playlist</h3>
                <p className="text-gray-300 mb-4">Save {queue.length} songs from the queue as a new playlist.</p>
                
                {error && <p className="text-red-500 mb-2">{error}</p>}
                {success && <p className="text-green-400 mb-2">{success}</p>}
                
                <input
                    type="text"
                    value={playlistName}
                    onChange={(e) => setPlaylistName(e.target.value)}
                    placeholder="Enter playlist name..."
                    className="w-full p-3 bg-gray-700 rounded mb-4 text-white placeholder-gray-400"
                    disabled={isCreating}
                    onKeyDown={(e) => {
                        if (e.key === 'Enter') {
                            handleCreatePlaylist();
                        }
                    }}
                />
                
                <div className="flex justify-end space-x-4">
                    <button 
                        onClick={onClose} 
                        disabled={isCreating}
                        className="border-2 border-gray-500 text-gray-400 bg-gray-500/10 hover:bg-gray-500/20 hover:scale-105 transition-all rounded-lg font-bold py-2 px-4 disabled:opacity-50 disabled:cursor-not-allowed disabled:hover:scale-100"
                    >
                        Cancel
                    </button>
                    <button 
                        onClick={handleCreatePlaylist}
                        disabled={isCreating || !playlistName.trim()}
                        className="border-2 border-teal-500 text-teal-400 bg-teal-500/10 hover:bg-teal-500/20 hover:scale-105 transition-all rounded-lg font-bold py-2 px-4 disabled:opacity-50 disabled:cursor-not-allowed disabled:hover:scale-100"
                    >
                        {isCreating ? 'Creating...' : 'Create Playlist'}
                    </button>
                </div>
            </div>
        </div>
    );
};

const SongActionsMenu = ({ song, onAddToPlaylist, onInstantMix, audioMuseUrl, onClose, onSetStart, onSetEnd, positionStyle }) => {
    const menuRef = useRef(null);

    useEffect(() => {
        const handleClickOutside = (event) => {
            if (menuRef.current && !menuRef.current.contains(event.target)) {
                onClose();
            }
        };
        document.addEventListener('mousedown', handleClickOutside);
        return () => {
            document.removeEventListener('mousedown', handleClickOutside);
        };
    }, [onClose]);
    
    return (
        <div
            ref={menuRef}
            className="absolute right-0 w-48 bg-gray-700 rounded-md shadow-lg z-20"
            style={positionStyle}
            onClick={(e) => e.stopPropagation()}
        >
            <div className="py-1">
                <button onClick={() => { onSetStart(song.id); onClose(); }} className="block w-full text-left px-4 py-2 text-sm border-2 border-blue-500 text-blue-400 bg-blue-500/10 hover:bg-blue-500/20 transition-all rounded-lg mb-1">Set as Path start</button>
                <button onClick={() => { onSetEnd(song.id); onClose(); }} className="block w-full text-left px-4 py-2 text-sm border-2 border-purple-500 text-purple-400 bg-purple-500/10 hover:bg-purple-500/20 transition-all rounded-lg mb-1">Set as Path end</button>
                <button onClick={() => { onAddToPlaylist(song); onClose(); }} className="block w-full text-left px-4 py-2 text-sm border-2 border-teal-500 text-teal-400 bg-teal-500/10 hover:bg-teal-500/20 transition-all rounded-lg mb-1">Add to Playlist</button>
                <button
                    onClick={() => { onInstantMix(song); onClose(); }}
                    disabled={!audioMuseUrl}
                    className="block w-full text-left px-4 py-2 text-sm border-2 border-yellow-500 text-yellow-400 bg-yellow-500/10 hover:bg-yellow-500/20 transition-all rounded-lg disabled:opacity-50 disabled:cursor-not-allowed"
                >
                    Instant Mix
                </button>
            </div>
        </div>
    );
};


/**
 * A modal component to display and manage the current play queue.
 */
function PlayQueueView({ isOpen, onClose, queue, currentIndex, onRemove, onSelect, onTogglePlayPause, onAddToPlaylist, onInstantMix, audioMuseUrl, onClearQueue, onReorder, onCreateSongPath }) {
    const [activeMenu, setActiveMenu] = useState({ index: null, style: {} });
    const [startSongId, setStartSongId] = useState(null);
    const [endSongId, setEndSongId] = useState(null);
    const [visibleCount, setVisibleCount] = useState(50);
    const [showSaveAsPlaylist, setShowSaveAsPlaylist] = useState(false);
    const queueListRef = useRef(null);

    useEffect(() => {
        if (queue.length !== 2) {
             setStartSongId(null);
             setEndSongId(null);
        } else {
             setStartSongId(queue[0].id);
             setEndSongId(queue[1].id);
        }
    }, [queue]);
    
    // Reset visible count and scroll to current song when queue changes or view is opened
    useEffect(() => {
        if (isOpen) {
            setVisibleCount(50);
            
            // Scroll to currently playing song after a short delay to ensure DOM is ready
            setTimeout(() => {
                if (queueListRef.current && currentIndex >= 0 && queue.length > 0) {
                    // Find the currently playing song element
                    const listElement = queueListRef.current;
                    const songElements = listElement.querySelectorAll('li');
                    
                    if (songElements[currentIndex]) {
                        // Check if the list actually has scroll (avoid unnecessary scrolling)
                        const hasScroll = listElement.scrollHeight > listElement.clientHeight;
                        
                        if (hasScroll) {
                            // Scroll to center the current song
                            const songElement = songElements[currentIndex];
                            const songRect = songElement.getBoundingClientRect();
                            
                            // Calculate the offset to center the song in the visible area
                            const scrollOffset = songElement.offsetTop - (listElement.clientHeight / 2) + (songRect.height / 2);
                            
                            listElement.scrollTo({
                                top: scrollOffset,
                                behavior: 'smooth'
                            });
                            
                            console.log('Scrolled PlayQueue to song index', currentIndex);
                        }
                    }
                }
            }, 100);
        }
    }, [isOpen, queue, currentIndex]);

    const songsToDisplay = queue.slice(0, visibleCount);
    const hasMore = visibleCount < queue.length;

    const loadMore = useCallback(() => {
        if (hasMore) {
            setVisibleCount(prev => prev + 50);
        }
    }, [hasMore]);

    const observer = useRef();
    const lastSongElementRef = useCallback(node => {
        if (observer.current) observer.current.disconnect();
        observer.current = new IntersectionObserver(entries => {
            if (entries[0].isIntersecting && hasMore) {
                loadMore();
            }
        });
        if (node) observer.current.observe(node);
    }, [hasMore, loadMore]);


    if (!isOpen) return null;

    const handleActionClick = (e, index) => {
        e.stopPropagation();
        if (activeMenu.index === index) {
            setActiveMenu({ index: null, style: {} }); // Close if already open
            return;
        }

        const buttonRect = e.currentTarget.getBoundingClientRect();
        const viewportHeight = window.innerHeight;
        
        const spaceBelow = viewportHeight - buttonRect.bottom;
        const menuHeight = 160; // Approximate height of the menu in pixels

        let style = {};
        if (spaceBelow < menuHeight && buttonRect.top > menuHeight) {
            style = { bottom: '100%' };
        } else {
            style = { top: '100%' };
        }
        
        setActiveMenu({ index, style });
    };

    const handleCreatePath = () => {
        if (!startSongId || !endSongId) return;
        onCreateSongPath(startSongId, endSongId);
        setStartSongId(null);
        setEndSongId(null);
    };
    
    const isPathCreationReady = (startSongId && endSongId) || queue.length === 2;

    return (
        <div className="fixed inset-0 bg-black bg-opacity-60 z-50 flex justify-center items-end" onClick={onClose}>
            <div 
                className="bg-gray-800 w-full max-w-2xl h-[60vh] rounded-t-lg shadow-lg flex flex-col"
                onClick={e => e.stopPropagation()}
            >
                <div className="p-3 sm:p-4 border-b border-gray-700 flex justify-between items-center flex-shrink-0">
                    <h2 className="text-lg sm:text-xl font-bold text-white">Up Next</h2>
                    <div className="flex items-center flex-wrap gap-1.5 sm:gap-2">
                        <button 
                            onClick={() => setShowSaveAsPlaylist(true)}
                            disabled={queue.length === 0}
                            className="text-xs sm:text-sm border-2 border-green-500 text-green-400 bg-green-500/10 hover:bg-green-500/20 hover:scale-105 transition-all rounded-lg font-bold py-1 px-2 sm:px-3 disabled:opacity-50 disabled:cursor-not-allowed disabled:hover:scale-100"
                            title="Save all songs in queue as a new playlist"
                        >
                            Save
                        </button>
                         <button 
                            onClick={() => {
                                if (!isPathCreationReady) {
                                    alert('Select a start and end song to create a path.');
                                    return;
                                }
                                // Minimal check: onCreateSongPath may depend on server config; call and let backend handle errors
                                handleCreatePath();
                            }}
                            className="text-xs sm:text-sm border-2 border-indigo-500 text-indigo-400 bg-indigo-500/10 hover:bg-indigo-500/20 hover:scale-105 transition-all rounded-lg font-bold py-1 px-2 sm:px-3"
                         >
                            Path
                        </button>
                         <button onClick={onClearQueue} className="text-xs sm:text-sm border-2 border-red-500 text-red-400 bg-red-500/10 hover:bg-red-500/20 hover:scale-105 transition-all rounded-lg font-bold py-1 px-2 sm:px-3">
                            Clear
                        </button>
                        <button onClick={onClose} className="text-gray-400 hover:text-white p-1">
                            <svg className="w-5 h-5 sm:w-6 sm:h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12"></path></svg>
                        </button>
                    </div>
                </div>

                <ul ref={queueListRef} className="overflow-y-auto flex-grow p-2">
                    {songsToDisplay.length > 0 ? songsToDisplay.map((song, index) => {
                        const isPlaying = index === currentIndex;
                        const isStart = song.id === startSongId;
                        const isEnd = song.id === endSongId;

                        let rowClass = 'hover:bg-gray-700';
                        if (isPlaying) rowClass = 'bg-teal-900/50';
                        if (isStart) rowClass = 'bg-green-900/50 border-l-4 border-green-400';
                        if (isEnd) rowClass = 'bg-red-900/50 border-l-4 border-red-400';

                        return (
                            <li 
                                ref={index === songsToDisplay.length - 1 ? lastSongElementRef : null}
                                key={`${song.id}-${index}`} 
                                className={`flex items-center justify-between p-3 rounded-md cursor-pointer transition-colors ${rowClass}`}
                                onClick={() => onSelect(index)}
                            >
                                <div className="flex items-center space-x-4 overflow-hidden">
                                    {isPlaying ? (
                                        <svg className="w-5 h-5 text-accent-400 flex-shrink-0 animate-pulse" fill="currentColor" viewBox="0 0 20 20">
                                            <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM9.555 7.168A1 1 0 008 8v4a1 1 0 001.555.832l3-2a1 1 0 000-1.664l-3-2z" clipRule="evenodd"></path>
                                        </svg>
                                    ) : (
                                        <span className="text-gray-400 w-5 text-center flex-shrink-0">{index + 1}</span>
                                    )}
                                    <div className="overflow-hidden">
                                        <p className={`font-medium truncate ${isPlaying ? 'text-accent-400' : 'text-white'}`}>{song.title}</p>
                                        <p className="text-sm text-gray-400 truncate">{song.artist}</p>
                                    </div>
                                </div>
                                <div className="flex items-center space-x-0.5 sm:space-x-1 flex-shrink-0">
                                     <div className="flex flex-col">
                                        <button onClick={(e) => {e.stopPropagation(); onReorder(index, index - 1);}} className="text-gray-500 hover:text-white disabled:opacity-50 p-0.5" disabled={index === 0}><svg className="w-4 h-4 sm:w-5 sm:h-5" fill="currentColor" viewBox="0 0 20 20"><path fillRule="evenodd" d="M14.707 12.707a1 1 0 01-1.414 0L10 9.414l-3.293 3.293a1 1 0 01-1.414-1.414l4-4a1 1 0 011.414 0l4 4a1 1 0 010 1.414z" clipRule="evenodd"></path></svg></button>
                                        <button onClick={(e) => {e.stopPropagation(); onReorder(index, index + 1);}} className="text-gray-500 hover:text-white disabled:opacity-50 p-0.5" disabled={index === queue.length - 1}><svg className="w-4 h-4 sm:w-5 sm:h-5" fill="currentColor" viewBox="0 0 20 20"><path fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd"></path></svg></button>
                                    </div>
                                    <div className="relative">
                                        <button onClick={(e) => handleActionClick(e, index)} className="text-gray-500 hover:text-white p-1 sm:p-2">
                                            <svg className="w-5 h-5 sm:w-6 sm:h-6" fill="currentColor" viewBox="0 0 20 20"><path d="M10 6a2 2 0 110-4 2 2 0 010 4zM10 12a2 2 0 110-4 2 2 0 010 4zM10 18a2 2 0 110-4 2 2 0 010 4z"></path></svg>
                                        </button>
                                        {activeMenu.index === index && (
                                            <SongActionsMenu
                                                song={song}
                                                onAddToPlaylist={onAddToPlaylist}
                                                onInstantMix={onInstantMix}
                                                audioMuseUrl={audioMuseUrl}
                                                onClose={() => setActiveMenu({ index: null, style: {} })}
                                                onSetStart={setStartSongId}
                                                onSetEnd={setEndSongId}
                                                positionStyle={activeMenu.style}
                                            />
                                        )}
                                    </div>
                                    <button 
                                        onClick={(e) => { e.stopPropagation(); onRemove(index); }} 
                                        className="text-gray-500 hover:text-red-500 p-1 sm:p-2"
                                        title="Remove from queue"
                                    >
                                         <svg className="w-5 h-5 sm:w-6 sm:h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12"></path></svg>
                                    </button>
                                </div>
                            </li>
                        );
                    }) : (
                        <li className="p-4 text-center text-gray-500">The queue is empty.</li>
                    )}
                </ul>
                
                <SaveAsPlaylistModal
                    isOpen={showSaveAsPlaylist}
                    onClose={() => setShowSaveAsPlaylist(false)}
                    queue={queue}
                    onSuccess={(playlistName, songCount) => {
                        // Optional: You could add a toast notification here
                        console.log(`Created playlist "${playlistName}" with ${songCount} songs`);
                    }}
                />
            </div>
        </div>
    );
}

export default PlayQueueView;
