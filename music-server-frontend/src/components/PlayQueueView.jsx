import React, { useState, useRef, useEffect } from 'react';

const QueueItemMenu = ({ song, onAddToPlaylist, onInstantMix, audioMuseUrl, onClose }) => {
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
                <li>
                    <button 
                        onClick={() => { onAddToPlaylist(song); onClose(); }} 
                        className="w-full text-left block px-4 py-2 hover:bg-gray-600"
                    >
                        Add to Playlist...
                    </button>
                </li>
                <li>
                     <button 
                        onClick={() => { onInstantMix(song); onClose(); }} 
                        disabled={!audioMuseUrl}
                        className={`w-full text-left block px-4 py-2 ${audioMuseUrl ? 'hover:bg-gray-600' : 'text-gray-500 cursor-not-allowed'}`}
                    >
                        Instant Mix
                    </button>
                </li>
            </ul>
        </div>
    );
};


function PlayQueueView({ isOpen, onClose, queue, currentIndex, onRemove, onSelect, onAddToPlaylist, onInstantMix, audioMuseUrl }) {
    const [activeMenu, setActiveMenu] = useState(null); // Holds the index of the song with the open menu

    if (!isOpen) return null;

    const toggleMenu = (index) => {
        setActiveMenu(activeMenu === index ? null : index);
    };

    return (
        <div className="fixed inset-0 bg-black bg-opacity-60 z-50 flex justify-center items-end" onClick={onClose}>
            <div 
                className="bg-gray-800 w-full max-w-2xl max-h-[70vh] rounded-t-lg shadow-lg flex flex-col"
                onClick={e => e.stopPropagation()}
            >
                <div className="p-4 border-b border-gray-700 flex justify-between items-center flex-shrink-0">
                    <h2 className="text-xl font-bold text-white">Up Next</h2>
                    <button onClick={onClose} className="text-gray-400 hover:text-white">
                        <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12"></path></svg>
                    </button>
                </div>

                <ul className="overflow-y-auto flex-grow p-2">
                    {queue.length > 0 ? queue.map((song, index) => {
                        const isPlaying = index === currentIndex;
                        return (
                            <li 
                                key={`${song.id}-${index}`} 
                                className={`flex items-center justify-between p-3 rounded-md group ${isPlaying ? 'bg-teal-900/50' : 'hover:bg-gray-700'}`}
                            >
                                <div className="flex items-center space-x-4 overflow-hidden flex-grow cursor-pointer" onClick={() => onSelect(index)}>
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
                                     <div className="relative">
                                        <button 
                                            onClick={() => toggleMenu(index)} 
                                            className="text-gray-500 hover:text-white p-2"
                                            title="More actions"
                                        >
                                           <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 20 20"><path d="M10 6a2 2 0 110-4 2 2 0 010 4zM10 12a2 2 0 110-4 2 2 0 010 4zM10 18a2 2 0 110-4 2 2 0 010 4z"></path></svg>
                                        </button>
                                        {activeMenu === index && (
                                            <QueueItemMenu 
                                                song={song} 
                                                onAddToPlaylist={onAddToPlaylist}
                                                onInstantMix={onInstantMix}
                                                audioMuseUrl={audioMuseUrl}
                                                onClose={() => setActiveMenu(null)}
                                            />
                                        )}
                                    </div>
                                    <button 
                                        onClick={(e) => { 
                                            e.stopPropagation();
                                            onRemove(index); 
                                        }} 
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
