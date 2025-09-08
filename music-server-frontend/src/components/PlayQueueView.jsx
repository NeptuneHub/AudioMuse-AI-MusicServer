import React from 'react';

/**
 * A modal component to display and manage the current play queue.
 */
function PlayQueueView({ isOpen, onClose, queue, currentIndex, onRemove, onSelect }) {
    if (!isOpen) return null;

    return (
        // Overlay
        <div className="fixed inset-0 bg-black bg-opacity-60 z-50 flex justify-center items-end" onClick={onClose}>
            <div 
                className="bg-gray-800 w-full max-w-2xl max-h-[70vh] rounded-t-lg shadow-lg flex flex-col"
                // Stop click propagation to prevent the modal from closing when clicking inside
                onClick={e => e.stopPropagation()}
            >
                {/* Header */}
                <div className="p-4 border-b border-gray-700 flex justify-between items-center flex-shrink-0">
                    <h2 className="text-xl font-bold text-white">Up Next</h2>
                    <button onClick={onClose} className="text-gray-400 hover:text-white">
                        <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12"></path></svg>
                    </button>
                </div>

                {/* Song List */}
                <ul className="overflow-y-auto flex-grow p-2">
                    {queue.length > 0 ? queue.map((song, index) => {
                        const isPlaying = index === currentIndex;
                        return (
                            <li 
                                key={`${song.id}-${index}`} 
                                className={`flex items-center justify-between p-3 rounded-md cursor-pointer ${isPlaying ? 'bg-teal-900/50' : 'hover:bg-gray-700'}`}
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
                                <button 
                                    onClick={(e) => { 
                                        e.stopPropagation(); // Prevent jumping to the song when removing
                                        onRemove(index); 
                                    }} 
                                    className="text-gray-500 hover:text-red-500 p-2 flex-shrink-0"
                                    title="Remove from queue"
                                >
                                     <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"></path></svg>
                                </button>
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

