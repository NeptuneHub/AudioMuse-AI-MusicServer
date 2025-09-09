import React, { useState, useRef, useEffect } from 'react';

const QueueItemMenu = ({ song, onClose, onAddToPlaylist, onInstantMix, audioMuseUrl, onSetStart, onSetEnd }) => {
    const menuRef = useRef(null);

    useEffect(() => {
        const handleClickOutside = (event) => {
            if (menuRef.current && !menuRef.current.contains(event.target)) {
                onClose();
            }
        };
        document.addEventListener("mousedown", handleClickOutside);
        return () => document.removeEventListener("mousedown", handleClickOutside);
    }, [onClose]);

    return (
        <div ref={menuRef} className="absolute right-0 bottom-full mb-1 w-48 bg-gray-700 rounded-md shadow-lg z-10">
            <ul className="py-1 text-sm text-gray-200">
                <li><button onClick={() => onSetStart(song.id)} className="w-full text-left block px-4 py-2 hover:bg-gray-600">Set as Start</button></li>
                <li><button onClick={() => onSetEnd(song.id)} className="w-full text-left block px-4 py-2 hover:bg-gray-600">Set as End</button></li>
                <li><hr className="border-gray-600 my-1"/></li>
                <li><button onClick={() => onAddToPlaylist(song)} className="w-full text-left block px-4 py-2 hover:bg-gray-600">Add to Playlist...</button></li>
                <li><button onClick={() => onInstantMix(song)} disabled={!audioMuseUrl} className={`w-full text-left block px-4 py-2 ${audioMuseUrl ? 'hover:bg-gray-600' : 'text-gray-500 cursor-not-allowed'}`}>Instant Mix</button></li>
            </ul>
        </div>
    );
};


