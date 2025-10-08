import React, { useState, useRef, useEffect, useCallback } from 'react';

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
                <button onClick={() => { onSetStart(song.id); onClose(); }} className="block w-full text-left px-4 py-2 text-sm text-gray-200 hover:bg-gray-600">Set as Path start</button>
                <button onClick={() => { onSetEnd(song.id); onClose(); }} className="block w-full text-left px-4 py-2 text-sm text-gray-200 hover:bg-gray-600">Set as Path end</button>
                <button onClick={() => { onAddToPlaylist(song); onClose(); }} className="block w-full text-left px-4 py-2 text-sm text-gray-200 hover:bg-gray-600">Add to Playlist</button>
                <button
                    onClick={() => { onInstantMix(song); onClose(); }}
                    disabled={!audioMuseUrl}
                    className="block w-full text-left px-4 py-2 text-sm text-gray-200 hover:bg-gray-600 disabled:text-gray-500 disabled:cursor-not-allowed"
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
function PlayQueueView({ isOpen, onClose, queue, currentIndex, onRemove, onSelect, onAddToPlaylist, onInstantMix, audioMuseUrl, onClearQueue, onReorder, onCreateSongPath }) {
    const [activeMenu, setActiveMenu] = useState({ index: null, style: {} });
    const [startSongId, setStartSongId] = useState(null);
    const [endSongId, setEndSongId] = useState(null);
    const [visibleCount, setVisibleCount] = useState(50);
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
    
    // Reset visible count when queue changes or view is opened
    useEffect(() => {
        if (isOpen) {
            setVisibleCount(50);
        }
    }, [isOpen, queue]);

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
                <div className="p-4 border-b border-gray-700 flex justify-between items-center flex-shrink-0">
                    <h2 className="text-xl font-bold text-white">Up Next</h2>
                    <div className="flex items-center space-x-2">
                         <button 
                            onClick={() => {
                                if (!isPathCreationReady) {
                                    alert('Select a start and end song to create a path.');
                                    return;
                                }
                                // Minimal check: onCreateSongPath may depend on server config; call and let backend handle errors
                                handleCreatePath();
                            }}
                            className="text-sm bg-indigo-600 hover:bg-indigo-700 text-white font-bold py-1 px-3 rounded"
                         >
                            Create Song Path
                        </button>
                         <button onClick={onClearQueue} className="text-sm bg-red-600 hover:bg-red-700 text-white font-bold py-1 px-3 rounded">
                            Clear All
                        </button>
                        <button onClick={onClose} className="text-gray-400 hover:text-white">
                            <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12"></path></svg>
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
                                        <svg className="w-5 h-5 text-green-400 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20"><path fillRule="evenodd" d="M9.383 3.076A1 1 0 0110 4v12a1 1 0 01-1.707.707L4.586 13H2a1 1 0 01-1-1V8a1 1 0 011-1h2.586l3.707-3.707a1 1 0 011.09-.217zM14.657 2.929a1 1 0 011.414 0A9.972 9.972 0 0119 10a9.972 9.972 0 01-2.929 7.071 1 1 0 01-1.414-1.414A7.971 7.971 0 0017 10c0-2.21-.894-4.208-2.343-5.657a1 1 0 010-1.414z" clipRule="evenodd"></path></svg>
                                    ) : (
                                         <span className="text-gray-400 w-5 text-center flex-shrink-0">{index + 1}</span>
                                    )}
                                    <div className="overflow-hidden">
                                        <p className={`font-medium truncate ${isPlaying ? 'text-green-400' : 'text-white'}`}>{song.title}</p>
                                        <p className="text-sm text-gray-400 truncate">{song.artist}</p>
                                    </div>
                                </div>
                                <div className="flex items-center space-x-1 flex-shrink-0">
                                     <div className="flex flex-col">
                                        <button onClick={(e) => {e.stopPropagation(); onReorder(index, index - 1);}} className="text-gray-500 hover:text-white disabled:opacity-50" disabled={index === 0}><svg className="w-4 h-4" fill="currentColor" viewBox="0 0 20 20"><path fillRule="evenodd" d="M14.707 12.707a1 1 0 01-1.414 0L10 9.414l-3.293 3.293a1 1 0 01-1.414-1.414l4-4a1 1 0 011.414 0l4 4a1 1 0 010 1.414z" clipRule="evenodd"></path></svg></button>
                                        <button onClick={(e) => {e.stopPropagation(); onReorder(index, index + 1);}} className="text-gray-500 hover:text-white disabled:opacity-50" disabled={index === queue.length - 1}><svg className="w-4 h-4" fill="currentColor" viewBox="0 0 20 20"><path fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd"></path></svg></button>
                                    </div>
                                    <div className="relative">
                                        <button onClick={(e) => handleActionClick(e, index)} className="text-gray-500 hover:text-white p-2">
                                            <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 20 20"><path d="M10 6a2 2 0 110-4 2 2 0 010 4zM10 12a2 2 0 110-4 2 2 0 010 4zM10 18a2 2 0 110-4 2 2 0 010 4z"></path></svg>
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
                                        className="text-gray-500 hover:text-red-500 p-2"
                                        title="Remove from queue"
                                    >
                                         <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12"></path></svg>
                                    </button>
                                </div>
                            </li>
                        );
                    }) : (
                        <li className="p-4 text-center text-gray-500">The queue is empty.</li>
                    )}
                </ul>
            </div>
        </div>
    );
}

export default PlayQueueView;
