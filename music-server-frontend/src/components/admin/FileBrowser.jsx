import React, { useState, useEffect, useCallback } from 'react';
import Modal from '../Modal';

function FileBrowser({ onSelect, onClose }) {
    const [currentPath, setCurrentPath] = useState('/');
    const [items, setItems] = useState([]);
    const [error, setError] = useState('');

    const fetchDirectory = useCallback(async (path) => {
        setError('');
        const token = localStorage.getItem('token');
        try {
            const response = await fetch(`/api/v1/admin/browse?path=${encodeURIComponent(path)}`, {
                headers: { 'Authorization': `Bearer ${token}` }
            });
            if (!response.ok) {
                const data = await response.json();
                throw new Error(data.error || `Server error: ${response.status}`);
            }
            const data = await response.json();
            let dirItems = (data.items || []).filter(i => i.type === 'dir');
            
            const isWindowsRoot = /^[a-zA-Z]:\\?$/.test(data.path);
            const isUnixRoot = data.path === '/';

            if (!isUnixRoot && !isWindowsRoot) {
                 if (!dirItems.some(i => i.name === '..')) {
                    dirItems.unshift({ name: '..', type: 'dir' });
                }
            }
            setItems(dirItems);
            setCurrentPath(data.path || path);
        } catch (err) {
            setError(err.message);
            setItems([]);
        }
    }, []);

    useEffect(() => {
        fetchDirectory('/');
    }, [fetchDirectory]);

    const handleItemClick = (item) => {
        let newPath;
        const isWindows = currentPath.includes('\\');
        const separator = isWindows ? '\\' : '/';

        if (item.name === '..') {
            const parts = currentPath.split(separator).filter(p => p && p !== ':');
            parts.pop();
            if (isWindows) {
                if (parts.length === 1 && parts[0].endsWith(':')) {
                    newPath = parts[0] + separator;
                } else if (parts.length === 0) {
                    newPath = '/';
                }
                else {
                    newPath = parts.join(separator);
                }
            } else {
                newPath = separator + parts.join(separator);
            }
        } else {
            if (currentPath.endsWith(separator)) {
                newPath = `${currentPath}${item.name}`;
            } else {
                newPath = `${currentPath}${separator}${item.name}`;
            }
        }
        fetchDirectory(newPath);
    };

    return (
        <Modal onClose={onClose}>
             <h3 className="text-xl font-bold mb-4 text-teal-400">Browse For Folder</h3>
             <div className="bg-gray-900 p-2 rounded mb-4 font-mono text-sm break-all">{currentPath}</div>
             {error && <p className="text-red-500 mb-4">Error: {error}</p>}
             <ul className="h-64 overflow-y-auto border border-gray-700 rounded p-2 mb-4">
                 {items.map((item, index) => (
                     <li key={index} onClick={() => handleItemClick(item)} className="p-2 hover:bg-gray-700 rounded cursor-pointer flex items-center"><svg className="w-5 h-5 mr-2 text-teal-400" fill="none" stroke="currentColor" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z"></path></svg>{item.name}</li>
                 ))}
             </ul>
             <div className="flex flex-col sm:flex-row justify-end space-y-2 sm:space-y-0 sm:space-x-4">
                 <button onClick={onClose} className="bg-gray-600 hover:bg-gray-700 text-white font-bold py-2 px-4 rounded">Cancel</button>
                 <button onClick={() => onSelect(currentPath)} className="bg-teal-500 hover:bg-teal-600 text-white font-bold py-2 px-4 rounded">Select Folder</button>
             </div>
        </Modal>
    );
}

export default FileBrowser;