function PlayQueueView({ isOpen, onClose, queue, currentIndex, onRemove, onSelect, onAddToPlaylist, onInstantMix, audioMuseUrl, onClearQueue, onReorder, onCreateSongPath }) {
    const [activeMenu, setActiveMenu] = useState(null);
    const [startSongId, setStartSongId] = useState(null);
    const [endSongId, setEndSongId] = useState(null);

    useEffect(() => {
        if (!isOpen) {
            setActiveMenu(null);
            return;
        }
        if (queue.length === 2) {
            setStartSongId(queue[0].id);
            setEndSongId(queue[1].id);
        } else {
            if (startSongId && !queue.some(s => s.id === startSongId)) setStartSongId(null);
            if (endSongId && !queue.some(s => s.id === endSongId)) setEndSongId(null);
        }
    }, [isOpen, queue, startSongId, endSongId]);

    const handleSetStart = (songId) => {
        if (songId === endSongId) setEndSongId(null);
        setStartSongId(songId);
        setActiveMenu(null);
    };

    const handleSetEnd = (songId) => {
        if (songId === startSongId) setStartSongId(null);
        setEndSongId(songId);
        setActiveMenu(null);
    };

    const isPathReady = audioMuseUrl && ((startSongId && endSongId) || queue.length === 2);

    const handleCreatePathClick = () => {
        if (!isPathReady) return;
        const start = queue.length === 2 ? queue[0].id : startSongId;
        const end = queue.length === 2 ? queue[1].id : endSongId;
        onCreateSongPath(start, end);
    };

    if (!isOpen) return null;

    return (
        <div className="fixed inset-0 bg-black bg-opacity-60 z-50 flex justify-center items-end" onClick={onClose}>
            <div 
                className="bg-gray-800 w-full max-w-2xl max-h-[70vh] rounded-t-lg shadow-lg flex flex-col"
                onClick={e => e.stopPropagation()}
            >
                <div className="p-4 border-b border-gray-700 flex justify-between items-center flex-shrink-0">
                    <h2 className="text-xl font-bold text-white">Up Next</h2>
                    <div className="flex items-center space-x-2">
                        <button onClick={onClearQueue} className="text-sm text-gray-400 hover:text-white">Clear All</button>
                        <button onClick={onClose} className="text-gray-400 hover:text-white">
                            <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12"></path></svg>
                        </button>
                    </div>
                </div>

                <ul className="overflow-y-auto flex-grow p-2">
                    {queue.map((song, index) => {
                        const isPlaying = index === currentIndex;
                        const isStart = song.id === startSongId;
                        const isEnd = song.id === endSongId;
                        const itemClass = isPlaying ? 'bg-teal-900/50' : isStart ? 'bg-green-900/60' : isEnd ? 'bg-red-900/60' : 'hover:bg-gray-700';

                        return (
                            <li key={`${song.id}-${index}`} className={`flex items-center justify-between p-2 rounded-md group ${itemClass}`}>
                                <div className="flex items-center space-x-2 overflow-hidden flex-grow cursor-pointer" onClick={() => onSelect(index)}>
                                    <div className="flex flex-col items-center">
                                        <button onClick={(e) => { e.stopPropagation(); onReorder(index, index - 1);}} disabled={index === 0} className="disabled:opacity-20 text-gray-400 hover:text-white"><svg className="w-4 h-4" fill="currentColor" viewBox="0 0 20 20"><path fillRule="evenodd" d="M14.707 12.707a1 1 0 01-1.414 0L10 9.414l-3.293 3.293a1 1 0 01-1.414-1.414l4-4a1 1 0 011.414 0l4 4a1 1 0 010 1.414z" clipRule="evenodd"></path></svg></button>
                                        <button onClick={(e) => { e.stopPropagation(); onReorder(index, index + 1);}} disabled={index === queue.length - 1} className="disabled:opacity-20 text-gray-400 hover:text-white"><svg className="w-4 h-4" fill="currentColor" viewBox="0 0 20 20"><path fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd"></path></svg></button>
                                    </div>
                                    <div className="overflow-hidden">
                                        <p className={`font-medium truncate ${isPlaying ? 'text-green-400' : 'text-white'}`}>{song.title}</p>
                                        <p className="text-sm text-gray-400 truncate">{song.artist}</p>
                                    </div>
                                </div>
                                <div className="flex items-center space-x-1 flex-shrink-0">
                                     <div className="relative">
                                        <button onClick={() => setActiveMenu(activeMenu === index ? null : index)} className="text-gray-500 hover:text-white p-2" title="More actions">
                                           <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 20 20"><path d="M10 6a2 2 0 110-4 2 2 0 010 4zM10 12a2 2 0 110-4 2 2 0 010 4zM10 18a2 2 0 110-4 2 2 0 010 4z"></path></svg>
                                        </button>
                                        {activeMenu === index && <QueueItemMenu song={song} onClose={() => setActiveMenu(null)} onAddToPlaylist={onAddToPlaylist} onInstantMix={onInstantMix} audioMuseUrl={audioMuseUrl} onSetStart={handleSetStart} onSetEnd={handleSetEnd} />}
                                    </div>
                                    <button onClick={(e) => { e.stopPropagation(); onRemove(index);}} className="text-gray-500 hover:text-red-500 p-2" title="Remove from queue">
                                        <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12"></path></svg>
                                    </button>
                                </div>
                            </li>
                        );
                    })}
                    {queue.length === 0 && <li className="p-4 text-center text-gray-500">The queue is empty.</li>}
                </ul>
                <div className="p-4 border-t border-gray-700 flex-shrink-0">
                    <button onClick={handleCreatePathClick} disabled={!isPathReady} className="w-full bg-indigo-600 hover:bg-indigo-700 text-white font-bold py-2 px-4 rounded disabled:bg-indigo-400 disabled:cursor-not-allowed disabled:opacity-50">
                        Create Song Path
                    </button>
                    {!audioMuseUrl && <p className="text-xs text-center text-gray-500 mt-2">Enable Song Path by setting the AudioMuse-AI URL in the Admin Panel.</p>}
                </div>
            </div>
        </div>
    );
}

export default PlayQueueView;

